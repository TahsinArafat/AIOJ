package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/feature"
	"github.com/tahsinarafat/aioj/internal/model"
)

const skipGateTestSecret = "test-secret-that-is-long-enough-for-hmac"

/**
 * The role that may void a submission is an admin decision, and the admin makes
 * it by editing the "skip_submission" feature policy (Admin → Feature Gates).
 * That is only true if the gate is actually mounted on the routes.
 *
 * A policy test proves the rule; this proves the requests are subject to it.
 * Without it, an admin could tighten the policy in the panel, watch it save, and
 * still have every setter able to void — the gate would simply be missing from
 * the route and nothing would say so.
 *
 * The refusal is asserted as a 403 naming the feature key, not merely as a
 * non-2xx: a route that failed for an unrelated reason must not be able to pass
 * for a working gate. Each request carries a real bearer token, so identity is
 * established (the gate needs a role to decide on) and the CSRF layer — which
 * every token-authenticated API call skips — cannot produce the 403 instead.
 */
func TestSkipRoutesAreSubjectToTheFeatureGate(t *testing.T) {
	jwt := auth.NewJWTManager(skipGateTestSecret, time.Minute, time.Hour)

	for _, tc := range []struct {
		name string
		path string
	}{
		{
			name: "the round's submissions tab",
			path: "/api/contests/just-cse-fest/submissions/some-submission/skip",
		},
		{
			name: "the problem's setter view",
			path: "/api/problems/two-sum/submissions/some-submission/skip",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := NewRouter(Deps{
				Submission: &handler.SubmissionHandler{},
				// Only admins may void, so the setter below must be refused.
				FeatureResolver: feature.NewResolver(&staticSkipSettings{
					policy: feature.Policy{Enabled: true, Roles: []string{model.RoleAdmin}},
				}, nil),
			}, jwt)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"reason":"x"}`))
			req.Header.Set("Authorization", "Bearer "+mustToken(t, jwt, "setter-1", model.RoleSetter))
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("POST %s = %d, want 403 — the skip gate is not mounted on this route (body=%q)",
					tc.path, rec.Code, rec.Body.String())
			}
			if body := rec.Body.String(); !strings.Contains(body, feature.KeySkipSubmission) {
				t.Errorf("the refusal does not name the %q feature, so it did not come from the gate:\n%s",
					feature.KeySkipSubmission, body)
			}
		})
	}
}

func mustToken(t *testing.T, jwt *auth.JWTManager, userID, role string) string {
	t.Helper()
	tok, err := jwt.GenerateAccessToken(userID, userID, role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}

// staticSkipSettings is a system_settings store backed by one policy, so the
// resolver can be driven without a database.
type staticSkipSettings struct{ policy feature.Policy }

func (s *staticSkipSettings) List(context.Context) ([]model.SystemSetting, error) {
	row, err := s.Get(context.Background(), feature.SettingKey(feature.KeySkipSubmission))
	if err != nil || row == nil {
		return nil, err
	}
	return []model.SystemSetting{*row}, nil
}

// Get answers for the skip_submission key only; every other key reads as absent,
// which the resolver treats as "use the registry default".
func (s *staticSkipSettings) Get(_ context.Context, key string) (*model.SystemSetting, error) {
	if key != feature.SettingKey(feature.KeySkipSubmission) {
		return nil, nil
	}
	roles := make([]string, 0, len(s.policy.Roles))
	for _, r := range s.policy.Roles {
		roles = append(roles, `"`+r+`"`)
	}
	raw := []byte(`{"enabled":true,"roles":[` + strings.Join(roles, ",") + `]}`)
	return &model.SystemSetting{Key: key, Value: raw, UpdatedAt: time.Now()}, nil
}

func (s *staticSkipSettings) Set(context.Context, *model.SystemSetting) error { return nil }
func (s *staticSkipSettings) Delete(context.Context, string) error            { return nil }
