//! Tokenization that reproduces `internal/plagiarism/engine.go` byte for byte.
//!
//! The Go implementation is a five-pass regex pipeline:
//!
//! ```text
//! (?m)//.*$            -> " "    strip line comments
//! (?s)/\*.*?\*/        -> " "    strip block comments
//! "[^"]*"|'[^']*'      -> " STR " strings become one token
//! \b\d+(\.\d+)?\b      -> " NUM " numbers become one token
//! \s+                  -> " "    then TrimSpace, then strings.Fields
//! ```
//!
//! The last step collapses every whitespace run to a single space and then
//! splits on it, so only token *boundaries* survive normalization. This module
//! therefore emits tokens directly instead of materializing the collapsed
//! string: `strings.Fields(NormalizeCode(code))` and `tokenize(code)` yield the
//! same sequence. `internal/plagiarism/engine_parity_test.go` asserts that
//! claim against the Go implementation on a corpus rather than trusting it.
//!
//! Every pass here deliberately reproduces Go's behaviour *including its
//! quirks*, because a "better" tokenizer would change similarity scores and
//! silently re-classify submissions. Two quirks are worth naming:
//!
//!   * `//` is stripped even inside a string literal, so `"http://x"` loses
//!     everything from `//` onward. That is what the Go regex does.
//!   * An unterminated `/*` is left in place, because the Go pattern requires a
//!     closing `*/` before it matches at all.
//!
//! The passes run on bytes: none of the patterns can match across a UTF-8
//! continuation byte, and invalid UTF-8 is not whitespace under
//! `unicode.IsSpace`, so the byte-oriented scan agrees with Go's rune-oriented
//! one while avoiding a decode per character.

/// Longest input accepted by the engine, in bytes. A report covers at most a
/// few hundred submissions per problem; anything past this is not a code review
/// request and must not become an out-of-memory crash in a background worker.
pub const MAX_INPUT_BYTES: usize = 256 * 1024 * 1024;

const STR_BYTES: &[u8] = b"STR";
const NUM_BYTES: &[u8] = b"NUM";

/// A tokenized source: normalized bytes plus the span of each token.
pub struct Tokens {
    buf: Vec<u8>,
    spans: Vec<(u32, u32)>,
}

impl Tokens {
    pub fn len(&self) -> usize {
        self.spans.len()
    }

    // Only the tests ask this question today; keeping it next to `len` keeps the
    // pair honest without silencing the lint for the whole file.
    #[cfg_attr(not(test), allow(dead_code))]
    pub fn is_empty(&self) -> bool {
        self.spans.is_empty()
    }

    pub fn iter(&self) -> impl Iterator<Item = &[u8]> {
        self.spans
            .iter()
            .map(move |&(s, e)| &self.buf[s as usize..e as usize])
    }

    /// Tokens joined by single spaces — the same shape as
    /// `strings.Join(Tokenize(code), " ")`, used by the `--tokens` mode so the
    /// Go parity test can diff token streams without re-deriving them.
    pub fn join(&self) -> String {
        let mut out = String::with_capacity(self.buf.len());
        for (i, tok) in self.iter().enumerate() {
            if i > 0 {
                out.push(' ');
            }
            // Tokens are slices of the input, which the caller already holds as
            // valid UTF-8; fall back to a lossy copy rather than panicking if a
            // token boundary ever split a character.
            out.push_str(std::str::from_utf8(tok).unwrap_or(""));
        }
        out
    }
}

/// `unicode.IsSpace`: Go's Unicode White_Space property, which is what
/// `strings.Fields` splits on.
fn is_unicode_space(cp: u32) -> bool {
    matches!(
        cp,
        0x0009..=0x000D
            | 0x0020
            | 0x0085
            | 0x00A0
            | 0x1680
            | 0x2000..=0x200A
            | 0x2028
            | 0x2029
            | 0x202F
            | 0x205F
            | 0x3000
    )
}

