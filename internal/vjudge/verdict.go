package vjudge

import (
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

// Verdict is a remote judge's verdict, normalized to the vocabulary this server
// stores. Every bot maps its platform's wording into one of these, and this
// package is the only place that translates them into model.SubmissionStatus.
//
// The two steps are separate because they answer different questions. A bot
// knows its platform's HTTP response; it does not know what this server's
// verdict enum contains, and it should not have to — that is one mapping, and
// it lives here.
//
// Pending is spelled out rather than left to the zero value so a bot that
// forgets to set a verdict cannot silently claim an accepted submission.
type Verdict string

const (
	VerdictPending      Verdict = "PENDING"
	VerdictAccepted     Verdict = "AC"
	VerdictWrong        Verdict = "WA"
	VerdictTimeLimit    Verdict = "TLE"
	VerdictMemoryLimit  Verdict = "MLE"
	VerdictRuntime      Verdict = "RE"
	VerdictCompile      Verdict = "CE"
	VerdictOutputLimit  Verdict = "OLE"
	VerdictPresentation Verdict = "PE"
)

// Status maps a normalized verdict onto the stored submission status.
//
// An unrecognized verdict becomes Wrong Answer rather than Accepted. The old
// four copies of this switch all did the same thing, and that default is
// deliberate: of the two silent answers, "wrong" understates a good submission,
// which a user will notice and report, while "accepted" overstates a bad one,
// which silently corrupts standings and ratings.
//
// This function is why the switch existed in four places and drifted: the
// per-platform polling paths each carried their own copy, and none of them knew
// about MLE, OLE or PE — three verdicts the bots do emit. A memory-limit
// exceeded submission was recorded as Wrong Answer on every remote platform.
func (v Verdict) Status() model.SubmissionStatus {
	switch v {
	case VerdictAccepted:
		return model.StatusAC
	case VerdictTimeLimit:
		return model.StatusTLE
	case VerdictMemoryLimit:
		return model.StatusMLE
	case VerdictRuntime:
		return model.StatusRE
	case VerdictCompile:
		return model.StatusCE
	case VerdictOutputLimit, VerdictPresentation:
		// Output Limit Exceeded and Presentation Error both mean "produced
		// output this problem did not want". The verdict enum has no slot for
		// either, and inventing one would need a migration plus a UI change for
		// a distinction most judges do not draw. Wrong Answer is the honest
		// nearest answer, and the raw platform verdict is still in the log.
		return model.StatusWA
	case VerdictWrong:
		return model.StatusWA
	default:
		return model.StatusWA
	}
}

// normalizeVerdict maps the many spellings platforms use onto a Verdict.
//
// Substring matching, because the platforms phrase the same outcome
// differently ("Time Limit Exceeded", "TIME_LIMIT_EXCEEDED", "tle") and the
// alternative is a table per platform that must be extended whenever a judge
// rewords its output. Order matters: "memory limit" must be tested before
// "limit", and every limit check before the bare "re"/"wa" fallbacks.
func normalizeVerdict(raw string) Verdict {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return VerdictPending
	}
	// Collapse separators so TIME_LIMIT_EXCEEDED and "time limit exceeded" agree.
	s = strings.NewReplacer("_", " ", "-", " ").Replace(s)

	switch {
	case strings.Contains(s, "accept"), strings.Contains(s, "solved"), s == "ok", s == "ac":
		return VerdictAccepted
	case strings.Contains(s, "compil"), s == "ce":
		return VerdictCompile
	case strings.Contains(s, "memory limit"), s == "mle", s == "ml":
		return VerdictMemoryLimit
	case strings.Contains(s, "output limit"), s == "ole", s == "ol":
		return VerdictOutputLimit
	case strings.Contains(s, "time limit"), s == "tle", s == "tl":
		return VerdictTimeLimit
	case strings.Contains(s, "presentation"), s == "pe":
		return VerdictPresentation
	case strings.Contains(s, "runtime"), s == "re", s == "rte", s == "nzec":
		return VerdictRuntime
	case strings.Contains(s, "wrong"), s == "wa":
		return VerdictWrong
	case strings.Contains(s, "idleness"):
		// Codeforces reports a submission that idled as its own verdict. Its
		// usual cause is a program that stopped reading input, which the problem
		// treats as exceeding the time limit.
		return VerdictTimeLimit
	case strings.Contains(s, "submitted"),
		strings.Contains(s, "pending"),
		strings.Contains(s, "judging"),
		strings.Contains(s, "running"),
		strings.Contains(s, "waiting"),
		strings.Contains(s, "queue"),
		strings.Contains(s, "testing"):
		return VerdictPending
	default:
		// An unknown verdict is dispatched above to Wrong Answer; see Status.
		return VerdictWrong
	}
}

// IsDone reports whether a verdict is final, so a poller knows to stop asking.
//
// Pending and the empty string are the only non-final answers: every other value
// in the vocabulary is a verdict the judge has settled on.
func (v Verdict) IsDone() bool {
	return v != VerdictPending && v != ""
}
