package plagiarism

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// The tests in this file exist to make one claim checkable: every engine reports
// exactly what the original per-pair `CompareCodes` loop would have reported.
// Fast paths, prefilters and a second implementation in another language are all
// only acceptable if that holds, so it is asserted against the reference on a
// corpus rather than argued for in a comment.

// corpus builds submissions with the awkward properties that break similarity
// engines: near-threshold copies, shared prologues with divergent bodies,
// comments and string literals, and non-ASCII text.
func corpus() []Source {
	bases := []string{
		`#include <bits/stdc++.h>
using namespace std;
int main(){
  ios::sync_with_stdio(false);
  int n; cin >> n;
  vector<long long> a(n);
  for (int i = 0; i < n; i++) cin >> a[i];
  long long sum = 0;
  for (int i = 0; i < n; i++) sum += a[i] * (i + 1);
  cout << sum << "\n";
  return 0;
}`,
		`def solve():
    n = int(input())
    a = list(map(int, input().split()))
    total = 0
    for i, x in enumerate(a):
        total += x * (i + 1)
    print(total)

solve()`,
		`import java.util.*;
public class Main {
    public static void main(String[] args) {
        Scanner sc = new Scanner(System.in);
        int n = sc.nextInt();
        long sum = 0;
        for (int i = 0; i < n; i++) sum += sc.nextLong() * (i + 1);
        System.out.println(sum);
    }
}`,
	}

	// Identifiers that get rewritten to create "renamed but structurally
	// identical" copies — the realistic plagiarism case.
	renames := [][2]string{
		{"sum", "acc"}, {"total", "acc"}, {"n", "count"}, {"i", "idx"},
		{"a", "arr"}, {"x", "value"}, {"solve", "compute"}, {"Main", "Solution"},
	}

	var out []Source
	next := 0
	add := func(problemID, userID, code string) {
		next++
		out = append(out, Source{
			Key:       fmt.Sprintf("sub-%02d", next),
			ProblemID: problemID,
			UserID:    userID,
			Code:      code,
		})
	}

	for p, base := range bases {
		problemID := fmt.Sprintf("problem-%d", p)
		// One author per submission, and one author with several submissions so
		// the same-author rule is exercised.
		add(problemID, "alice", base)
		add(problemID, "bob", base)                                   // identical copy: similarity 1.0
		add(problemID, "carol", strings.ReplaceAll(base, "i", "idx")) // heavy rename
		add(problemID, "dave", commentVariants(base))
		add(problemID, "erin", whitespaceVariants(base))
		for r := 0; r < 3; r++ {
			add(problemID, fmt.Sprintf("user-%d-%d", p, r), applyRenames(base, renames[r:r+3]))
		}
		// A submission from the same author as the first one: must never be a pair.
		add(problemID, "alice", applyRenames(base, renames))
		// Something genuinely unrelated.
		add(problemID, "frank", "int main(){ return 0; }")
	}

	// Non-ASCII source, which the tokenizer must keep byte-identical across
	// engines.
	add("problem-0", "grace", "// komentar: ünïcode ✓\nint café = 0; void solve(){ café++; }")

	return out
}

func commentVariants(code string) string {
	return strings.ReplaceAll(code, "\n", "\n// a comment here\n/* and a block\n comment */\n")
}

func whitespaceVariants(code string) string {
	return strings.ReplaceAll(code, " ", "   \n\t")
}

func applyRenames(code string, renames [][2]string) string {
	for _, r := range renames {
		code = strings.ReplaceAll(code, r[0], r[1])
	}
	return code
}

// referenceAgreement runs the old pair-by-pair reference over the corpus and
// returns the similarity it reports for every eligible pair.
func referenceAgreement(sources []Source) map[[2]string]float64 {
	out := make(map[[2]string]float64)
	for i := range sources {
		for j := i + 1; j < len(sources); j++ {
			if sources[i].ProblemID != sources[j].ProblemID || sources[i].UserID == sources[j].UserID {
				continue
			}
			out[[2]string{sources[i].Key, sources[j].Key}] = CompareCodes(sources[i].Code, sources[j].Code)
		}
	}
	return out
}

