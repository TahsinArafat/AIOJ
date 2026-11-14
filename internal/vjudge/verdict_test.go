package vjudge

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * The verdict vocabulary and its mapping to a stored status.
 *
 * This table exists because the mapping used to live in four places — the
 * force-poll path, the generic poll path, and one per-platform normalizer on QOJ
 * and Toph — and all four had drifted from what the bots actually emit. None of
 * them knew about MLE, although three bots produce it, so a remote submission
 * that exceeded its memory limit was recorded as Wrong Answer on every platform.
 * The platform normalizers were also inconsistent about the unknown case: QOJ
 * and Toph returned "" for an unlisted verdict, which is the same answer as
 * "still judging", so such a submission was polled until it timed out.
 *
 * A single mapper makes those failure modes testable. Each case below is a
 * spelling a real platform sends.
 */

func TestVerdictStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		verdict Verdict
		want    model.SubmissionStatus
		why     string
	}{
		{VerdictAccepted, model.StatusAC, "accepted"},
		{VerdictWrong, model.StatusWA, "wrong answer"},
		{VerdictTimeLimit, model.StatusTLE, "time limit"},
		// The regression this file was written for: MLE must not become WA.
		{VerdictMemoryLimit, model.StatusMLE, "memory limit is its own status, not a wrong answer"},
		{VerdictRuntime, model.StatusRE, "runtime error"},
		{VerdictCompile, model.StatusCE, "compile error"},
		// OLE and PE have no slot in the verdict enum; WA is the honest nearest.
		{VerdictOutputLimit, model.StatusWA, "output limit maps to WA, the enum has no slot"},
		{VerdictPresentation, model.StatusWA, "presentation error maps to WA"},
		// An unrecognised verdict must not be able to claim an accepted submission.
		{Verdict("SOMETHING_NEW"), model.StatusWA, "unknown verdict understates rather than overstates"},
	} {
		if got := tc.verdict.Status(); got != tc.want {
			t.Errorf("%s.Status() = %q, want %q (%s)", tc.verdict, got, tc.want, tc.why)
		}
	}
}

func TestNormalizeVerdictSpellings(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want Verdict
		why  string
	}{
		// The same outcome, phrased the three ways platforms actually send it.
		{"OK", VerdictAccepted, "Codeforces JSON"},
		{"TIME_LIMIT_EXCEEDED", VerdictTimeLimit, "Codeforces enum style"},
		{"Time Limit Exceeded", VerdictTimeLimit, "human-readable text"},
		{"tle", VerdictTimeLimit, "an abbreviation"},
		{"Accepted", VerdictAccepted, "AtCoder text"},
		{"accepted", VerdictAccepted, "lowercase"},
		{"AC", VerdictAccepted, "the canonical code"},
		{"MEMORY_LIMIT_EXCEEDED", VerdictMemoryLimit, "the verdict that was being lost"},
		{"Memory Limit Exceeded", VerdictMemoryLimit, "spelled out"},
		{"mle", VerdictMemoryLimit, "abbreviated"},
		{"OUTPUT_LIMIT_EXCEEDED", VerdictOutputLimit, "output limit"},
		{"Output Limit Exceeded", VerdictOutputLimit, "spelled out"},
		{"PRESENTATION_ERROR", VerdictPresentation, "presentation error"},
		{"Runtime Error", VerdictRuntime, "with a space"},
		{"RUNTIME_ERROR", VerdictRuntime, "with underscores"},
		{"nzec", VerdictRuntime, "non-zero exit code is a runtime error"},
		{"COMPILATION_ERROR", VerdictCompile, "Codeforces style"},
		{"Compilation Error", VerdictCompile, "spelled out"},
		{"WRONG_ANSWER", VerdictWrong, "Codeforces style"},
		{"IDLENESS_LIMIT_EXCEEDED", VerdictTimeLimit, "CF idleness means the program stopped reading"},
		// Non-final answers: a poller must keep asking.
		{"TESTING", VerdictPending, "CF"},
		{"PENDING", VerdictPending, "canonical"},
		{"Running", VerdictPending, "QOJ"},
		{"Waiting for Judging", VerdictPending, "AtCoder"},
		{"Submitted", VerdictPending, "accepted but not yet judged"},
		{"judging", VerdictPending, "lowercase"},
		{"", VerdictPending, "an empty verdict is not a verdict"},
		{"   ", VerdictPending, "whitespace is not a verdict"},
	} {
		if got := normalizeVerdict(tc.raw); got != tc.want {
			t.Errorf("normalizeVerdict(%q) = %q, want %q (%s)", tc.raw, got, tc.want, tc.why)
		}
	}
}