/// Decode one UTF-8 scalar at `i`. Returns `(codepoint, byte length)`; invalid
/// sequences decode to U+FFFD with length 1, matching how Go's regexp engine
/// treats them. U+FFFD is never whitespace, so invalid bytes never separate
/// tokens in either implementation.
fn decode_utf8(b: &[u8], i: usize) -> (u32, usize) {
    let c0 = b[i];
    if c0 < 0x80 {
        return (c0 as u32, 1);
    }
    let (len, mut cp) = if c0 & 0xE0 == 0xC0 {
        (2usize, (c0 & 0x1F) as u32)
    } else if c0 & 0xF0 == 0xE0 {
        (3usize, (c0 & 0x0F) as u32)
    } else if c0 & 0xF8 == 0xF0 {
        (4usize, (c0 & 0x07) as u32)
    } else {
        return (0xFFFD, 1);
    };
    if i + len > b.len() {
        return (0xFFFD, 1);
    }
    for k in 1..len {
        let cc = b[i + k];
        if cc & 0xC0 != 0x80 {
            return (0xFFFD, 1);
        }
        cp = (cp << 6) | (cc & 0x3F) as u32;
    }
    (cp, len)
}

/// Go regexp `\b`: an ASCII word boundary. `\w` is `[0-9A-Za-z_]` in RE2.
#[inline]
fn is_word_byte(b: u8) -> bool {
    b.is_ascii_alphanumeric() || b == b'_'
}

/// Pass 1 — `(?m)//.*$` replaced by a single space.
fn strip_line_comments(src: &[u8], out: &mut Vec<u8>) {
    out.clear();
    out.reserve(src.len());
    let n = src.len();
    let mut i = 0;
    while i < n {
        if src[i] == b'/' && i + 1 < n && src[i + 1] == b'/' {
            // `.` does not match `\n`, so the match ends just before the
            // newline; the newline itself survives.
            while i < n && src[i] != b'\n' {
                i += 1;
            }
            out.push(b' ');
        } else {
            out.push(src[i]);
            i += 1;
        }
    }
}

/// Pass 2 — `(?s)/\*.*?\*/` replaced by a single space.
///
/// Non-greedy and dot-matches-newline, so it spans lines. An unterminated `/*`
/// does not match and is copied through unchanged.
fn strip_block_comments(src: &[u8], out: &mut Vec<u8>) {
    out.clear();
    out.reserve(src.len());
    let n = src.len();
    let mut i = 0;
    while i < n {
        if src[i] == b'/' && i + 1 < n && src[i + 1] == b'*' {
            match find_close(src, i + 2) {
                Some(end) => {
                    out.push(b' ');
                    i = end;
                    continue;
                }
                None => {
                    // No terminator anywhere ahead: this `/*` cannot start a
                    // match, so the regex would retry at the next position.
                    out.extend_from_slice(b"/*");
                    i += 2;
                    continue;
                }
            }
        }
        out.push(src[i]);
        i += 1;
    }
}

fn find_close(src: &[u8], from: usize) -> Option<usize> {
    let mut i = from;
    while i + 1 < src.len() {
        if src[i] == b'*' && src[i + 1] == b'/' {
            return Some(i + 2);
        }
        i += 1;
    }
    None
}

/// Match a `"`/`'` literal starting at `open`. Returns the index just past the
/// closing quote.
///
/// Mirrors `(?:[^"\\]|\\.)*`: `[^"\\]` matches any byte but the quote and
/// backslash (newlines included), and `\\.` matches a backslash plus one byte
/// that is not a newline. A backslash immediately followed by a newline matches
/// neither alternative, so the literal fails to terminate at this position —
/// exactly as the regex engine would report no match here.
fn match_string(src: &[u8], open: usize, quote: u8) -> Option<usize> {
    let n = src.len();
    let mut j = open + 1;
    while j < n {
        let c = src[j];
        if c == quote {
            return Some(j + 1);
        }
        if c == b'\\' {
            if j + 1 < n && src[j + 1] != b'\n' {
                j += 2;
                continue;
            }
            return None;
        }
        j += 1;
    }
    None
}

