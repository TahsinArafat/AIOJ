package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

/**
 * A round can be created from its name alone, and comes out unstarted.
 *
 * Creating a contest used to require a full schedule in the request: start_time
 * and end_time are NOT NULL with no column default, and the create page collected
 * six sections of settings before it would submit. The page is now name-only, so
 * the times arrive as Go's zero value.
 *
 * That is not a cosmetic default. Zero time is year 1, and every status derivation
 * compares the clock against start/end — so a round stored with zero times reads as
 * ENDED everywhere, and the client renders a two-thousand-year countdown. The
 * requirement is that a scheduled-nowhere round looks unstarted, which is what
 * accessNotStarted and the "Upcoming" badge mean.
 *
 * Asserted on the stored row and on the policy's answer, because the response body
 * echoes whatever was stored and would agree with either outcome.
 */
func TestContestCreateDefaultsAnUnscheduledRound(t *testing.T) {
	db := openHandlerTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := postgres.NewUserStore(db)
	contestStore := postgres.NewContestStore(db)
	problemStore := postgres.NewProblemStore(db)
	h := NewContestHandler(contestStore, postgres.NewRatingStore(db), problemStore, userStore, nil)

	owner := &model.User{
		ID: uuid.New().String(), Username: "cr8_" + uuid.New().String()[:8],
		Email:        "cr8_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: model.RoleSetter,
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/contests", bytes.NewBufferString(body))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
			&auth.Claims{UserID: owner.ID, Role: model.RoleSetter}))
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		return rec
	}

	t.Run("a name alone is enough", func(t *testing.T) {
		before := time.Now().UTC()
		rec := post(`{"title":"Unscheduled ` + uuid.New().String()[:8] + `"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create = %d, want 201 (body=%q)", rec.Code, rec.Body.String())
		}

		var created model.Contest
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", created.ID)

		got, err := contestStore.GetByID(ctx, created.ID)
		if err != nil || got == nil {
			t.Fatalf("reload: %v", err)
		}
		// Not the zero time. This is the whole point: year 1 is a valid TIMESTAMPTZ
		// and the insert would succeed, so the only thing standing between a name-only
		// create and a round that reads as ended is this default.
		if got.StartTime.IsZero() || got.StartTime.Year() < 2000 {
			t.Fatalf("start_time = %v, want a real upcoming instant — a zero time "+
				"makes every status derivation report the round as ended", got.StartTime)
		}
		if !got.StartTime.After(before) {
			t.Fatalf("start_time = %v is not after the request (%v), so the round is "+
				"not upcoming", got.StartTime, before)
		}
		if !got.EndTime.After(got.StartTime) {
			t.Fatalf("end_time = %v is not after start_time = %v", got.EndTime, got.StartTime)
		}

		// And the round is unstarted once published, which is the answer the badge and
		// the gates read. Published explicitly here because a setter's round starts as
		// a draft, and a draft is refused before any schedule is consulted — asserting
		// this on a draft would pass whatever start_time held.
		published := *got
		published.Visible = true
		published.Status = model.ContestStatusPublished
		p := newContestAccessPolicy(&accessStub{groupOK: true}, grantStub{})
		decision, err := p.Decide(ctx, &published, nil, model.SurfaceParticipate)
		if err != nil {
			t.Fatalf("decide: %v", err)
		}
		if !decision.IsUpcoming() {
			t.Fatalf("policy answered level %v, want accessNotStarted — a round with no "+
				"schedule must be unstarted, not live and not over", decision.level)
		}
	})

	t.Run("an end before the start is refused", func(t *testing.T) {
		rec := post(`{"title":"Backwards","start_time":"2026-06-01T10:00:00Z","end_time":"2026-06-01T09:00:00Z"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create = %d, want 400 — an end before the start would make the "+
				"round instantly over (body=%q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("a freeze outside the window is refused", func(t *testing.T) {
		rec := post(`{"title":"Bad freeze","start_time":"2026-06-01T10:00:00Z",
			"end_time":"2026-06-01T12:00:00Z","freeze_time":"2026-06-01T13:00:00Z"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("create = %d, want 400 — a freeze after the end either never "+
				"applies or hides the whole board (body=%q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("an explicit schedule is kept", func(t *testing.T) {
		// The default must not override a real answer. This is the case a careless
		// implementation breaks by assigning unconditionally.
		start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
		rec := post(`{"title":"Scheduled","start_time":"` + start.Format(time.RFC3339) +
			`","end_time":"` + start.Add(3*time.Hour).Format(time.RFC3339) + `"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create = %d, want 201 (body=%q)", rec.Code, rec.Body.String())
		}
		var created model.Contest
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", created.ID)

		got, _ := contestStore.GetByID(ctx, created.ID)
		if !got.StartTime.Equal(start) {
			t.Fatalf("start_time = %v, want the requested %v", got.StartTime, start)
		}
	})
}
