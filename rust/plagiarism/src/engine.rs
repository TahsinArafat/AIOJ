//! Pairwise similarity scan over a batch of submissions.
//!
//! The scan is exact: it reports every pair whose similarity is `>= threshold`,
//! with the same numbers the Go implementation would produce. Speed comes from
//! three changes of shape, none of which alter the result:
//!
//!   * sources are tokenized **once**, not twice per pair;
//!   * a multiset-intersection bound rejects pairs that provably cannot reach
//!     the threshold (`lcs::similarity_bound`) before any LCS runs;
//!   * surviving pairs are scored in parallel across cores.

use rayon::prelude::*;
use std::collections::HashMap;

use crate::lcs::{intersection_size, lcs_len, similarity_bound, MaskTable};
use crate::tokenizer::{tokenize, Tokens};

/// One submission to compare, as the Go service sends it.
#[derive(Debug)]
pub struct Input {
    pub key: String,
    pub problem_id: String,
    pub user_id: String,
    pub code: String,
}

/// A pair that crossed the threshold.
pub struct Hit {
    pub problem_id: String,
    pub a: String,
    pub b: String,
    pub similarity: f64,
    pub matched_tokens: u32,
}

pub struct ScanOutput {
    pub hits: Vec<Hit>,
    /// Pairs that were eligible for comparison: same problem, different author.
    /// This is the denominator the report shows, so it must match the Go
    /// engine's count exactly.
    pub pairs_total: usize,
    /// Pairs that actually ran an LCS. Surfaced in the summary line so the
    /// prefilter's effectiveness is observable in production, not guessed at.
    pub pairs_scored: usize,
}

/// A tokenized source plus the derived structures reused by every pair it
/// participates in.
struct Prepared {
    ids: Vec<u32>,
    /// `(symbol, count)` sorted by symbol, for the intersection bound.
    counts: Vec<(u32, u32)>,
}

impl Prepared {
    fn len(&self) -> usize {
        self.ids.len()
    }
}

/// One scored pair, carrying indices rather than owned strings so the parallel
/// section allocates nothing per hit.
struct ScoredPair {
    a: usize,
    b: usize,
    lcs: u32,
    similarity: f64,
}

/// What one chunk of candidates produced.
struct ChunkOutcome {
    /// Candidates that were scored at all (i.e. not rejected by the bound).
    scored: usize,
    hits: Vec<ScoredPair>,
}

/// Why a candidate did not become a hit. Kept explicit rather than folded into
/// `Option` so `pairs_scored` reports the truth about what the bound saved.
enum Outcome {
    Hit {
        lcs: u32,
        similarity: f64,
        lcs_ran: bool,
    },
    BelowThreshold {
        lcs_ran: bool,
    },
    BoundSkipped,
}