/// Passes 3 and 4 — string and number literals become the `STR` / `NUM` tokens,
/// surrounded by spaces so they can never fuse with neighbouring punctuation.
fn substitute_literals(src: &[u8], out: &mut Vec<u8>) {
    out.clear();
    out.reserve(src.len());
    let n = src.len();
    let mut i = 0;
    while i < n {
        let c = src[i];
        if c == b'"' || c == b'\'' {
            if let Some(end) = match_string(src, i, c) {
                out.push(b' ');
                out.extend_from_slice(STR_BYTES);
                out.push(b' ');
                i = end;
                continue;
            }
            out.push(c);
            i += 1;
            continue;
        }
        if c.is_ascii_digit() {
            // `\b` before: the preceding byte must not be a word byte. Since
            // `c` is a digit (a word byte), a boundary exists iff there is no
            // preceding word byte.
            if i > 0 && is_word_byte(src[i - 1]) {
                out.push(c);
                i += 1;
                continue;
            }
            let mut j = i;
            while j < n && src[j].is_ascii_digit() {
                j += 1;
            }
            // Optional `(?:\.\d+)?` — requires at least one digit after the dot.
            if j + 1 < n && src[j] == b'.' && src[j + 1].is_ascii_digit() {
                let mut k = j + 1;
                while k < n && src[k].is_ascii_digit() {
                    k += 1;
                }
                j = k;
            }
            // `\b` after. If it fails, the greedy `\d+` cannot be shortened into
            // a match either (every shorter prefix ends between two digits), so
            // the regex would advance one byte and retry — emulate that.
            if j >= n || !is_word_byte(src[j]) {
                out.push(b' ');
                out.extend_from_slice(NUM_BYTES);
                out.push(b' ');
                i = j;
                continue;
            }
            out.push(c);
            i += 1;
            continue;
        }
        // Collapse whitespace runs while copying: Go's `\s+` -> " " followed by
        // `strings.Fields`, which splits on Unicode spaces.
        if c < 0x80 {
            if is_unicode_space(c as u32) {
                while i < n && src[i] < 0x80 && is_unicode_space(src[i] as u32) {
                    i += 1;
                }
                out.push(b' ');
                continue;
            }
            out.push(c);
            i += 1;
            continue;
        }
        let (cp, len) = decode_utf8(src, i);
        if is_unicode_space(cp) {
            i += len;
            out.push(b' ');
            continue;
        }
        out.extend_from_slice(&src[i..i + len]);
        i += len;
    }
}

/// Split the normalized buffer into tokens on single spaces.
fn collect_tokens(buf: &[u8], spans: &mut Vec<(u32, u32)>) {
    spans.clear();
    let mut start = 0usize;
    for (i, &byte) in buf.iter().enumerate() {
        if byte == b' ' {
            if i > start {
                spans.push((start as u32, i as u32));
            }
            start = i + 1;
        }
    }
    if buf.len() > start {
        spans.push((start as u32, buf.len() as u32));
    }
}

