package judge

import (
	"context"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// memoryStore is the minimal submission surface the admission path needs. It
// records the verdict the pool writes so the test can assert on it.
type memoryStore struct {
	sub     *model.Submission
	claimed bool
	status  model.SubmissionStatus
	detail  string
}

func (s *memoryStore) GetByID(context.Context, string) (*model.Submission, error) { return s.sub, nil }

func (s *memoryStore) ClaimPending(context.Context, string, string) (bool, error) {
	s.claimed = true
	return true, nil
}

func (s *memoryStore) UpdateResultClaimed(_ context.Context, _, _ string, status model.SubmissionStatus, _, _, _ int, compileOutput string, _ []model.TestCaseResult) (bool, error) {
	s.status = status
	s.detail = compileOutput
	return true, nil
}

type memoryProblemStore struct{ prob *model.Problem }

func (s *memoryProblemStore) GetByID(context.Context, string) (*model.Problem, error) {
	return s.prob, nil
}
func (s *memoryProblemStore) UpdateCounts(context.Context, string, int, int) error { return nil }

// newMemoryPool builds a pool that reads real language configs, so the
// multiplier under test is the one production would apply.
func newMemoryPool(t *testing.T, sub *model.Submission, prob *model.Problem) (*WorkerPool, *memoryStore) {
	t.Helper()
	store := &memoryStore{sub: sub}
	return &WorkerPool{
		subStore:  store,
		probStore: &memoryProblemStore{prob: prob},
		langDir:   "../../lang",
	}, store
}

func intPtr(v int) *int { return &v }

func TestRequiredMemoryBytesAppliesLanguageMultiplier(t *testing.T) {
	cases := []struct {
		name       string
		language   string
		problemMB  int
		overrideKB *int
		wantBytes  uint64
	}{
		{
			name: "c++ has no multiplier", language: "cpp-gpp-64",
			problemMB: 256, wantBytes: 256 << 20,
		},
		{
			// Java's 2.0 multiplier is why admission cannot just read the
			// problem row: 256 MB on paper is 512 MB to the sandbox.
			name: "java doubles the requirement", language: "java",
			problemMB: 256, wantBytes: 512 << 20,
		},
		{
			name: "java at the 2 GB ceiling needs 4 GB", language: "java",
			problemMB: 2048, wantBytes: 4096 << 20,
		},
		{
			// A language_limits override replaces the base BEFORE the
			// multiplier, which is the supported way to offer a 2 GB Java
			// problem on a 3 GB budget.
			name: "language override replaces the base", language: "java",
			problemMB: 2048, overrideKB: intPtr(1048576), wantBytes: 2048 << 20,
		},
		{
			name: "unknown language reserves nothing", language: "not-a-language",
			problemMB: 2048, wantBytes: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prob := &model.Problem{ID: "p1", TimeLimit: 1000, MemoryLimit: tc.problemMB * 1024}
			if tc.overrideKB != nil {
				prob.LanguageLimits = []model.LanguageLimit{
					{LanguageID: tc.language, MemoryLimitKB: tc.overrideKB},
				}
			}
			pool, _ := newMemoryPool(t, &model.Submission{ProblemID: "p1", Language: tc.language}, prob)

			got, err := pool.requiredMemoryBytes(context.Background(), "s1")
			if err != nil {
				t.Fatalf("requiredMemoryBytes: %v", err)
			}
			if got != tc.wantBytes {
				t.Fatalf("requiredMemoryBytes = %d bytes (%d MB), want %d bytes (%d MB)",
					got, got>>20, tc.wantBytes, tc.wantBytes>>20)
			}
		})
	}
}

func TestRejectOversizedWritesExplanatoryVerdict(t *testing.T) {
	pool, store := newMemoryPool(t,
		&model.Submission{ProblemID: "p1", Language: "java"},
		&model.Problem{ID: "p1", TimeLimit: 1000, MemoryLimit: 2048 * 1024},
	)
	pool.WithMaxJudgmentMemory(3072 << 20)

	// 2048 MB x Java's 2.0 = 4096 MB, over the 3072 MB budget.
	pool.rejectOversized(context.Background(), "s1", 4096<<20)

	if !store.claimed {
		t.Fatal("rejectOversized must claim the submission before writing a verdict")
	}
	if store.status != model.StatusSE {
		t.Fatalf("status = %q, want %q", store.status, model.StatusSE)
	}
	// The message has to tell an admin what to do; a bare "system error" is the
	// thing this replaces.
	for _, want := range []string{"4096 MB", "3072 MB", "JUDGE_MAX_JUDGMENT_MEM_MB", "memory_limit_kb"} {
		if !strings.Contains(store.detail, want) {
			t.Errorf("verdict detail %q does not mention %q", store.detail, want)
		}
	}
}

func TestRequiredMemoryBytesPropagatesMissingSubmission(t *testing.T) {
	pool := &WorkerPool{
		subStore:  &memoryStore{sub: nil},
		probStore: &memoryProblemStore{prob: &model.Problem{ID: "p1"}},
		langDir:   "../../lang",
	}
	if _, err := pool.requiredMemoryBytes(context.Background(), "s1"); err == nil {
		t.Fatal("expected an error for a missing submission")
	}
}
