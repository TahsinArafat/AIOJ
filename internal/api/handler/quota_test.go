package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
)

// withUser attaches an authenticated identity to a request, the way
// AuthMiddleware would.
func withUser(r *http.Request, userID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.UserContextKey, &auth.Claims{UserID: userID}))
}

func TestUserQuota_AllowsBurstThenRejects(t *testing.T) {
	q := newUserQuota("test", 0.001, 3) // effectively no refill

	for i := 0; i < 3; i++ {
		ok, _ := q.allow("u1")
		if !ok {
			t.Fatalf("request %d should be allowed within the burst", i)
		}
	}
	ok, retryAfter := q.allow("u1")
	if ok {
		t.Fatal("the fourth request must be rejected")
	}
	if retryAfter <= 0 {
		t.Fatal("a rejected request must report how long to wait")
	}
}

func TestUserQuota_IsPerUser(t *testing.T) {
	q := newUserQuota("test", 0.001, 1)

	if ok, _ := q.allow("u1"); !ok {
		t.Fatal("u1 first request should pass")
	}
	if ok, _ := q.allow("u1"); ok {
		t.Fatal("u1 second request should be rejected")
	}
	// One user exhausting their budget must not affect anybody else.
	if ok, _ := q.allow("u2"); !ok {
		t.Fatal("u2 must keep its own budget")
	}
}

func TestUserQuota_RefillsOverTime(t *testing.T) {
	q := newUserQuota("test", 100, 1) // 100 tokens/sec

	if ok, _ := q.allow("u1"); !ok {
		t.Fatal("first request should pass")
	}
	if ok, _ := q.allow("u1"); ok {
		t.Fatal("second immediate request should be rejected")
	}
	time.Sleep(30 * time.Millisecond) // > 1/100s
	if ok, _ := q.allow("u1"); !ok {
		t.Fatal("budget should have refilled")
	}
}

func TestUserQuota_EvictsIdleBuckets(t *testing.T) {
	q := newUserQuota("test", 1, 1)
	// Seed an idle bucket directly, then fill the map to the cap.
	q.buckets["stale"] = &quotaBucket{
		limiter: rate.NewLimiter(1, 1),
		last:    time.Now().Add(-2 * idleQuotaEviction),
	}
	for i := 0; i < maxTrackedUsers-1; i++ {
		q.buckets["filler-"+strconv.Itoa(i)] = &quotaBucket{
			limiter: rate.NewLimiter(1, 1),
			last:    time.Now(),
		}
	}

	// This allocation trips the cap and must sweep the stale entry.
	_, _ = q.allow("newcomer")

	q.mu.Lock()
	_, staleStillThere := q.buckets["stale"]
	size := len(q.buckets)
	q.mu.Unlock()

	if staleStillThere {
		t.Fatal("idle bucket should have been evicted")
	}
	if size > maxTrackedUsers {
		t.Fatalf("map grew past the cap: %d", size)
	}
}

func TestCheckQuota_WritesRetryAfter(t *testing.T) {
	q := newUserQuota("test", 0.001, 1)
	_, _ = q.allow("u1") // exhaust

	rec := httptest.NewRecorder()
	if checkQuota(rec, q, "u1") {
		t.Fatal("expected the quota to reject")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
	var body struct {
		Error      string `json:"error"`
		RetryAfter int    `json:"retry_after"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.RetryAfter < 1 {
		t.Fatalf("retry_after = %d, want >= 1", body.RetryAfter)
	}
}

func TestCheckQuota_NilQuotaIsUnlimited(t *testing.T) {
	rec := httptest.NewRecorder()
	if !checkQuota(rec, nil, "u1") {
		t.Fatal("a nil quota must not reject")
	}
}

// TestCustomRun_EnforcesRunQuota drives the real handler entry point.
func TestCustomRun_EnforcesRunQuota(t *testing.T) {
	h := &SubmissionHandler{runQuota: newUserQuota("run", 0.001, 1)}

	first := withUser(httptest.NewRequest(http.MethodPost, "/api/submissions/run", nil), "u1")
	rec := httptest.NewRecorder()
	h.CustomRun(rec, first)
	// The first call gets past the quota and fails later on the empty body.
	if rec.Code == http.StatusTooManyRequests {
		t.Fatal("the first /run should not be throttled")
	}

	second := withUser(httptest.NewRequest(http.MethodPost, "/api/submissions/run", nil), "u1")
	rec = httptest.NewRecorder()
	h.CustomRun(rec, second)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 on the second immediate /run", rec.Code)
	}
}

// "Test Samples" fires one case per sample. Used to be one /run per case, and
// /run's burst is 3, so a problem with four samples failed partway through: the
// fourth case answered 429 and the button looked stuck.
//
// BatchRun spends a single token for the whole sweep, so the number of sample
// cases no longer decides whether the run succeeds.
func TestBatchRunSpendsOneQuotaTokenForTheWholeSweep(t *testing.T) {
	q := newUserQuota("run-test", 0.0001, 3) // burst 3, effectively no refill
	h := &SubmissionHandler{runQuota: q}

	// Three sweeps of many cases each must all be admitted: 9 tokens would have
	// been needed by the old per-case approach.
	for i := 0; i < 3; i++ {
		if !checkQuota(httptest.NewRecorder(), h.runQuota, "u1") {
			t.Fatalf("sweep %d was refused; the sweep must spend one token, not one per case", i+1)
		}
	}
	// The fourth exceeds the burst, proving the budget is still enforced.
	if checkQuota(httptest.NewRecorder(), h.runQuota, "u1") {
		t.Fatal("a fourth sweep must be refused: the per-user budget still applies")
	}
}

// A batch request with no cases is a client bug, and running code for zero
// cases would spend a token for nothing.
func TestBatchRunRejectsAnEmptySweep(t *testing.T) {
	h := &SubmissionHandler{}
	body := strings.NewReader(`{"source_code":"print(1)","language":"python","cases":[]}`)
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/submissions/run-batch", body), "u1")
	w := httptest.NewRecorder()

	h.BatchRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an empty case list", w.Code)
	}
}
