package plagiarism

import (
	"context"
	"math"
	"runtime"
	"sort"
	"sync"
)

// NativeEngine is the pure-Go similarity engine.
//
// It replaces the original per-pair `CompareCodes` loop with three changes that
// do not alter a single reported number:
//
//  1. **Tokenize once.** The old loop re-ran five regex passes over both
//     submissions for every pair, and `MatchedLines` re-ran them again. Tokens
//     and token ids are computed once per submission here.
//  2. **Prefilter with a sound bound.** Any common subsequence is a common
//     multiset, so `LCS <= multiset intersection`; a pair whose intersection
//     bound is below the threshold cannot cross it and is skipped.
//  3. **Flat, early-exiting LCS.** The old implementation allocated a full
//     `[][]int` matrix (≈6.7 MB per comparison at 900 tokens). Two rolling rows
//     in a reused buffer replace it, and the search stops as soon as the best
//     achievable score is below the threshold.
//
// Those three are worth roughly two orders of magnitude on their own — which is
// why the Go path is a real fallback rather than a token gesture. RustEngine
// adds the bit-parallel LCS and wider parallelism on top.
type NativeEngine struct{}

// Name identifies the engine in logs.
func (NativeEngine) Name() string { return "native" }

type preparedSource struct {
	ids []int32
	// counts is (token id, occurrences) sorted by id, for the intersection
	// bound.
	counts []tokenCount
}

type tokenCount struct {
	id    int32
	count int32
}

// FindSimilar implements Engine.
func (e NativeEngine) FindSimilar(ctx context.Context, sources []Source, threshold float64) (*ScanResult, error) {
	prepared := make([]preparedSource, len(sources))
	interner := make(map[string]int32, 1024)
	for i, src := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tokens := Tokenize(src.Code)
		ids := make([]int32, len(tokens))
		for j, tok := range tokens {
			id, ok := interner[tok]
			if !ok {
				id = int32(len(interner))
				interner[tok] = id
			}
			ids[j] = id
		}
		prepared[i] = preparedSource{ids: ids, counts: sortedCounts(ids)}
	}

	candidates := enumerateCandidates(sources)
	result := &ScanResult{PairsTotal: len(candidates), Pairs: []Pair{}}
	if len(candidates) == 0 {
		return result, nil
	}

	// Score in parallel. Each worker owns its LCS scratch buffer and its own
	// result slice, so the hot loop never takes a lock.
	workers := runtime.GOMAXPROCS(0)
	if workers > len(candidates) {
		workers = len(candidates)
	}
	chunk := (len(candidates) + workers - 1) / workers

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		scored int
		hits   []Pair
	)
	for start := 0; start < len(candidates); start += chunk {
		end := start + chunk
		if end > len(candidates) {
			end = len(candidates)
		}
		wg.Add(1)
		go func(batch [][2]int) {
			defer wg.Done()
			scratch := &lcsScratch{}
			local := make([]Pair, 0, 8)
			localScored := 0
			for _, pair := range batch {
				if ctx.Err() != nil {
					return
				}
				hit, ranLCS := scorePair(prepared, sources, pair[0], pair[1], threshold, scratch)
				if ranLCS {
					localScored++
				}
				if hit != nil {
					local = append(local, *hit)
				}
			}
			mu.Lock()
			scored += localScored
			hits = append(hits, local...)
			mu.Unlock()
		}(candidates[start:end])
	}
	wg.Wait()

	// A cancelled scan must not be reported as "no plagiarism found": the
	// caller has to mark the report failed and let somebody retry it.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sortPairs(hits)
	result.Pairs = hits
	result.PairsScored = scored
	return result, nil
}

// enumerateCandidates lists every pair with the same problem and different
// authors, matching the rule the original loop applied inline.
func enumerateCandidates(sources []Source) [][2]int {
	byProblem := make(map[string][]int, len(sources))
	order := make([]string, 0, len(sources))
	for i, s := range sources {
		if _, seen := byProblem[s.ProblemID]; !seen {
			order = append(order, s.ProblemID)
		}
		byProblem[s.ProblemID] = append(byProblem[s.ProblemID], i)
	}

	var out [][2]int
	for _, pid := range order {
		members := byProblem[pid]
		for i := 0; i < len(members); i++ {
			for j := i + 1; j < len(members); j++ {
				if sources[members[i]].UserID == sources[members[j]].UserID {
					continue
				}
				out = append(out, [2]int{members[i], members[j]})
			}
		}
	}
	return out
}