pub fn scan(inputs: &[Input], threshold: f64) -> ScanOutput {
    // ---- 1. Tokenize once per source, interning tokens to dense u32 ids. ----
    //
    // Ids are assigned by exact byte equality. Hashing tokens to fixed-width
    // fingerprints would be faster and would occasionally equate two different
    // tokens, which would silently raise a similarity score — not acceptable in
    // a tool used to accuse people.
    let mut interner: HashMap<Vec<u8>, u32> = HashMap::new();
    let mut prepared: Vec<Prepared> = Vec::with_capacity(inputs.len());

    for input in inputs {
        let tokens: Tokens = tokenize(input.code.as_bytes());
        let mut ids: Vec<u32> = Vec::with_capacity(tokens.len());
        for tok in tokens.iter() {
            let next_id = interner.len() as u32;
            let id = *interner.entry(tok.to_vec()).or_insert(next_id);
            ids.push(id);
        }
        prepared.push(Prepared {
            counts: sorted_counts(&ids),
            ids,
        });
    }

    let vocab = interner.len();
    let max_len = prepared.iter().map(|p| p.len()).max().unwrap_or(0);
    drop(interner);

    // ---- 2. Enumerate candidates: same problem, different author. ----------
    //
    // Grouping by problem keeps the report's pair accounting identical to the
    // Go engine, which compares within one problem at a time.
    let mut groups: HashMap<&str, Vec<usize>> = HashMap::new();
    for (idx, input) in inputs.iter().enumerate() {
        groups
            .entry(input.problem_id.as_str())
            .or_default()
            .push(idx);
    }

    let mut candidates: Vec<(usize, usize)> = Vec::new();
    for members in groups.values() {
        for (i, &a) in members.iter().enumerate() {
            for &b in &members[i + 1..] {
                // One person's own submissions are never a plagiarism pair.
                if inputs[a].user_id == inputs[b].user_id {
                    continue;
                }
                candidates.push((a, b));
            }
        }
    }

    let pairs_total = candidates.len();
    if pairs_total == 0 || threshold.is_nan() {
        return ScanOutput {
            hits: Vec::new(),
            pairs_total,
            pairs_scored: 0,
        };
    }

    // ---- 3. Score in parallel, with a per-thread mask table. ---------------
    const CHUNK: usize = 512;

    // Each chunk reports its hits and how many candidates needed a real LCS, so
    // the cost counter comes straight from the scorer and cannot drift from it.
    let chunk_results: Vec<ChunkOutcome> = candidates
        .par_chunks(CHUNK)
        .map(|chunk| {
            // One table per worker, reused across the whole chunk: sizing it is
            // the only per-thread setup cost.
            let mut table = MaskTable::new(vocab.max(1), max_len.max(1));
            let mut outcome = ChunkOutcome {
                scored: 0,
                hits: Vec::new(),
            };
            for &(ia, ib) in chunk {
                match score_pair(&prepared[ia], &prepared[ib], threshold, &mut table) {
                    Outcome::Hit {
                        lcs,
                        similarity,
                        lcs_ran,
                    } => {
                        if lcs_ran {
                            outcome.scored += 1;
                        }
                        outcome.hits.push(ScoredPair {
                            a: ia,
                            b: ib,
                            lcs,
                            similarity,
                        });
                    }
                    // Scored but below the threshold. Counted only when a dynamic
                    // program actually ran, so `pairs_scored` measures LCS work
                    // rather than "pairs that were not filtered".
                    Outcome::BelowThreshold { lcs_ran } => {
                        if lcs_ran {
                            outcome.scored += 1;
                        }
                    }
                    Outcome::BoundSkipped => {}
                }
            }
            outcome
        })
        .collect();

    let mut hits: Vec<Hit> = Vec::new();
    let mut pairs_scored = 0usize;
    for outcome in chunk_results {
        pairs_scored += outcome.scored;
        for scored_pair in outcome.hits {
            hits.push(Hit {
                problem_id: inputs[scored_pair.a].problem_id.clone(),
                a: inputs[scored_pair.a].key.clone(),
                b: inputs[scored_pair.b].key.clone(),
                similarity: scored_pair.similarity,
                matched_tokens: scored_pair.lcs,
            });
        }
    }

    // Deterministic order regardless of how work was scheduled, so identical
    // runs produce identical report rows and diffs stay readable.
    hits.sort_by(|x, y| {
        x.problem_id
            .cmp(&y.problem_id)
            .then_with(|| x.a.cmp(&y.a))
            .then_with(|| x.b.cmp(&y.b))
    });

    ScanOutput {
        hits,
        pairs_total,
        pairs_scored,
    }
}

fn sorted_counts(ids: &[u32]) -> Vec<(u32, u32)> {
    let mut v = ids.to_vec();
    v.sort_unstable();
    let mut out: Vec<(u32, u32)> = Vec::with_capacity(v.len());
    for x in v {
        match out.last_mut() {
            Some(last) if last.0 == x => last.1 += 1,
            _ => out.push((x, 1)),
        }
    }
    out
}

