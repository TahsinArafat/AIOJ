//! Sparse-token LCS and the sound prefilter that keeps it off almost every pair.
//!
//! Two properties matter here and both are tested rather than assumed:
//!
//!  1. **Exactness.** `lcs_len` returns the same integer as a textbook
//!     O(n·m) dynamic program. The bit-parallel formulation is a different
//!     algorithm, not an approximation, so similarity scores are unchanged by
//!     the port — the report a contest admin reads is the report they would have
//!     read from the Go engine.
//!  2. **Soundness of the prefilter.** `similarity_bound` must never discard a
//!     pair that would have crossed the threshold. If it did, the engine would
//!     silently stop reporting plagiarists instead of failing loudly.

/// Exact LCS length of two token-id sequences, via the bit-parallel formulation
/// of Crochemore, Iliopoulos, Pinzon & Reid (2001):
///
/// ```text
/// V' = (V + U) | (V - U)     where U = V & Peq[c]
/// ```
///
/// `Peq[c]` has bit *i* set when `a[i] == c`. After processing all of `b`, the
/// popcount of `V` counts matched positions, so `|a| - popcount(V)` is the LCS
/// length. Cost is `|b| · ⌈|a|/64⌉` word operations — 64× fewer than the
/// dynamic program, with no per-pair matrix allocation.
///
/// The roles are not symmetric in cost, but they are symmetric in result, so the
/// caller passes the shorter sequence as `a` to keep the mask table small.
pub fn lcs_len(a: &[u32], b: &[u32], table: &mut MaskTable) -> u32 {
    if a.is_empty() || b.is_empty() {
        return 0;
    }

    let words = a.len().div_ceil(64);
    // Bits above |a| must never enter the carry chain: they would inflate the
    // popcount and make the result wrong. Track the mask of the last word.
    let tail_bits = a.len() % 64;
    let tail_mask = if tail_bits == 0 {
        u64::MAX
    } else {
        (1u64 << tail_bits) - 1
    };

    table.begin(a, words);

    let mut v = vec![u64::MAX; words];
    v[words - 1] = tail_mask;

    let mut u = vec![0u64; words];
    let mut vp = vec![0u64; words];
    let mut vm = vec![0u64; words];

    for &c in b {
        let row = table.row(c);
        match row {
            // A symbol absent from the pattern contributes nothing.
            None => continue,
            Some(row) => {
                for w in 0..words {
                    u[w] = v[w] & row[w];
                }
            }
        }

        // v + u, propagating carry across words.
        let mut carry = 0u64;
        for w in 0..words {
            let (s1, o1) = v[w].overflowing_add(u[w]);
            let (s2, o2) = s1.overflowing_add(carry);
            vp[w] = s2;
            carry = (o1 as u64) | (o2 as u64);
        }

        // v - u, propagating borrow across words.
        let mut borrow = 0u64;
        for w in 0..words {
            let (d1, b1) = v[w].overflowing_sub(u[w]);
            let (d2, b2) = d1.overflowing_sub(borrow);
            vm[w] = d2;
            borrow = (b1 as u64) | (b2 as u64);
        }

        for w in 0..words {
            v[w] = (vp[w] | vm[w]) & if w + 1 == words { tail_mask } else { u64::MAX };
        }
    }

    let ones: u32 = v.iter().map(|w| w.count_ones()).sum();
    a.len() as u32 - ones
}

/// A reusable `Peq` table for `lcs_len`.
///
/// Rows are addressed as `symbol * stride + word` inside one flat buffer, so a
/// scan over thousands of pairs performs no allocation per pair. A generation
/// stamp per symbol avoids clearing rows that the current pattern never touches;
/// a row is zero-filled only the first time its symbol appears in a pattern.
pub struct MaskTable {
    data: Vec<u64>,
    stamp: Vec<u32>,
    generation: u32,
    stride: usize,
}

impl MaskTable {
    /// `vocab` is the number of distinct symbols the scan can produce, and
    /// `max_len` the longest sequence, which fixes the row stride.
    pub fn new(vocab: usize, max_len: usize) -> Self {
        let stride = max_len.div_ceil(64).max(1);
        Self {
            data: vec![0u64; vocab * stride],
            stamp: vec![0u32; vocab],
            generation: 0,
            stride,
        }
    }

