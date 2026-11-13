package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

/**
 * The standings may say "B". They must never say what B is called.
 *
 * Reported ask: "from leaderboard remove problem names, keep only A, B, C...
 * problem name is info leaky." The board was already supposed to strip the
 * title, and it did — for a caller who could not read the round. What it did
 * NOT strip was the slug, which the store row carries alongside the title and
 * which is generated from the title at create time (slugFromTitle). So the name
 * was still on the wire, wearing a URL. Verified against the live round before
 * this test existed: an anonymous GET of the public board returned
 * {"index":"B","slug":"common-binary-harvest"} and eight more like it.
 *
 * The fix is a narrow projection (scoreboardProblem) rather than a strip, so the
 * test asserts against the RAW JSON instead of a decoded struct. A struct-with-
 * the-right-fields test would pass again the moment someone adds a field to
 * model.ContestProblem, which is exactly how this leaked: the strip enumerated
 * what to remove, and the projection enumerates what to send.
 *
 * That is the assertion worth having. It is deliberately a whitelist of every
 * key on the object, so a new column on the problem — visible, rating, tags, a
 * future `source_url` — fails here rather than in production.
 */
func TestScoreboardProblemPayloadCarriesNoName(t *testing.T) {
	db := openHandlerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	problemStore := postgres.NewProblemStore(db)
	h := NewContestHandler(contestStore, postgres.NewRatingStore(db), problemStore, userStore, nil)

	author := &model.User{
		ID: uuid.New().String(), Username: "sbp_" + uuid.New().String()[:8],
		Email:        "sbp_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, author); err != nil {
		t.Fatalf("create author: %v", err)
	}

	// The title and the slug are deliberately the same secret in two shapes, so a
	// leak of either is caught by the same assertion.
	const secretName = "Distinctive Secret Problem Name"
	const secretSlug = "distinctive-secret-problem-name"

	problem := &model.Problem{
		ID: uuid.New().String(), Slug: secretSlug, Title: secretName,
		Description: "secret statement body", TimeLimit: 1000, MemoryLimit: 262144,
		Status: model.ProblemStatusPublished, CreatedBy: author.ID,
	}
	if err := problemStore.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}

	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Name leak " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(3 * time.Hour),
		CreatedBy: author.ID, Status: model.ContestStatusPublished,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
		// A public board is the case that matters: it is watchable by someone the
		// content gate refuses, so it is the only board a stranger can read.
		ScoreboardVisibility: model.ScoreboardPublic,
	}
	if err := contestStore.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	if err := contestStore.AddProblem(ctx, contest.ID, problem.ID, "B", 100, 0); err != nil {
		t.Fatalf("attach problem: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_problems WHERE contest_id = $1", contest.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)
		db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", problem.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", author.ID)
	}()

	// Signed out, and the board is public — so the gate admits this caller and the
	// body is exactly what an internet stranger receives.
	rec := getScoreboard(t, h, contest.ID, "public", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("anonymous board = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	// Belt and braces: the names must not appear anywhere in the response, in case
	// they are nested somewhere this test does not inspect.
	if strings.Contains(body, secretName) {
		t.Errorf("the problem title reached a signed-out spectator's board:\n%s", body)
	}
	if strings.Contains(body, secretSlug) {
		t.Errorf("the problem slug reached a signed-out spectator's board (it is the title, URL-encoded):\n%s", body)
	}

	var payload struct {
		Problems []map[string]any `json:"problems"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode board: %v", err)
	}
	if len(payload.Problems) != 1 {
		t.Fatalf("problems = %d, want 1 (the fixture did not land)", len(payload.Problems))
	}

	// The whitelist. Every key the header is allowed to carry, and nothing else.
	allowed := map[string]bool{"problem_id": true, "index": true, "score": true}
	for key := range payload.Problems[0] {
		if !allowed[key] {
			t.Errorf("the board's problem header carries %q, which is not in the "+
				"allowlist %v — a field added to model.ContestProblem must not reach "+
				"this payload by default", key, allowed)
		}
	}
	// And the letter must still be there: a board that identifies problems by
	// nothing at all is not a fix.
	if got := payload.Problems[0]["index"]; got != "B" {
		t.Errorf("the letter = %v, want B — the board must still identify problems by letter", got)
	}
}
