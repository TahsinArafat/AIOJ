//! `aioj-plagiarism` — the similarity engine behind AIOJ plagiarism reports.
//!
//! It is a **pure function over stdin/stdout**, deliberately: no database, no
//! network listener, no state. The Go service owns the report lifecycle and the
//! SQL; this process owns the compute. That boundary is what lets the engine be
//! swapped, tested in isolation, and killed without corrupting a report.
//!
//! # Protocol
//!
//! One JSON request object on stdin (newline-delimited source text is escaped
//! inside the JSON, so the request is a single line):
//!
//! ```json
//! {"threshold":0.7,"inputs":[
//!   {"key":"sub-1","problem_id":"prob-1","user_id":"user-1","code":"..."}
//! ]}
//! ```
//!
//! The response is newline-delimited JSON on stdout — one object per flagged
//! pair, then exactly one summary line:
//!
//! ```text
//! {"pair":{"problem_id":"prob-1","a":"sub-1","b":"sub-2","similarity":0.83,"matched_tokens":742}}
//! {"summary":{"inputs":2,"pairs_total":1,"pairs_scored":1,"flagged":1,"engine":"rust","elapsed_ms":0}}
//! ```
//!
//! Streaming per pair keeps peak memory at one line, so a contest with thousands
//! of flags does not need the whole result set in memory on either side.
//!
//! Errors are also JSON on stdout (`{"error":"..."}`) with a non-zero exit code,
//! because a Go caller that gets an unparseable stdout must fall back rather
//! than record a report full of zeroes.
//!
//! `--tokens` reads raw source bytes from stdin and writes the space-joined token
//! stream. `internal/plagiarism/engine_parity_test.go` uses it to prove the Rust
//! tokenizer agrees with the Go one instead of assuming it does.
//!
//! # Why the JSON encoding is hand-assembled
//!
//! The wire schema has five fields, so `serde`'s derive macros would buy very
//! little — and they are proc-macros, which cannot be built for a `*-musl`
//! target. That is not a hypothetical: a `rust:alpine` image fails with
//! "cannot produce proc-macro for `serde_derive` as the target does not support
//! these crate types". Using `serde_json::Value` keeps correct JSON string
//! escaping (the part that must not be hand-rolled) while leaving the crate free
//! of proc-macro dependencies, so the engine builds statically in CI with no
//! extra toolchain setup.

use std::io::{self, BufWriter, Read, Write};
use std::process::ExitCode;
use std::time::Instant;

use serde_json::{json, Value};

mod engine;
mod lcs;
mod tokenizer;

use engine::Input;
use tokenizer::MAX_INPUT_BYTES;

#[derive(Debug)]
struct Request {
    threshold: f64,
    inputs: Vec<Input>,
}

fn main() -> ExitCode {
    let args: Vec<String> = std::env::args().skip(1).collect();
    match args.first().map(String::as_str) {
        Some("--tokens") => run_tokens_mode(),
        Some("--version") => {
            println!("{} {}", env!("CARGO_PKG_NAME"), env!("CARGO_PKG_VERSION"));
            ExitCode::SUCCESS
        }
        Some("--help") | Some("-h") => {
            print_help();
            ExitCode::SUCCESS
        }
        // Default mode, and an explicit `--scan` for callers that prefer to be
        // unambiguous. No other flags are accepted, so a typo fails loudly
        // instead of silently reading stdin as something else.
        None | Some("--scan") => run_scan_mode(),
        Some(other) => {
            emit_error(&format!("unknown argument: {}", other));
            ExitCode::from(2)
        }
    }
}

fn print_help() {
    println!("aioj-plagiarism {}", env!("CARGO_PKG_VERSION"));
    println!();
    println!("USAGE:");
    println!("  aioj-plagiarism [--scan]   read a JSON request on stdin, stream NDJSON results");
    println!(
        "  aioj-plagiarism --tokens   tokenize raw source on stdin, print space-joined tokens"
    );
    println!("  aioj-plagiarism --version  print the version");
}