    fn begin(&mut self, pattern: &[u32], words: usize) {
        // `words` may be smaller than `stride` when this pattern is shorter than
        // the longest sequence the table was sized for; only the first `words`
        // entries of a row are ever read, and only those are cleared.
        debug_assert!(words <= self.stride);
        self.generation = self.generation.wrapping_add(1);
        if self.generation == 0 {
            // Wrapped: the stamps are stale, so clear them and start over.
            self.stamp.iter_mut().for_each(|s| *s = 0);
            self.generation = 1;
        }
        for (i, &sym) in pattern.iter().enumerate() {
            let s = sym as usize;
            if self.stamp[s] != self.generation {
                self.stamp[s] = self.generation;
                let base = s * self.stride;
                self.data[base..base + words]
                    .iter_mut()
                    .for_each(|w| *w = 0);
            }
            let base = s * self.stride;
            self.data[base + i / 64] |= 1u64 << (i % 64);
        }
    }

    fn row(&self, sym: u32) -> Option<&[u64]> {
        let s = sym as usize;
        if self.stamp[s] == self.generation {
            let base = s * self.stride;
            Some(&self.data[base..])
        } else {
            None
        }
    }
}

/// Upper bound on `2·LCS(a,b) / (|a|+|b|)` that costs a sorted merge instead of a
/// dynamic program.
///
/// Proof of soundness: any common subsequence is also a common *multiset*, so
/// `LCS(a,b) ≤ Σ_c min(count_a[c], count_b[c])`. Substituting that into the
/// similarity formula can only over-estimate, therefore a pair whose bound is
/// below the threshold cannot cross it and is safe to skip.
///
/// Inputs are `(symbol, count)` pairs sorted by `symbol`.
pub fn similarity_bound(a: &[(u32, u32)], b: &[(u32, u32)], shared_tokens: usize) -> f64 {
    let total = a.iter().map(|&(_, c)| c as usize).sum::<usize>()
        + b.iter().map(|&(_, c)| c as usize).sum::<usize>();
    if total == 0 {
        return 0.0;
    }
    2.0 * shared_tokens as f64 / total as f64
}