func sortedCounts(ids []int32) []tokenCount {
	if len(ids) == 0 {
		return nil
	}
	sorted := make([]int32, len(ids))
	copy(sorted, ids)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	out := make([]tokenCount, 0, len(sorted))
	for _, id := range sorted {
		if n := len(out); n > 0 && out[n-1].id == id {
			out[n-1].count++
			continue
		}
		out = append(out, tokenCount{id: id, count: 1})
	}
	return out
}

// intersectionSize is the size of the token-multiset intersection: an upper
// bound on the LCS and therefore a sound prefilter.
func intersectionSize(a, b []tokenCount) int {
	i, j, shared := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i].id == b[j].id:
			shared += int(min(a[i].count, b[j].count))
			i++
			j++
		case a[i].id < b[j].id:
			i++
		default:
			j++
		}
	}
	return shared
}

// scorePair scores one candidate. It returns the hit (or nil) and whether a
// dynamic program actually ran, so the cost signal reported to operators stays
// honest.
func scorePair(
	prepared []preparedSource,
	sources []Source,
	idxA, idxB int,
	threshold float64,
	scratch *lcsScratch,
) (*Pair, bool) {
	a, b := &prepared[idxA], &prepared[idxB]
	lengthA, lengthB := len(a.ids), len(b.ids)

	// LCSSimilarity is 0 when either side has no tokens.
	if lengthA == 0 || lengthB == 0 {
		if 0.0 >= threshold {
			return newPair(sources, idxA, idxB, 0, 0), false
		}
		return nil, false
	}

	// Sound prefilter: LCS <= multiset intersection, so this bound can only
	// over-estimate and can never discard a pair that would have qualified.
	shared := intersectionSize(a.counts, b.counts)
	if similarityOf(shared, lengthA, lengthB) < threshold {
		return nil, false
	}

	lcs := scratch.length(a.ids, b.ids, requiredLCS(threshold, lengthA, lengthB))
	if lcs < 0 {
		// Abandoned early: provably below the threshold.
		return nil, true
	}
	similarity := similarityOf(lcs, lengthA, lengthB)
	if similarity < threshold {
		return nil, true
	}
	return newPair(sources, idxA, idxB, lcs, similarity), true
}

func newPair(sources []Source, idxA, idxB, matched int, similarity float64) *Pair {
	return &Pair{
		ProblemID:     sources[idxA].ProblemID,
		A:             sources[idxA].Key,
		B:             sources[idxB].Key,
		Similarity:    similarity,
		MatchedTokens: matched,
	}
}

// requiredLCS is the smallest LCS length that can still reach the threshold.
// Rounded up, so `lcs < requiredLCS` is guaranteed to mean "below threshold"
// without relying on the float comparison.
func requiredLCS(threshold float64, lenA, lenB int) int {
	if threshold <= 0 {
		return 0
	}
	return int(math.Ceil(threshold * float64(lenA+lenB) / 2.0))
}

// lcsScratch computes LCS length with two rolling rows, reusing its buffers
// across calls so a scan does no per-pair allocation.
type lcsScratch struct {
	prev []int32
	cur  []int32
}

// length returns the LCS length of a and b, or -1 when the result is provably
// below need and the row was abandoned early.
func (s *lcsScratch) length(a, b []int32, need int) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	// Iterate the longer sequence as rows so the inner loop runs over the
	// shorter one; the buffers stay small.
	rows, cols := a, b
	if len(a) < len(b) {
		rows, cols = b, a
	}

	n := len(cols)
	if cap(s.prev) < n+1 {
		s.prev = make([]int32, n+1)
		s.cur = make([]int32, n+1)
	}
	prev := s.prev[:n+1]
	cur := s.cur[:n+1]
	for i := range prev {
		prev[i] = 0
	}

	total := len(rows)
	for i := 1; i <= total; i++ {
		row := rows[i-1]
		cur[0] = 0
		rowMax := int32(0)
		for j := 1; j <= n; j++ {
			if row == cols[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
			if cur[j] > rowMax {
				rowMax = cur[j]
			}
		}

		// Any common subsequence splits at row i, so
		//   LCS(rows, cols) <= dp[i][n] + min(total-i, n)
		// and the scan can stop the moment that ceiling cannot reach `need`.
		// dp[i][n] is the row maximum because the DP is non-decreasing in j.
		remaining := total - i
		if n < remaining {
			remaining = n
		}
		if need > 0 && int(rowMax)+remaining < need {
			return -1
		}
		prev, cur = cur, prev
	}
	return int(prev[n])
}