fn run_scan_mode() -> ExitCode {
    let started = Instant::now();

    let mut raw = Vec::new();
    if let Err(err) = io::stdin()
        .take(MAX_INPUT_BYTES as u64 + 1)
        .read_to_end(&mut raw)
    {
        emit_error(&format!("failed to read stdin: {}", err));
        return ExitCode::from(2);
    }
    if raw.len() > MAX_INPUT_BYTES {
        emit_error(&format!(
            "request exceeds the {} byte limit",
            MAX_INPUT_BYTES
        ));
        return ExitCode::from(2);
    }

    let request = match parse_request(&raw) {
        Ok(req) => req,
        Err(message) => {
            emit_error(&message);
            return ExitCode::from(2);
        }
    };
    drop(raw);

    let output = engine::scan(&request.inputs, request.threshold);

    let stdout = io::stdout();
    let mut out = BufWriter::new(stdout.lock());

    let flagged = output.hits.len();
    for hit in &output.hits {
        let line = json!({
            "pair": {
                "problem_id": hit.problem_id,
                "a": hit.a,
                "b": hit.b,
                "similarity": hit.similarity,
                "matched_tokens": hit.matched_tokens,
            }
        });
        if let Err(err) = write_line(&mut out, &line) {
            emit_error(&format!("failed to write result: {}", err));
            return ExitCode::from(2);
        }
    }

    let summary = json!({
        "summary": {
            "inputs": request.inputs.len(),
            "pairs_total": output.pairs_total,
            "pairs_scored": output.pairs_scored,
            "flagged": flagged,
            "engine": "rust",
            "elapsed_ms": started.elapsed().as_millis() as u64,
        }
    });
    if let Err(err) = write_line(&mut out, &summary) {
        emit_error(&format!("failed to write summary: {}", err));
        return ExitCode::from(2);
    }
    if let Err(err) = out.flush() {
        emit_error(&format!("failed to flush results: {}", err));
        return ExitCode::from(2);
    }
    ExitCode::SUCCESS
}

/// Parse and validate the request. Every rejection names the offending field, so
/// a protocol change in the Go service shows up as a precise message in the log
/// instead of "invalid request".
fn parse_request(raw: &[u8]) -> Result<Request, String> {
    let value: Value =
        serde_json::from_slice(raw).map_err(|err| format!("invalid request: {}", err))?;

    // serde_json rejects NaN/Infinity spellings ("number out of range") and has no
    // literal for them, so a parsed f64 here is always finite. `engine::scan` still
    // tolerates a NaN threshold defensively.
    let threshold = value
        .get("threshold")
        .and_then(Value::as_f64)
        .ok_or("threshold must be a number")?;

    let raw_inputs = value
        .get("inputs")
        .and_then(Value::as_array)
        .ok_or("inputs must be an array")?;

    let mut inputs = Vec::with_capacity(raw_inputs.len());
    for (idx, item) in raw_inputs.iter().enumerate() {
        inputs.push(Input {
            key: string_field(item, "key").map_err(|err| format!("inputs[{}].{}", idx, err))?,
            problem_id: string_field(item, "problem_id")
                .map_err(|err| format!("inputs[{}].{}", idx, err))?,
            user_id: string_field(item, "user_id")
                .map_err(|err| format!("inputs[{}].{}", idx, err))?,
            code: string_field(item, "code").map_err(|err| format!("inputs[{}].{}", idx, err))?,
        });
    }

    Ok(Request { threshold, inputs })
}

fn string_field(value: &Value, field: &str) -> Result<String, String> {
    value
        .get(field)
        .and_then(Value::as_str)
        .map(str::to_owned)
        .ok_or_else(|| format!("{} must be a string", field))
}

