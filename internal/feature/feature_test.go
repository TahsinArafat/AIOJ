package feature

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

func TestDecide(t *testing.T) {
	gym, ok := Lookup(KeyGym)
	if !ok {
		t.Fatal("gym must be in the registry")
	}
	setterOnly, ok := Lookup(KeySetterWorkspace)
	if !ok {
		t.Fatal("setter_workspace must be in the registry")
	}

	admin := Principal{UserID: "a", Role: model.RoleAdmin, Rating: 0}
	setter := Principal{UserID: "s", Role: model.RoleSetter, Rating: 1800}
	user := Principal{UserID: "u", Role: model.RoleUser, Rating: 1100}
	anon := Principal{Anonymous: true}

	tests := []struct {
		name      string
		feature   Feature
		policy    Policy
		principal Principal
		want      string // "" means allowed
	}{
		{
			name:    "default policy allows everyone",
			feature: gym,
			policy:  Unrestricted,
			want:    "",
		},
		{
			name:    "master switch denies even admins",
			feature: gym,
			policy:  Policy{Enabled: false},
			// The switch is absolute: an admin who turned a feature off must
			// not still see it, or "off" would be a lie for the only person
			// who can observe it.
			principal: admin,
			want:      ReasonDisabled,
		},
		{
			name:      "master switch denies anonymous",
			feature:   gym,
			policy:    Policy{Enabled: false},
			principal: anon,
			want:      ReasonDisabled,
		},
		{
			name:      "admin bypasses the role list",
			feature:   gym,
			policy:    Policy{Enabled: true, Roles: []string{model.RoleSetter}},
			principal: admin,
			want:      "",
		},
		{
			name:      "admin bypasses the rating window",
			feature:   gym,
			policy:    Policy{Enabled: true, MinRating: 2000},
			principal: admin,
			want:      "",
		},
		{
			name:      "role outside the list is denied",
			feature:   gym,
			policy:    Policy{Enabled: true, Roles: []string{model.RoleSetter}},
			principal: user,
			want:      ReasonRole,
		},
		{
			name:      "role inside the list is allowed",
			feature:   gym,
			policy:    Policy{Enabled: true, Roles: []string{model.RoleSetter}},
			principal: setter,
			want:      "",
		},
		{
			name:      "anonymous hitting a role-restricted feature is asked to log in",
			feature:   gym,
			policy:    Policy{Enabled: true, Roles: []string{model.RoleUser}},
			principal: anon,
			want:      ReasonLoginRequired,
		},
		{
			name:      "rating below the minimum is denied",
			feature:   gym,
			policy:    Policy{Enabled: true, MinRating: 1400},
			principal: user,
			want:      ReasonRating,
		},
		{
			name:      "rating at the minimum is allowed",
			feature:   gym,
			policy:    Policy{Enabled: true, MinRating: 1100},
			principal: user,
			want:      "",
		},
		{
			name:      "rating above the maximum is denied",
			feature:   gym,
			policy:    Policy{Enabled: true, MaxRating: 1000},
			principal: user,
			want:      ReasonRating,
		},
		{
			name:      "rating window includes the caller",
			feature:   gym,
			policy:    Policy{Enabled: true, MinRating: 1000, MaxRating: 1200},
			principal: user,
			want:      "",
		},
		{
			name:      "anonymous hitting a rating-gated feature is asked to log in",
			feature:   gym,
			policy:    Policy{Enabled: true, MinRating: 1200},
			principal: anon,
			want:      ReasonLoginRequired,
		},
		{
			name:      "requires-login denies anonymous on an otherwise open feature",
			feature:   setterOnly,
			policy:    Unrestricted,
			principal: anon,
			want:      ReasonLoginRequired,
		},
		{
			name:      "requires-login allows a signed-in user",
			feature:   setterOnly,
			policy:    Unrestricted,
			principal: user,
			want:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(tc.feature, tc.policy, tc.principal)
			if got.Reason != tc.want {
				t.Fatalf("reason = %q, want %q (decision %+v)", got.Reason, tc.want, got)
			}
			if got.Allowed != (tc.want == "") {
				t.Fatalf("allowed = %v, want %v", got.Allowed, tc.want == "")
			}
		})
	}
}