/// Size of the token multiset intersection implied by two sorted `(id, count)`
/// vectors.
pub fn intersection_size(a: &[(u32, u32)], b: &[(u32, u32)]) -> usize {
    let mut i = 0;
    let mut j = 0;
    let mut shared = 0usize;
    while i < a.len() && j < b.len() {
        match a[i].0.cmp(&b[j].0) {
            std::cmp::Ordering::Equal => {
                shared += a[i].1.min(b[j].1) as usize;
                i += 1;
                j += 1;
            }
            std::cmp::Ordering::Less => i += 1,
            std::cmp::Ordering::Greater => j += 1,
        }
    }
    shared
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Textbook O(n·m) LCS, used only to check the bit-parallel result.
    fn lcs_dp(a: &[u32], b: &[u32]) -> u32 {
        let mut prev = vec![0u32; b.len() + 1];
        let mut cur = vec![0u32; b.len() + 1];
        for &x in a {
            for j in 1..=b.len() {
                cur[j] = if x == b[j - 1] {
                    prev[j - 1] + 1
                } else {
                    prev[j].max(cur[j - 1])
                };
            }
            std::mem::swap(&mut prev, &mut cur);
        }
        prev[b.len()]
    }

    fn bitset(a: &[u32], b: &[u32]) -> u32 {
        let mut t = MaskTable::new(64, a.len().max(b.len()).max(1));
        lcs_len(a, b, &mut t)
    }

    /// xorshift so the test corpus is reproducible on every machine.
    struct Rng(u64);
    impl Rng {
        fn next(&mut self) -> u64 {
            self.0 ^= self.0 << 13;
            self.0 ^= self.0 >> 7;
            self.0 ^= self.0 << 17;
            self.0
        }
        fn below(&mut self, n: u64) -> u64 {
            self.next() % n
        }
    }

    #[test]
    fn matches_dynamic_program_on_random_inputs() {
        let mut rng = Rng(0x5EED_1234);
        for _ in 0..2000 {
            let la = rng.below(70) as usize + 1;
            let lb = rng.below(70) as usize + 1;
            let alpha = rng.below(6) + 1;
            let a: Vec<u32> = (0..la).map(|_| rng.below(alpha) as u32).collect();
            let b: Vec<u32> = (0..lb).map(|_| rng.below(alpha) as u32).collect();
            assert_eq!(bitset(&a, &b), lcs_dp(&a, &b), "a={:?} b={:?}", a, b);
        }
    }

    #[test]
    fn matches_dynamic_program_across_word_boundaries() {
        // 63/64/65/127/128 exercise the last-word mask, which is where a
        // bit-parallel implementation leaks bits and over-counts.
        let mut rng = Rng(0xABCD_0001);
        for &n in &[63usize, 64, 65, 127, 128, 129, 191, 192] {
            for _ in 0..20 {
                let a: Vec<u32> = (0..n).map(|_| rng.below(4) as u32).collect();
                let b: Vec<u32> = (0..n + 7).map(|_| rng.below(4) as u32).collect();
                assert_eq!(bitset(&a, &b), lcs_dp(&a, &b), "len={}", n);
            }
        }
    }

    #[test]
    fn identical_sequences_score_one() {
        let a: Vec<u32> = (0..500).map(|i| (i % 37) as u32).collect();
        let mut t = MaskTable::new(37, 500);
        assert_eq!(lcs_len(&a, &a, &mut t), 500);
    }

    #[test]
    fn disjoint_sequences_score_zero() {
        let a: Vec<u32> = (0..200).map(|i| i as u32).collect();
        let b: Vec<u32> = (200..400).map(|i| i as u32).collect();
        let mut t = MaskTable::new(400, 200);
        assert_eq!(lcs_len(&a, &b, &mut t), 0);
    }

    #[test]
    fn empty_inputs_score_zero() {
        let mut t = MaskTable::new(4, 1);
        assert_eq!(lcs_len(&[], &[1, 2, 3], &mut t), 0);
        assert_eq!(lcs_len(&[1, 2, 3], &[], &mut t), 0);
    }

    #[test]
    fn table_is_reusable_across_patterns() {
        // The generation stamp is the risky part: a second, unrelated pattern
        // must not see the first pattern's masks.
        let mut t = MaskTable::new(16, 40);
        let a: Vec<u32> = (0..40).map(|i| (i % 5) as u32).collect();
        let b: Vec<u32> = (0..40).map(|i| ((i + 2) % 5) as u32).collect();
        let c: Vec<u32> = (0..40).map(|i| (10 + i % 5) as u32).collect();
        for _ in 0..5 {
            assert_eq!(lcs_len(&a, &b, &mut t), lcs_dp(&a, &b));
            assert_eq!(lcs_len(&c, &b, &mut t), lcs_dp(&c, &b));
            assert_eq!(lcs_len(&c, &c, &mut t), 40);
        }
    }

    #[test]
    fn intersection_counts_multiset_overlap() {
        // a: 10×1, 1×2, 2×3   b: 1×1, 5×2, 2×3
        let a = vec![(1u32, 10u32), (2, 1), (3, 2)];
        let b = vec![(1, 1), (2, 5), (3, 2)];
        assert_eq!(intersection_size(&a, &b), 1 + 1 + 2);
        assert_eq!(intersection_size(&[], &b), 0);
        assert_eq!(intersection_size(&a, &[]), 0);
    }

    #[test]
    fn bound_is_never_below_the_true_similarity() {
        let mut rng = Rng(0xF00D_9999);
        for _ in 0..3000 {
            let la = rng.below(40) as usize + 1;
            let lb = rng.below(40) as usize + 1;
            let alpha = rng.below(5) + 1;
            let a: Vec<u32> = (0..la).map(|_| rng.below(alpha) as u32).collect();
            let b: Vec<u32> = (0..lb).map(|_| rng.below(alpha) as u32).collect();

            let sa = sorted_counts(&a);
            let sb = sorted_counts(&b);
            let shared = intersection_size(&sa, &sb);
            let bound = similarity_bound(&sa, &sb, shared);

            let true_sim = 2.0 * lcs_dp(&a, &b) as f64 / (la + lb) as f64;
            assert!(
                bound + 1e-12 >= true_sim,
                "bound {} < true {} for a={:?} b={:?}",
                bound,
                true_sim,
                a,
                b
            );
        }
    }

    fn sorted_counts(t: &[u32]) -> Vec<(u32, u32)> {
        let mut v = t.to_vec();
        v.sort_unstable();
        let mut out: Vec<(u32, u32)> = Vec::new();
        for x in v {
            match out.last_mut() {
                Some(last) if last.0 == x => last.1 += 1,
                _ => out.push((x, 1)),
            }
        }
        out
    }
}