// assertSameAsReference checks an engine's hits against the reference at several
// thresholds, including ones that sit on top of the corpus's actual similarities
// so the prefilter boundary is probed rather than avoided.
func assertSameAsReference(t *testing.T, engine Engine, sources []Source) {
	t.Helper()
	reference := referenceAgreement(sources)

	thresholds := []float64{0.05, 0.2, 0.4, 0.5, 0.6, 0.64, 0.7, 0.8, 0.95, 1.0}
	for _, threshold := range thresholds {
		got, err := engine.FindSimilar(context.Background(), sources, threshold)
		if err != nil {
			t.Fatalf("%s: FindSimilar: %v", engine.Name(), err)
		}

		want := make(map[[2]string]float64)
		for key, sim := range reference {
			if sim >= threshold {
				want[key] = sim
			}
		}

		seen := make(map[[2]string]bool, len(got.Pairs))
		for _, p := range got.Pairs {
			key := [2]string{p.A, p.B}
			expected, ok := want[key]
			if !ok {
				t.Errorf("%s @%.2f: reported %s/%s (%.6f) which the reference does not flag",
					engine.Name(), threshold, p.A, p.B, p.Similarity)
				continue
			}
			seen[key] = true
			if p.Similarity != expected {
				t.Errorf("%s @%.2f: similarity for %s/%s is %.17g, reference says %.17g",
					engine.Name(), threshold, p.A, p.B, p.Similarity, expected)
			}
			if p.ProblemID != sources[indexOf(t, sources, p.A)].ProblemID {
				t.Errorf("%s: pair %s/%s carries problem %q", engine.Name(), p.A, p.B, p.ProblemID)
			}
		}
		for key, sim := range want {
			if !seen[key] {
				t.Errorf("%s @%.2f: missed pair %s/%s (reference %.6f)",
					engine.Name(), threshold, key[0], key[1], sim)
			}
		}

		// The denominator must match too: a report that says "0 pairs compared"
		// is as misleading as a wrong hit list.
		if wantTotal := CountEligiblePairs(sources); got.PairsTotal != wantTotal {
			t.Errorf("%s @%.2f: PairsTotal = %d, want %d", engine.Name(), threshold, got.PairsTotal, wantTotal)
		}
		if got.PairsScored > got.PairsTotal {
			t.Errorf("%s: PairsScored %d exceeds PairsTotal %d", engine.Name(), got.PairsScored, got.PairsTotal)
		}
	}
}

func indexOf(t *testing.T, sources []Source, key string) int {
	t.Helper()
	for i, s := range sources {
		if s.Key == key {
			return i
		}
	}
	t.Fatalf("unknown source key %q", key)
	return -1
}

func TestNativeEngineMatchesCompareCodes(t *testing.T) {
	assertSameAsReference(t, NativeEngine{}, corpus())
}

func TestNativeEngineMatchedTokensIsLCSSpan(t *testing.T) {
	sources := corpus()
	got, err := NativeEngine{}.FindSimilar(context.Background(), sources, 0.5)
	if err != nil {
		t.Fatalf("FindSimilar: %v", err)
	}
	if len(got.Pairs) == 0 {
		t.Fatal("expected hits from the corpus")
	}
	byKey := make(map[string]Source, len(sources))
	for _, s := range sources {
		byKey[s.Key] = s
	}
	for _, p := range got.Pairs {
		want := LCSLength(Tokenize(byKey[p.A].Code), Tokenize(byKey[p.B].Code))
		if p.MatchedTokens != want {
			t.Errorf("matched tokens for %s/%s = %d, want %d", p.A, p.B, p.MatchedTokens, want)
		}
	}
}

func TestCountEligiblePairs(t *testing.T) {
	sources := []Source{
		{Key: "a", ProblemID: "p1", UserID: "u1"},
		{Key: "b", ProblemID: "p1", UserID: "u2"},
		{Key: "c", ProblemID: "p1", UserID: "u2"}, // same author as b
		{Key: "d", ProblemID: "p2", UserID: "u1"},
		{Key: "e", ProblemID: "p2", UserID: "u3"},
	}
	// p1: a-b, a-c (b-c is same author) = 2; p2: d-e = 1.
	if got := CountEligiblePairs(sources); got != 3 {
		t.Errorf("CountEligiblePairs = %d, want 3", got)
	}
	if got := CountEligiblePairs(nil); got != 0 {
		t.Errorf("CountEligiblePairs(nil) = %d, want 0", got)
	}
}

func TestServiceFallsBackWhenEngineFails(t *testing.T) {
	engine := failingEngine{}
	svc := NewServiceWithEngine(nil, nil, nil, engine)
	if svc.fallback == nil {
		t.Fatal("a non-native engine must install the native fallback")
	}
	result, err := svc.scan(context.Background(), corpus(), 0.9)
	if err != nil {
		t.Fatalf("scan should have fallen back, got error: %v", err)
	}
	if len(result.Pairs) == 0 {
		t.Fatal("fallback produced no hits")
	}
}