func TestDecideCarriesAdminMessage(t *testing.T) {
	gym, _ := Lookup(KeyGym)
	policy := Policy{Enabled: true, MinRating: 1500, Message: "Div. 2 opens on Sunday."}
	got := Decide(gym, policy, Principal{UserID: "u", Role: model.RoleUser, Rating: 900})
	if got.Allowed {
		t.Fatal("expected a denial")
	}
	if got.Message != "Div. 2 opens on Sunday." {
		t.Fatalf("message = %q, want the admin message", got.Message)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		policy  Policy
		wantErr string
	}{
		{name: "unrestricted", policy: Unrestricted},
		{name: "known roles", policy: Policy{Enabled: true, Roles: []string{model.RoleAdmin, model.RoleUser}}},
		{name: "rating window", policy: Policy{Enabled: true, MinRating: 1200, MaxRating: 1800}},
		{
			name:    "unknown role",
			policy:  Policy{Enabled: true, Roles: []string{"wizard"}},
			wantErr: "unknown role",
		},
		{
			name:    "inverted rating window",
			policy:  Policy{Enabled: true, MinRating: 1800, MaxRating: 1200},
			wantErr: "must not exceed",
		},
		{
			name:    "negative minimum",
			policy:  Policy{Enabled: true, MinRating: -1},
			wantErr: "min_rating must be between",
		},
		{
			name:    "absurd maximum",
			policy:  Policy{Enabled: true, MaxRating: 999999},
			wantErr: "max_rating must be between",
		},
		{
			name:    "message too long",
			policy:  Policy{Enabled: true, Message: strings.Repeat("x", maxMessageLen+1)},
			wantErr: "at most",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.policy)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize(Policy{
		Enabled: true,
		Roles:   []string{model.RoleUser, "wizard", model.RoleAdmin, model.RoleUser},
		Message: "  spaced  ",
	})
	want := []string{model.RoleAdmin, model.RoleUser}
	if len(got.Roles) != len(want) {
		t.Fatalf("roles = %v, want %v", got.Roles, want)
	}
	for i := range want {
		if got.Roles[i] != want[i] {
			t.Fatalf("roles = %v, want %v (sorted, de-duplicated, known only)", got.Roles, want)
		}
	}
	if got.Message != "spaced" {
		t.Fatalf("message = %q, want it trimmed", got.Message)
	}
}

func TestEqualIgnoresRoleOrderAndNoise(t *testing.T) {
	a := Policy{Enabled: true, Roles: []string{model.RoleUser, model.RoleAdmin}}
	b := Policy{Enabled: true, Roles: []string{model.RoleAdmin, model.RoleUser, model.RoleUser}}
	if !Equal(a, b) {
		t.Fatal("policies that gate identically must compare equal")
	}
	if Equal(a, Policy{Enabled: true, Roles: []string{model.RoleAdmin}}) {
		t.Fatal("different role sets must not compare equal")
	}
}

func TestNeedsRating(t *testing.T) {
	if NeedsRating(Unrestricted) {
		t.Fatal("an unrestricted policy must not need a rating lookup")
	}
	if !NeedsRating(Policy{Enabled: true, MinRating: 1200}) {
		t.Fatal("a rating floor must need a rating lookup")
	}
	if NeedsRating(Policy{Enabled: false, MinRating: 1200}) {
		t.Fatal("a disabled feature must not need a rating lookup")
	}
}

func TestRegistryIsConsistent(t *testing.T) {
	seen := make(map[string]struct{}, len(registry))
	for _, f := range registry {
		if f.Key == "" || f.Label == "" || f.Description == "" {
			t.Fatalf("feature %+v is missing key/label/description", f)
		}
		if _, dup := seen[f.Key]; dup {
			t.Fatalf("duplicate feature key %q", f.Key)
		}
		seen[f.Key] = struct{}{}
		if err := Validate(f.Default); err != nil {
			t.Fatalf("feature %q has an invalid default policy: %v", f.Key, err)
		}
		if !f.Default.Enabled {
			t.Fatalf("feature %q must default to enabled; shipping a gate closed would hide an existing surface", f.Key)
		}
		if _, ok := Lookup(f.Key); !ok {
			t.Fatalf("Lookup(%q) failed", f.Key)
		}
	}

	for _, key := range Keys() {
		if _, ok := Lookup(key); !ok {
			t.Fatalf("Keys() returned %q, which is not in the registry", key)
		}
	}

	// The constants exist so the router cannot misspell a key; make sure they
	// point at real entries.
	for _, key := range []string{
		KeyProblems, KeySubmissions, KeyContests, KeyRankings, KeyGym, KeyPractice,
		KeyVirtualContests, KeyIDE, KeyRecommendations, KeyBlog, KeyEditorials,
		KeyGroups, KeyTeams, KeyOrganizations, KeyClasses, KeyTraining, KeyHacks,
		KeyPlagiarism, KeyNotifications, KeyAPIKeys, KeyWebhooks, KeyMediaUploads,
		KeyVJudge, KeySetterWorkspace, KeyAIGeneration,
	} {
		if _, ok := Lookup(key); !ok {
			t.Fatalf("constant %q is not in the registry", key)
		}
	}
}

func TestUnknownKeyIsNeverGated(t *testing.T) {
	// A typo in a route must not lock a surface out. The registry lookup is
	// what enforces that, so the assertion belongs on the resolver: Decide is
	// a pure function and evaluates whatever feature it is handed.
	store := newFakeStore()
	r := NewResolver(store, nil)

	if got := r.Policy(context.Background(), "nope"); !Equal(got, Unrestricted) {
		t.Fatalf("policy for an unknown key = %+v, want unrestricted", got)
	}
	if d := r.Decide(context.Background(), "nope", Principal{}); !d.Allowed {
		t.Fatalf("decision for an unknown key = %+v, want allowed", d)
	}
}

// --- resolver -------------------------------------------------------------

type fakeStore struct {
	items     map[string]model.SystemSetting
	listCalls int
	err       error
}

func newFakeStore() *fakeStore {
	return &fakeStore{items: make(map[string]model.SystemSetting)}
}

func (s *fakeStore) List(context.Context) ([]model.SystemSetting, error) {
	s.listCalls++
	if s.err != nil {
		return nil, s.err
	}
	out := make([]model.SystemSetting, 0, len(s.items))
	for _, v := range s.items {
		out = append(out, v)
	}
	return out, nil
}

func (s *fakeStore) Get(_ context.Context, key string) (*model.SystemSetting, error) {
	if v, ok := s.items[key]; ok {
		return &v, nil
	}
	return nil, nil
}

func (s *fakeStore) Set(_ context.Context, setting *model.SystemSetting) error {
	s.items[setting.Key] = *setting
	return nil
}

func (s *fakeStore) Delete(_ context.Context, key string) error {
	delete(s.items, key)
	return nil
}

func (s *fakeStore) put(t *testing.T, key string, p Policy) {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s.items[SettingKey(key)] = model.SystemSetting{Key: SettingKey(key), Value: raw}
}

type fakeProfiles struct {
	ratings map[string]int
	calls   int
}

func (p *fakeProfiles) GetProfile(_ context.Context, userID string) (*model.UserProfile, error) {
	p.calls++
	if r, ok := p.ratings[userID]; ok {
		return &model.UserProfile{UserID: userID, Rating: r}, nil
	}
	return nil, errors.New("not found")
}

func TestResolverUsesDefaultsUntilAnAdminSaves(t *testing.T) {
	store := newFakeStore()
	r := NewResolver(store, nil)

	if got := r.Policy(context.Background(), KeyGym); !got.Enabled {
		t.Fatalf("default policy must be enabled, got %+v", got)
	}

	store.put(t, KeyGym, Policy{Enabled: false})
	r.Invalidate()

	if got := r.Policy(context.Background(), KeyGym); got.Enabled {
		t.Fatalf("stored policy must win, got %+v", got)
	}
	if got := r.Policy(context.Background(), KeyBlog); !got.Enabled {
		t.Fatal("an untouched feature must keep its default")
	}
}

func TestResolverCachesUntilInvalidated(t *testing.T) {
	store := newFakeStore()
	r := NewResolver(store, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = r.Policy(ctx, KeyGym)
	}
	if store.listCalls != 1 {
		t.Fatalf("List called %d times, want 1 (policies are cached)", store.listCalls)
	}

	r.Invalidate()
	_ = r.Policy(ctx, KeyGym)
	if store.listCalls != 2 {
		t.Fatalf("List called %d times after invalidate, want 2", store.listCalls)
	}
}

func TestResolverReloadsAfterTTL(t *testing.T) {
	store := newFakeStore()
	r := NewResolver(store, nil)
	ctx := context.Background()

	now := time.Unix(1_700_000_000, 0)
	r.now = func() time.Time { return now }
	r.SetTTL(time.Minute)

	_ = r.Policy(ctx, KeyGym)
	if store.listCalls != 1 {
		t.Fatalf("List called %d times, want 1", store.listCalls)
	}

	now = now.Add(30 * time.Second)
	_ = r.Policy(ctx, KeyGym)
	if store.listCalls != 1 {
		t.Fatalf("List called %d times inside the TTL, want 1", store.listCalls)
	}

	now = now.Add(31 * time.Second)
	_ = r.Policy(ctx, KeyGym)
	if store.listCalls != 2 {
		t.Fatalf("List called %d times after the TTL, want 2", store.listCalls)
	}
}

func TestResolverFallsBackToDefaultsOnStoreError(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("db down")
	r := NewResolver(store, nil)

	// Availability wins: a settings outage must not 403 the whole site.
	if got := r.Policy(context.Background(), KeyGym); !got.Enabled {
		t.Fatalf("policy = %+v, want the enabled default", got)
	}
}

func TestResolverIgnoresMalformedRows(t *testing.T) {
	store := newFakeStore()
	store.items[SettingKey(KeyGym)] = model.SystemSetting{Key: SettingKey(KeyGym), Value: json.RawMessage(`{"enabled":`)}
	store.items["feature.not_a_feature"] = model.SystemSetting{Key: "feature.not_a_feature", Value: json.RawMessage(`{"enabled":false}`)}
	r := NewResolver(store, nil)

	if got := r.Policy(context.Background(), KeyGym); !got.Enabled {
		t.Fatalf("malformed row must be ignored, got %+v", got)
	}
	if _, err := r.Snapshot(context.Background(), Principal{}); err != nil {
		t.Fatalf("a malformed row must not fail the snapshot: %v", err)
	}
}

func TestResolverOnlyLooksUpRatingsWhenNeeded(t *testing.T) {
	store := newFakeStore()
	profiles := &fakeProfiles{ratings: map[string]int{"u": 1500}}
	r := NewResolver(store, profiles)
	ctx := context.Background()
	user := Principal{UserID: "u", Role: model.RoleUser}

	// No rating bound: the hot path must not touch user_profiles.
	if d := r.Decide(ctx, KeyProblems, user); !d.Allowed {
		t.Fatalf("expected allowed, got %+v", d)
	}
	if profiles.calls != 0 {
		t.Fatalf("profile lookups = %d, want 0 for an unbounded policy", profiles.calls)
	}

	store.put(t, KeyGym, Policy{Enabled: true, MinRating: 1400})
	r.Invalidate()
	if d := r.Decide(ctx, KeyGym, user); !d.Allowed {
		t.Fatalf("user rating 1500 must clear a 1400 floor, got %+v", d)
	}
	if profiles.calls != 1 {
		t.Fatalf("profile lookups = %d, want 1", profiles.calls)
	}

	store.put(t, KeyGym, Policy{Enabled: true, MinRating: 1600})
	r.Invalidate()
	if d := r.Decide(ctx, KeyGym, user); d.Allowed || d.Reason != ReasonRating {
		t.Fatalf("decision = %+v, want a rating denial", d)
	}
}

func TestResolverTreatsLookupFailureAsUnrated(t *testing.T) {
	store := newFakeStore()
	store.put(t, KeyGym, Policy{Enabled: true, MinRating: 1200})
	// No ratings recorded, so GetProfile errors.
	r := NewResolver(store, &fakeProfiles{ratings: map[string]int{}})

	d := r.Decide(context.Background(), KeyGym, Principal{UserID: "ghost", Role: model.RoleUser})
	if d.Allowed || d.Reason != ReasonRating {
		t.Fatalf("decision = %+v, want a rating denial when the rating is unknown", d)
	}
}

func TestSnapshotCoversEveryFeature(t *testing.T) {
	store := newFakeStore()
	store.put(t, KeyBlog, Policy{Enabled: false})
	r := NewResolver(store, nil)

	got, err := r.Snapshot(context.Background(), Principal{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(All()) {
		t.Fatalf("snapshot has %d entries, want %d", len(got), len(All()))
	}

	byKey := make(map[string]Resolved, len(got))
	for _, r := range got {
		byKey[r.Key] = r
	}
	if byKey[KeyBlog].Allowed {
		t.Fatal("a disabled feature must be reported as denied")
	}
	if byKey[KeyBlog].Reason != ReasonDisabled {
		t.Fatalf("reason = %q, want %q", byKey[KeyBlog].Reason, ReasonDisabled)
	}
	if !byKey[KeyGym].Allowed {
		t.Fatal("an untouched feature must be reported as allowed")
	}
}