fn run_tokens_mode() -> ExitCode {
    let mut raw = Vec::new();
    if let Err(err) = io::stdin().read_to_end(&mut raw) {
        emit_error(&format!("failed to read stdin: {}", err));
        return ExitCode::from(2);
    }
    let tokens = tokenizer::tokenize(&raw);
    let stdout = io::stdout();
    let mut out = BufWriter::new(stdout.lock());
    if writeln!(out, "{}", tokens.join()).is_err() {
        return ExitCode::from(2);
    }
    let _ = out.flush();
    ExitCode::SUCCESS
}

fn write_line(out: &mut impl Write, value: &Value) -> io::Result<()> {
    serde_json::to_writer(&mut *out, value)?;
    out.write_all(b"\n")
}

/// Emit an error as JSON on stdout so a Go caller always receives a parseable
/// stream, then exit non-zero so the failure is still visible to the shell.
fn emit_error(message: &str) {
    let stdout = io::stdout();
    let mut out = BufWriter::new(stdout.lock());
    let _ = write_line(&mut out, &json!({ "error": message }));
    let _ = out.flush();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_a_minimal_request() {
        let request = parse_request(
            br#"{"threshold":0.75,"inputs":[
                {"key":"s1","problem_id":"p1","user_id":"u1","code":"int main(){}"}
            ]}"#,
        )
        .expect("valid request");
        assert!((request.threshold - 0.75).abs() < f64::EPSILON);
        assert_eq!(request.inputs.len(), 1);
        assert_eq!(request.inputs[0].key, "s1");
        assert_eq!(request.inputs[0].code, "int main(){}");
    }

    #[test]
    fn preserves_escapes_in_source_code() {
        // Source text is full of characters that must survive JSON escaping:
        // quotes, backslashes, newlines, tabs and non-ASCII. If this drifts, the
        // similarity scores drift with it.
        let code = "printf(\"a\\\\b\\n\\t\"); int café = 1; // ünïcode ✓";
        let request = parse_request(
            format!(
                r#"{{"threshold":0.5,"inputs":[{{"key":"k","problem_id":"p","user_id":"u","code":{}}}]}}"#,
                serde_json::to_string(code).unwrap()
            )
            .as_bytes(),
        )
        .expect("valid request");
        assert_eq!(request.inputs[0].code, code);
    }

    #[test]
    fn rejects_malformed_requests_with_a_named_field() {
        for (body, want) in [
            (r#"{"inputs":[]}"#, "threshold"),
            (r#"{"threshold":"x","inputs":[]}"#, "threshold"),
            (r#"{"threshold":0.5}"#, "inputs"),
            (
                r#"{"threshold":0.5,"inputs":[{"problem_id":"p","user_id":"u","code":"c"}]}"#,
                "inputs[0].key",
            ),
            (
                r#"{"threshold":0.5,"inputs":[{"key":"k","problem_id":"p","user_id":"u"}]}"#,
                "inputs[0].code",
            ),
        ] {
            let err = parse_request(body.as_bytes()).expect_err("must be rejected");
            assert!(err.contains(want), "error {:?} should name {}", err, want);
        }
    }

    #[test]
    fn rejects_numbers_outside_the_f64_range() {
        // Guards the assumption above: if serde_json ever accepted an overflowing
        // exponent as infinity, the threshold would silently match nothing.
        let err = parse_request(br#"{"threshold":1e400,"inputs":[]}"#)
            .expect_err("an overflowing exponent must be rejected");
        assert!(err.contains("out of range"), "unexpected error: {}", err);
    }

    #[test]
    fn emits_pairs_and_a_summary_line() {
        let raw = br#"{"threshold":0.9,"inputs":[
            {"key":"a","problem_id":"p","user_id":"u1","code":"int main(){return 0;}"},
            {"key":"b","problem_id":"p","user_id":"u2","code":"int main(){return 0;}"}
        ]}"#;
        let request = parse_request(raw).expect("valid");
        let output = engine::scan(&request.inputs, request.threshold);
        assert_eq!(output.hits.len(), 1);
        assert_eq!(output.hits[0].similarity, 1.0);
        assert_eq!(output.pairs_total, 1);
    }
}