func TestServiceDoesNotFallBackOnCancellation(t *testing.T) {
	svc := NewServiceWithEngine(nil, nil, nil, failingEngine{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.scan(ctx, corpus(), 0.9); err == nil {
		t.Fatal("a cancelled scan must return the context error, not silently retry")
	}
}

type failingEngine struct{}

func (failingEngine) Name() string { return "failing" }

func (failingEngine) FindSimilar(context.Context, []Source, float64) (*ScanResult, error) {
	return nil, fmt.Errorf("engine exploded")
}

// ---- Rust engine ---------------------------------------------------------

// rustBinary locates the compiled engine, skipping the test when it is absent.
// `make test-parity` builds it first; a plain `go test` stays green without a
// Rust toolchain.
func rustBinary(t *testing.T) string {
	t.Helper()
	bin, err := locateRustBinary()
	if err != nil {
		t.Skipf("aioj-plagiarism not built: %v (run `make rust-build`)", err)
	}
	return bin
}

func TestRustEngineMatchesReference(t *testing.T) {
	bin := rustBinary(t)
	assertSameAsReference(t, NewRustEngine(bin), corpus())
}

func TestNativeAndRustEnginesAgree(t *testing.T) {
	bin := rustBinary(t)
	sources := corpus()
	native, err := NativeEngine{}.FindSimilar(context.Background(), sources, 0.4)
	if err != nil {
		t.Fatalf("native: %v", err)
	}
	rust, err := NewRustEngine(bin).FindSimilar(context.Background(), sources, 0.4)
	if err != nil {
		t.Fatalf("rust: %v", err)
	}
	if len(native.Pairs) != len(rust.Pairs) {
		t.Fatalf("pair count differs: native %d, rust %d", len(native.Pairs), len(rust.Pairs))
	}
	for i := range native.Pairs {
		if native.Pairs[i] != rust.Pairs[i] {
			t.Errorf("pair %d differs:\n native %+v\n rust   %+v", i, native.Pairs[i], rust.Pairs[i])
		}
	}
	if native.PairsTotal != rust.PairsTotal {
		t.Errorf("PairsTotal differs: native %d, rust %d", native.PairsTotal, rust.PairsTotal)
	}
}

// TestRustTokenizerMatchesGo is the load-bearing test for the port. The Rust
// tokenizer reimplements a five-pass regex pipeline by hand; if it drifts, every
// similarity score shifts with it and reports become quietly wrong. Comparing
// token streams directly names the failing input instead of surfacing a
// mysterious 0.03 score difference.
func TestRustTokenizerMatchesGo(t *testing.T) {
	bin := rustBinary(t)

	cases := append([]string{}, []string{
		"",
		"   \n\t ",
		"// only a comment",
		"int x = 5; // trailing comment",
		`printf("hello world");`,
		"int /* block */ x = 5;",
		"int x /* unterminated",
		`a = "http://example.com";`,
		"123abc 0x1F a1",
		"12.34.56 v1.5",
		`"a\"b" '\''"`,
		"\"line\nbreak\"",
		"a\x0bb",
		"a\u00a0b c\u3000d",
		"int café = 1; // ünïcode ✓",
		"a / * b",
		"a /*/ b",
		"a /* b /* c */ d",
		"for (int i = 0; i < n; i++) sum += a[i] * (i + 1);",
		"#include <bits/stdc++.h>\nusing namespace std;\n",
		"x = 'unterminated",
		"/*/",
		"*/",
		"\r\n\tint\ty\r\n",
	}...)
	for _, s := range corpus() {
		cases = append(cases, s.Code)
	}

	for i, code := range cases {
		cmd := exec.Command(bin, "--tokens")
		cmd.Stdin = strings.NewReader(code)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("case %d: run --tokens: %v", i, err)
		}
		got := strings.TrimRight(string(out), "\n")
		want := strings.Join(Tokenize(code), " ")
		if got != want {
			t.Errorf("case %d: token drift\n input: %q\n  go: %q\nrust: %q", i, code, want, got)
		}
	}
}

func TestRustEngineRejectsInvalidRequest(t *testing.T) {
	bin := rustBinary(t)
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("{not json")
	out, err := cmd.Output()
	if err == nil {
		t.Fatal("expected a non-zero exit for an invalid request")
	}
	if !strings.Contains(string(out), `"error"`) {
		t.Errorf("expected a JSON error line, got %q", string(out))
	}
}

func TestSelectEngineHonoursEnvironment(t *testing.T) {
	t.Run("native", func(t *testing.T) {
		t.Setenv(EngineEnv, "native")
		engine, err := SelectEngine()
		if err != nil {
			t.Fatalf("SelectEngine: %v", err)
		}
		if engine.Name() != "native" {
			t.Errorf("engine = %q, want native", engine.Name())
		}
	})

	t.Run("explicit rust without a binary fails loudly", func(t *testing.T) {
		t.Setenv(EngineEnv, "rust")
		t.Setenv(BinaryEnv, "/nonexistent/aioj-plagiarism")
		if _, err := SelectEngine(); err == nil {
			t.Fatal("asking for rust explicitly must not silently downgrade")
		}
	})

	t.Run("auto falls back when the binary is missing", func(t *testing.T) {
		t.Setenv(EngineEnv, "auto")
		t.Setenv(BinaryEnv, "/nonexistent/aioj-plagiarism")
		engine, err := SelectEngine()
		if err != nil {
			t.Fatalf("auto must not fail on a missing binary: %v", err)
		}
		if engine.Name() != "native" {
			t.Errorf("engine = %q, want native", engine.Name())
		}
	})

	t.Run("unknown mode is an error", func(t *testing.T) {
		t.Setenv(EngineEnv, "cobol")
		if _, err := SelectEngine(); err == nil {
			t.Fatal("an unknown mode must be rejected")
		}
	})
}

// ---- benchmarks ----------------------------------------------------------

// benchCorpus mirrors the cap the service applies: 300 accepted submissions on a
// single problem, i.e. 44,850 pairs.
func benchCorpus(n int) []Source {
	base := corpus()[0].Code
	out := make([]Source, n)
	for i := 0; i < n; i++ {
		code := base
		if i%3 != 0 {
			// Two thirds are renamed-but-equal in structure: the realistic case,
			// and the one a sound prefilter cannot reject.
			code = applyRenames(base, [][2]string{{"sum", fmt.Sprintf("sum%d", i)}, {"n", fmt.Sprintf("n%d", i)}})
		}
		out[i] = Source{
			Key:       fmt.Sprintf("bench-%03d", i),
			ProblemID: "bench-problem",
			UserID:    fmt.Sprintf("user-%03d", i),
			Code:      code,
		}
	}
	return out
}

// BenchmarkReferencePairLoop is the original implementation's shape: tokenize
// both sides and allocate a full matrix, for every pair.
func BenchmarkReferencePairLoop(b *testing.B) {
	sources := benchCorpus(300)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for x := 0; x < len(sources); x++ {
			for y := x + 1; y < len(sources); y++ {
				if sources[x].UserID == sources[y].UserID {
					continue
				}
				CompareCodes(sources[x].Code, sources[y].Code)
			}
		}
	}
}

