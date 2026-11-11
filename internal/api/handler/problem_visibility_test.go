package handler

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// A setter publishing straight to the archive was the behaviour this replaced.
// Only an admin may create a problem that is already published; everybody else
// starts as a draft and reaches the archive through the review queue.
func TestInitialStatus(t *testing.T) {
	cases := []struct {
		name      string
		requested bool
		role      string
		want      string
	}{
		{name: "admin can publish directly", requested: true, role: "admin", want: model.ProblemStatusPublished},
		{name: "admin can keep it a draft", requested: false, role: "admin", want: model.ProblemStatusDraft},
		{name: "a setter cannot publish", requested: true, role: "setter", want: model.ProblemStatusDraft},
		{name: "a setter can keep it a draft", requested: false, role: "setter", want: model.ProblemStatusDraft},
		{name: "an ordinary user cannot publish", requested: true, role: "user", want: model.ProblemStatusDraft},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := initialStatus(tc.requested, tc.role); got != tc.want {
				t.Fatalf("initialStatus(%v, %q) = %q, want %q", tc.requested, tc.role, got, tc.want)
			}
		})
	}
}