/// Tokenize one source file. See the module docs for the pipeline.
pub fn tokenize(code: &[u8]) -> Tokens {
    let mut no_line = Vec::new();
    strip_line_comments(code, &mut no_line);

    let mut no_block = Vec::new();
    strip_block_comments(&no_line, &mut no_block);

    let mut literals = Vec::new();
    substitute_literals(&no_block, &mut literals);

    let mut spans = Vec::new();
    collect_tokens(&literals, &mut spans);
    Tokens {
        buf: literals,
        spans,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn toks(s: &str) -> Vec<String> {
        tokenize(s.as_bytes())
            .iter()
            .map(|t| String::from_utf8_lossy(t).into_owned())
            .collect()
    }

    // These four expectations are copied from engine_test.go, which asserts the
    // same normalizations on the Go side.
    #[test]
    fn strips_line_comments() {
        assert_eq!(
            toks("int x = 5; // this is a comment"),
            vec!["int", "x", "=", "NUM", ";"]
        );
    }

    #[test]
    fn replaces_strings() {
        assert_eq!(
            toks(r#"printf("hello world");"#),
            vec!["printf(", "STR", ");"]
        );
    }

    #[test]
    fn replaces_numbers() {
        assert_eq!(toks("int x = 42;"), vec!["int", "x", "=", "NUM", ";"]);
    }

    #[test]
    fn strips_block_comments() {
        assert_eq!(
            toks("int /* foo */ x = 5;"),
            vec!["int", "x", "=", "NUM", ";"]
        );
    }

    #[test]
    fn empty_input_has_no_tokens() {
        assert!(tokenize(b"").is_empty());
        assert!(tokenize(b"   \n\t ").is_empty());
        assert!(tokenize(b"// only a comment").is_empty());
    }

    #[test]
    fn unterminated_block_comment_is_kept() {
        // The Go pattern needs a closing `*/`, so it never matches here.
        assert_eq!(toks("int x /* half"), vec!["int", "x", "/*", "half"]);
    }

    #[test]
    fn line_comment_inside_string_is_still_stripped() {
        // Pass 1 runs before pass 3, so `//` wins even inside a literal: the
        // tail of the literal is deleted and the string no longer terminates, so
        // the quote survives as an ordinary byte. This is the Go behaviour and
        // must not be "fixed" here.
        assert_eq!(
            toks(r#"a = "http://example.com";"#),
            vec!["a", "=", "\"http:"]
        );
    }

    #[test]
    fn digits_glued_to_letters_are_not_numbers() {
        assert_eq!(toks("123abc"), vec!["123abc"]);
        assert_eq!(toks("0x1F"), vec!["0x1F"]);
        assert_eq!(toks("a1"), vec!["a1"]);
    }

    #[test]
    fn multiple_dots_become_separate_numbers() {
        assert_eq!(toks("12.34.56"), vec!["NUM", ".", "NUM"]);
        // `\b` cannot hold between `v` and `1`, so only `5` becomes a number and
        // the replacement's leading space is what splits `v1.` from `NUM`.
        assert_eq!(toks("v1.5"), vec!["v1.", "NUM"]);
    }

    #[test]
    fn escaped_quotes_do_not_end_the_literal() {
        assert_eq!(toks(r#""a\"b""#), vec!["STR"]);
        assert_eq!(toks("'\\''"), vec!["STR"]);
    }

    #[test]
    fn newlines_inside_literals_are_allowed() {
        assert_eq!(toks("\"a\nb\""), vec!["STR"]);
    }

    #[test]
    fn vertical_tab_is_a_separator() {
        // Go: `\s` excludes 0x0B so the collapse leaves it, but strings.Fields
        // splits on it, so it still separates tokens.
        assert_eq!(toks("a\x0bb"), vec!["a", "b"]);
    }

    #[test]
    fn unicode_spaces_separate_tokens() {
        assert_eq!(toks("a\u{00a0}b"), vec!["a", "b"]);
        assert_eq!(toks("a\u{3000}b"), vec!["a", "b"]);
    }

    #[test]
    fn unicode_identifiers_survive() {
        assert_eq!(toks("int café = 1;"), vec!["int", "café", "=", "NUM", ";"]);
    }

    #[test]
    fn token_bytes_are_lossless() {
        let src = "int main() { return 0; }";
        assert_eq!(
            tokenize(src.as_bytes()).join(),
            "int main() { return NUM ; }"
        );
    }

    #[test]
    fn overlapping_comment_markers() {
        // `/ *` is not a block comment; `/*/` opens but never closes.
        assert_eq!(toks("a / * b"), vec!["a", "/", "*", "b"]);
        assert_eq!(toks("a /*/ b"), vec!["a", "/*/", "b"]);
        // Non-greedy: the match ends at the *first* `*/`.
        assert_eq!(toks("a /* b /* c */ d"), vec!["a", "d"]);
    }
}