func BenchmarkNativeEngineScan(b *testing.B) {
	sources := benchCorpus(300)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := (NativeEngine{}).FindSimilar(context.Background(), sources, 0.7); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRustEngineScan(b *testing.B) {
	bin, err := locateRustBinary()
	if err != nil {
		b.Skipf("aioj-plagiarism not built: %v", err)
	}
	sources := benchCorpus(300)
	engine := NewRustEngine(bin)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.FindSimilar(context.Background(), sources, 0.7); err != nil {
			b.Fatal(err)
		}
	}
}

// benchRealisticCorpus models a real submission: ~900 tokens, the size measured
// on a typical competitive-programming solution. LCS cost scales with tokens, so
// this is the corpus where a bit-parallel LCS can actually pay for itself, and
// the one that separates the engines on compute rather than process startup.
func benchRealisticCorpus(n int) []Source {
	var b strings.Builder
	b.WriteString("#include <bits/stdc++.h>\nusing namespace std;\n")
	b.WriteString("int main(){\n  ios::sync_with_stdio(false);\n  int n; cin >> n;\n")
	b.WriteString("  vector<long long> a(n);\n  for (int i = 0; i < n; i++) cin >> a[i];\n")
	for r := 0; r < 40; r++ {
		fmt.Fprintf(&b, "  long long s%d = 0; for (int i = 0; i < n; i++) { if (a[i] > %d) s%d += a[i] * %d; }\n", r, r, r, r+1)
	}
	b.WriteString("  cout << 0 << \"\\n\";\n  return 0;\n}\n")
	base := b.String()

	out := make([]Source, n)
	for i := 0; i < n; i++ {
		code := base
		if i%3 != 0 {
			code = strings.ReplaceAll(base, "long long s", fmt.Sprintf("long long acc%d_", i))
		}
		out[i] = Source{
			Key:       fmt.Sprintf("real-%03d", i),
			ProblemID: "real-problem",
			UserID:    fmt.Sprintf("user-%03d", i),
			Code:      code,
		}
	}
	return out
}

func BenchmarkReferencePairLoopRealistic(b *testing.B) {
	sources := benchRealisticCorpus(300)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for x := 0; x < len(sources); x++ {
			for y := x + 1; y < len(sources); y++ {
				CompareCodes(sources[x].Code, sources[y].Code)
			}
		}
	}
}

func BenchmarkNativeEngineScanRealistic(b *testing.B) {
	sources := benchRealisticCorpus(300)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := (NativeEngine{}).FindSimilar(context.Background(), sources, 0.7); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRustEngineScanRealistic(b *testing.B) {
	bin, err := locateRustBinary()
	if err != nil {
		b.Skipf("aioj-plagiarism not built: %v", err)
	}
	sources := benchRealisticCorpus(300)
	engine := NewRustEngine(bin)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.FindSimilar(context.Background(), sources, 0.7); err != nil {
			b.Fatal(err)
		}
	}
}