/// Score one candidate.
fn score_pair(a: &Prepared, b: &Prepared, threshold: f64, table: &mut MaskTable) -> Outcome {
    let (la, lb) = (a.len(), b.len());

    // `LCSSimilarity` returns 0 when either side has no tokens.
    if la == 0 || lb == 0 {
        return if 0.0 >= threshold {
            Outcome::Hit {
                lcs: 0,
                similarity: 0.0,
                lcs_ran: false,
            }
        } else {
            Outcome::BelowThreshold { lcs_ran: false }
        };
    }

    // Exact fast path: identical token streams are the common case inside a
    // copy-paste cluster and need no dynamic program at all.
    if la == lb && a.ids == b.ids {
        return if 1.0 >= threshold {
            Outcome::Hit {
                lcs: la as u32,
                similarity: 1.0,
                lcs_ran: false,
            }
        } else {
            Outcome::BelowThreshold { lcs_ran: false }
        };
    }

    let shared = intersection_size(&a.counts, &b.counts);
    if similarity_bound(&a.counts, &b.counts, shared) < threshold {
        return Outcome::BoundSkipped;
    }

    // Pass the shorter side as the pattern: same result, smaller mask table.
    let (pattern, text) = if la <= lb {
        (&a.ids, &b.ids)
    } else {
        (&b.ids, &a.ids)
    };
    let lcs = lcs_len(pattern, text, table);

    // Same expression as Go's LCSSimilarity, in the same order, so the float is
    // bit-identical and a report generated by either engine matches.
    let similarity = 2.0 * lcs as f64 / (la + lb) as f64;

    if similarity >= threshold {
        Outcome::Hit {
            lcs,
            similarity,
            lcs_ran: true,
        }
    } else {
        Outcome::BelowThreshold { lcs_ran: true }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn input(key: &str, problem: &str, user: &str, code: &str) -> Input {
        Input {
            key: key.to_string(),
            problem_id: problem.to_string(),
            user_id: user.to_string(),
            code: code.to_string(),
        }
    }

    fn naive_similarity(a: &[String], b: &[String]) -> f64 {
        if a.is_empty() || b.is_empty() {
            return 0.0;
        }
        let mut prev = vec![0u32; b.len() + 1];
        let mut cur = vec![0u32; b.len() + 1];
        for x in a {
            for j in 1..=b.len() {
                cur[j] = if *x == b[j - 1] {
                    prev[j - 1] + 1
                } else {
                    prev[j].max(cur[j - 1])
                };
            }
            std::mem::swap(&mut prev, &mut cur);
        }
        2.0 * prev[b.len()] as f64 / (a.len() + b.len()) as f64
    }

    /// Oracle: every eligible pair scored with a plain dynamic program.
    fn reference_scan(inputs: &[Input], threshold: f64) -> (Vec<(String, String, f64)>, usize) {
        let toks: Vec<Vec<String>> = inputs
            .iter()
            .map(|i| {
                tokenize(i.code.as_bytes())
                    .iter()
                    .map(|t| String::from_utf8_lossy(t).into_owned())
                    .collect()
            })
            .collect();
        let mut out = Vec::new();
        let mut total = 0;
        for a in 0..inputs.len() {
            for b in a + 1..inputs.len() {
                if inputs[a].problem_id != inputs[b].problem_id
                    || inputs[a].user_id == inputs[b].user_id
                {
                    continue;
                }
                total += 1;
                let sim = naive_similarity(&toks[a], &toks[b]);
                if sim >= threshold {
                    out.push((inputs[a].key.clone(), inputs[b].key.clone(), sim));
                }
            }
        }
        (out, total)
    }

    const A: &str =
        "int main() { int n; cin >> n; for (int i = 0; i < n; i++) sum += i; cout << sum; }";
    const A_RENAMED: &str =
        "int main() { int m; cin >> m; for (int j = 0; j < m; j++) total += j; cout << total; }";
    const B: &str = "def solve():\n    n = int(input())\n    print(sum(range(n)))\n";

    #[test]
    fn identical_sources_score_one() {
        let inputs = vec![input("s1", "p1", "u1", A), input("s2", "p1", "u2", A)];
        let out = scan(&inputs, 0.7);
        assert_eq!(out.hits.len(), 1);
        assert_eq!(out.hits[0].similarity, 1.0);
        let want = tokenize(A.as_bytes()).len() as u32;
        assert_eq!(out.hits[0].matched_tokens, want);
    }

    #[test]
    fn renamed_variables_score_partially() {
        // Renaming `n`/`i`/`sum` leaves the structure but not the identifiers:
        // 16 of 25 tokens line up, so the true similarity is 0.64. Pinned as an
        // exact value because it is the number an admin would see in a report,
        // and because a tokenizer change would move it.
        let inputs = vec![
            input("s1", "p1", "u1", A),
            input("s2", "p1", "u2", A_RENAMED),
        ];

        let flagged = scan(&inputs, 0.6);
        assert_eq!(flagged.hits.len(), 1, "a renamed copy must clear 0.6");
        assert!((flagged.hits[0].similarity - 0.64).abs() < 1e-12);
        assert_eq!(flagged.hits[0].matched_tokens, 16);

        // ...but it is correctly *not* a flag at the default threshold.
        assert!(scan(&inputs, 0.7).hits.is_empty());
    }

    #[test]
    fn unrelated_sources_are_not_flagged() {
        let inputs = vec![input("s1", "p1", "u1", A), input("s2", "p1", "u2", B)];
        let out = scan(&inputs, 0.7);
        assert!(out.hits.is_empty());
        assert_eq!(out.pairs_total, 1);
    }

    #[test]
    fn same_author_is_never_a_pair() {
        let inputs = vec![input("s1", "p1", "u1", A), input("s2", "p1", "u1", A)];
        let out = scan(&inputs, 0.7);
        assert!(out.hits.is_empty());
        assert_eq!(out.pairs_total, 0);
    }

    #[test]
    fn different_problems_are_never_compared() {
        let inputs = vec![input("s1", "p1", "u1", A), input("s2", "p2", "u2", A)];
        let out = scan(&inputs, 0.7);
        assert!(out.hits.is_empty());
        assert_eq!(out.pairs_total, 0);
    }

    #[test]
    fn empty_sources_do_not_flag_at_a_positive_threshold() {
        let inputs = vec![input("s1", "p1", "u1", ""), input("s2", "p1", "u2", "")];
        let out = scan(&inputs, 0.7);
        assert!(out.hits.is_empty());
        assert_eq!(out.pairs_total, 1);
        // An empty pair cannot reach a positive threshold, so no LCS should run.
        assert_eq!(out.pairs_scored, 0);
    }

    #[test]
    fn zero_threshold_flags_empty_sources_like_go() {
        // Go: LCSSimilarity returns 0.0, and 0.0 >= 0 is a hit.
        let inputs = vec![input("s1", "p1", "u1", ""), input("s2", "p1", "u2", "")];
        let out = scan(&inputs, 0.0);
        assert_eq!(out.hits.len(), 1);
        assert_eq!(out.hits[0].similarity, 0.0);
    }

    #[test]
    fn matches_naive_scan_on_a_mixed_corpus() {
        let mut inputs = Vec::new();
        let variants = [A, A_RENAMED, B];
        for p in 0..3 {
            for u in 0..4 {
                for (v, code) in variants.iter().enumerate() {
                    inputs.push(input(
                        &format!("p{}u{}v{}", p, u, v),
                        &format!("problem{}", p),
                        &format!("user{}", u),
                        code,
                    ));
                }
            }
        }
        for threshold in [0.3, 0.5, 0.7, 0.9, 1.0] {
            let got = scan(&inputs, threshold);
            let (want, want_total) = reference_scan(&inputs, threshold);
            assert_eq!(got.pairs_total, want_total, "threshold {}", threshold);

            let mut got_pairs: Vec<(String, String)> = got
                .hits
                .iter()
                .map(|h| (h.a.clone(), h.b.clone()))
                .collect();
            got_pairs.sort();
            let mut want_pairs: Vec<(String, String)> =
                want.iter().map(|w| (w.0.clone(), w.1.clone())).collect();
            want_pairs.sort();
            assert_eq!(got_pairs, want_pairs, "threshold {}", threshold);

            // Similarity values, not just membership, must match the oracle.
            for want_hit in &want {
                let got_hit = got
                    .hits
                    .iter()
                    .find(|h| h.a == want_hit.0 && h.b == want_hit.1)
                    .expect("hit present");
                assert!(
                    (want_hit.2 - got_hit.similarity).abs() < 1e-12,
                    "similarity mismatch {} vs {}",
                    want_hit.2,
                    got_hit.similarity
                );
            }
        }
    }

    #[test]
    fn prefilter_never_drops_a_flagged_pair() {
        // A low threshold forces most pairs through; a high one leans on the
        // bound. Either way the hit set must equal the oracle's.
        let mut inputs = Vec::new();
        for u in 0..8 {
            let code = if u % 2 == 0 { A } else { A_RENAMED };
            inputs.push(input(&format!("s{}", u), "p1", &format!("u{}", u), code));
        }
        for threshold in [0.05, 0.25, 0.5, 0.75, 0.95, 1.0] {
            let got = scan(&inputs, threshold);
            let (want, _) = reference_scan(&inputs, threshold);
            assert_eq!(got.hits.len(), want.len(), "threshold {}", threshold);
        }
    }

    #[test]
    fn output_order_is_deterministic() {
        let mut inputs = Vec::new();
        for p in 0..2 {
            for u in 0..6 {
                inputs.push(input(
                    &format!("k{}{}", p, u),
                    &format!("problem{}", p),
                    &format!("user{}", u),
                    A,
                ));
            }
        }
        let first: Vec<String> = scan(&inputs, 0.7)
            .hits
            .iter()
            .map(|h| h.a.clone())
            .collect();
        for _ in 0..5 {
            let again: Vec<String> = scan(&inputs, 0.7)
                .hits
                .iter()
                .map(|h| h.a.clone())
                .collect();
            assert_eq!(first, again);
        }
    }
}