// The MLE regression, stated as the property rather than as a case: every
// verdict a bot can emit must survive the round trip to a stored status without
// silently becoming "wrong answer" unless that is the intended answer.
func TestEveryEmittedVerdictIsKnown(t *testing.T) {
	// Taken from what the bots assign to RemoteResult.Verdict.
	for _, emitted := range []string{"AC", "WA", "TLE", "MLE", "RE", "CE", "OLE", "PE", "PENDING"} {
		v := normalizeVerdict(emitted)
		if v == VerdictWrong && emitted != "WA" && emitted != "OLE" && emitted != "PE" {
			t.Errorf("verdict %q the bots emit normalizes to WA, so the real outcome is lost", emitted)
		}
	}
	// And the enum slots that exist must be reachable.
	for _, want := range []model.SubmissionStatus{
		model.StatusAC, model.StatusWA, model.StatusTLE, model.StatusMLE,
		model.StatusRE, model.StatusCE,
	} {
		reached := false
		for _, v := range []Verdict{
			VerdictAccepted, VerdictWrong, VerdictTimeLimit, VerdictMemoryLimit,
			VerdictRuntime, VerdictCompile,
		} {
			if v.Status() == want {
				reached = true
			}
		}
		if !reached {
			t.Errorf("no verdict maps to %q, so a submission can never earn it from a remote judge", want)
		}
	}
}

func TestVerdictIsDone(t *testing.T) {
	if VerdictPending.IsDone() {
		t.Error("pending must not be done, or a poller stops before the judge has answered")
	}
	if Verdict("").IsDone() {
		t.Error("an empty verdict must not be done")
	}
	for _, v := range []Verdict{VerdictAccepted, VerdictWrong, VerdictTimeLimit, VerdictMemoryLimit, VerdictRuntime, VerdictCompile, VerdictOutputLimit, VerdictPresentation} {
		if !v.IsDone() {
			t.Errorf("%q is a settled verdict, so IsDone must be true", v)
		}
	}
}

// TestMapCFSubmissionBatch covers the judgement inside the Codeforces batch
// poll: which remote ids match, which are still judging, and the platform's
// units. Extracted as a free function precisely so this can be asserted without
// a Codeforces account or an HTTP server.
func TestMapCFSubmissionBatch(t *testing.T) {
	// A user.status result, in the shape the CF API returns: ids as JSON
	// numbers, time in milliseconds, memory in bytes.
	result := []map[string]interface{}{
		{"id": float64(1001), "verdict": "OK", "timeConsumedMillis": float64(31), "memoryConsumedBytes": float64(262144)},
		// The verdict that used to be lost: an MLE must not arrive as WA.
		{"id": float64(1002), "verdict": "MEMORY_LIMIT_EXCEEDED", "timeConsumedMillis": float64(7), "memoryConsumedBytes": float64(1048576)},
		{"id": float64(1003), "verdict": "TIME_LIMIT_EXCEEDED", "timeConsumedMillis": float64(2000), "memoryConsumedBytes": float64(1024)},
		// Still judging: must be absent so the caller keeps polling.
		{"id": float64(1004), "verdict": "TESTING"},
		// A verdict field that is missing entirely is not a verdict.
		{"id": float64(1005)},
		// Someone else's submission, reported by the same handle query.
		{"id": float64(9999), "verdict": "OK"},
	}

	want := []PendingResult{
		{SubmissionID: "s1", RemoteID: "1001"},
		{SubmissionID: "s2", RemoteID: "1002"},
		{SubmissionID: "s3", RemoteID: "1003"},
		{SubmissionID: "s4", RemoteID: "1004"},
		{SubmissionID: "s5", RemoteID: "1005"},
		// Asked about but not present in the batch at all.
		{SubmissionID: "s6", RemoteID: "1006"},
	}

	got := mapCFSubmissionBatch(result, want)

	if len(got) != 3 {
		t.Fatalf("got %d results, want 3 (AC, MLE, TLE): %+v", len(got), got)
	}

	ac, ok := got["1001"]
	if !ok || ac.Verdict != VerdictAccepted || !ac.Done {
		t.Errorf("1001 = %+v, want accepted and done", got["1001"])
	}
	// Units: 31 ms stays 31 ms; 262144 bytes becomes 256 KB.
	if ac.TimeUsed != 31 {
		t.Errorf("1001 time = %d ms, want 31 (already milliseconds)", ac.TimeUsed)
	}
	if ac.MemoryUsed != 256 {
		t.Errorf("1001 memory = %d KB, want 256 (bytes converted)", ac.MemoryUsed)
	}

	mle, ok := got["1002"]
	if !ok {
		t.Fatal("1002 (MEMORY_LIMIT_EXCEEDED) is missing; it must not be dropped")
	}
	if mle.Verdict != VerdictMemoryLimit {
		t.Errorf("1002 verdict = %q, want %q — an MLE must not become a wrong answer", mle.Verdict, VerdictMemoryLimit)
	}
	if mle.Verdict.Status() != model.StatusMLE {
		t.Errorf("1002 status = %q, want %q", mle.Verdict.Status(), model.StatusMLE)
	}

	// Still-judging and verdict-less entries must be absent, not guessed at.
	for _, id := range []string{"1004", "1005", "1006"} {
		if r, ok := got[id]; ok {
			t.Errorf("id %s must be absent (still judging, no verdict, or not in the batch), got %+v", id, r)
		}
	}
	// A submission the caller never asked about must not be reported.
	if _, ok := got["9999"]; ok {
		t.Error("9999 was not requested but came back; one handle's history must not write unrelated rows")
	}
}

