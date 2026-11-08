package generate

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// The limit error used to read "AI generation limit reached", which told a setter
// nothing about what was blocking them or what to do next.
func TestLimitMessageNamesTheRunningJob(t *testing.T) {
	msg := limitMessage(1, 0, &model.AIGenerationJob{ID: "job-abc", Stage: "drafting"})

	for _, want := range []string{"generation running", "job-abc", "drafting", "cancel"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %s", want, msg)
		}
	}
	// It must not claim a queue limit that was not hit.
	if strings.Contains(msg, "queued") {
		t.Fatalf("message mentions the queue when the running limit was hit: %s", msg)
	}
}

func TestLimitMessageExplainsTheQueuedLimit(t *testing.T) {
	msg := limitMessage(0, maxQueuedPerActor, nil)

	if !strings.Contains(msg, "3 generations queued") {
		t.Fatalf("message should state the queue depth: %s", msg)
	}
	if !strings.Contains(msg, "limit 3") {
		t.Fatalf("message should state the limit: %s", msg)
	}
}

// A missing blocking job must still produce a usable message rather than an
// empty or half-built sentence.
func TestLimitMessageWithoutABlockingJob(t *testing.T) {
	msg := limitMessage(1, 0, nil)
	if !strings.Contains(msg, "generation running") || strings.Contains(msg, "%!") {
		t.Fatalf("message is malformed: %s", msg)
	}
	if strings.Contains(msg, "job-") {
		t.Fatalf("message invents a job id: %s", msg)
	}
}