// A nil entry in the parsed result must not panic the poll worker.
func TestMapCFSubmissionBatchToleratesNilEntries(t *testing.T) {
	got := mapCFSubmissionBatch(
		[]map[string]interface{}{nil, {"id": float64(1), "verdict": "OK"}},
		[]PendingResult{{SubmissionID: "s", RemoteID: "1"}},
	)
	if len(got) != 1 || got["1"].Verdict != VerdictAccepted {
		t.Fatalf("got %+v, want the one valid entry", got)
	}
}

// TestBotRegistryCoversEveryLanguagePlatform ties the registry to the list the
// rest of the server uses to decide what "remote" means. A platform in
// model.RemotePlatforms with no registered bot is a problem that cannot be
// judged, and a bot for a platform the model does not know is unreachable —
// both silent, both worth failing on.
func TestBotRegistryCoversEveryLanguagePlatform(t *testing.T) {
	registered := make(map[string]bool)
	for _, p := range RegisteredPlatforms() {
		registered[p] = true
	}

	for _, p := range model.RemotePlatforms {
		if !registered[p] {
			t.Errorf("model.RemotePlatforms lists %q but no bot is registered for it, "+
				"so a problem imported from there can never be judged", p)
		}
	}
	for p := range registered {
		if !model.IsRemotePlatform(p) {
			t.Errorf("a bot is registered for %q but model.RemotePlatforms does not list it, "+
				"so submissions to it are not routed as remote", p)
		}
	}
}

// Every registered bot must satisfy the mandatory interface. A factory that
// returns something incomplete would only fail at the first submission.
func TestRegisteredBotsImplementBot(t *testing.T) {
	deps := BotDeps{RemoteLang: nil}
	for _, platform := range RegisteredPlatforms() {
		bot, ok := NewBot(platform, BotConfig{}, deps)
		if !ok {
			t.Fatalf("NewBot(%q) reported no factory", platform)
		}
		if bot == nil {
			t.Fatalf("NewBot(%q) returned a nil bot", platform)
		}
		if bot.Name() != platform {
			t.Errorf("bot registered as %q reports Name() = %q; the registry key and the "+
				"bot's own name must agree or GetBotNames and Lookup disagree", platform, bot.Name())
		}
	}
}

func TestNewBotReportsUnknownPlatform(t *testing.T) {
	if _, ok := NewBot("no-such-judge", BotConfig{}, BotDeps{}); ok {
		t.Error("an unregistered platform must report ok=false")
	}
	if _, err := NewBotChecked("no-such-judge", BotConfig{}, BotDeps{}); err == nil {
		t.Error("NewBotChecked must return an error for an unregistered platform")
	}
}
