package feature

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// SettingsStore is the slice of the system-settings store the resolver needs.
// Declared here (rather than importing the concrete Postgres store) so tests
// can drive the resolver without a database.
type SettingsStore interface {
	List(ctx context.Context) ([]model.SystemSetting, error)
	Get(ctx context.Context, key string) (*model.SystemSetting, error)
	Set(ctx context.Context, setting *model.SystemSetting) error
	Delete(ctx context.Context, key string) error
}

// ProfileLookup supplies a user's rating for rating-bounded policies.
type ProfileLookup interface {
	GetProfile(ctx context.Context, userID string) (*model.UserProfile, error)
}

// SettingKeyPrefix namespaces feature policies inside `system_settings`.
// The stored key is SettingKeyPrefix + Feature.Key, e.g. "feature.gym".
const SettingKeyPrefix = "feature."

// SettingKey returns the system_settings key holding key's policy.
func SettingKey(key string) string { return SettingKeyPrefix + key }

// DefaultTTL is how long a loaded policy set is reused.
//
// It bounds how stale a policy can be. Writes call Invalidate() on the
// instance that served them, so a single-replica deployment is immediate;
// with several replicas this is the worst-case propagation delay.
const DefaultTTL = 15 * time.Second

// Resolver answers "may this principal use this feature?".
//
// It caches the stored policies (not the decisions: decisions depend on the
// caller) and reads a rating only when a policy actually bounds ratings, so
// the unconfigured hot paths cost nothing beyond the cache lookup.
type Resolver struct {
	store    SettingsStore
	profiles ProfileLookup
	ttl      time.Duration
	now      func() time.Time

	mu       sync.RWMutex
	policies map[string]Policy
	loadedAt time.Time
	loaded   bool
}

// NewResolver builds a resolver. profiles may be nil, in which case
// rating-bounded policies treat every non-admin rating as 0.
func NewResolver(store SettingsStore, profiles ProfileLookup) *Resolver {
	return &Resolver{
		store:    store,
		profiles: profiles,
		ttl:      DefaultTTL,
		now:      time.Now,
	}
}

// SetTTL overrides the cache lifetime. Tests use it to make staleness
// deterministic; production keeps DefaultTTL.
func (r *Resolver) SetTTL(ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ttl = ttl
	r.loaded = false
}

// Invalidate drops the cached policies so the next read reloads them. Called
// after an admin write so the change takes effect on this instance at once.
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded = false
	r.policies = nil
}

// Policy returns the policy in force for key.
//
// A load failure falls back to the registry default rather than denying
// everything: an unreachable settings table must not take the whole site down,
// and the defaults are the "everything on" baseline an operator started from.
func (r *Resolver) Policy(ctx context.Context, key string) Policy {
	f, ok := Lookup(key)
	if !ok {
		// Unknown keys are never gated; a typo in a route must not lock a
		// surface out silently.
		return Unrestricted
	}
	policies, err := r.load(ctx)
	if err != nil {
		slog.Warn("feature: policy load failed, using defaults", "error", err)
		return Normalize(f.Default)
	}
	if p, ok := policies[key]; ok {
		return Normalize(p)
	}
	return Normalize(f.Default)
}

// Policies returns every stored policy, keyed by feature key. Missing entries
// mean "use the registry default".
func (r *Resolver) Policies(ctx context.Context) (map[string]Policy, error) {
	return r.load(ctx)
}

// Decide evaluates the policy for key against principal.
func (r *Resolver) Decide(ctx context.Context, key string, principal Principal) Decision {
	f, ok := Lookup(key)
	if !ok {
		return Allowed
	}
	policy := r.Policy(ctx, key)

	if NeedsRating(policy) && !principal.Anonymous && principal.UserID != "" {
		principal.Rating = r.rating(ctx, principal.UserID)
	}
	return Decide(f, policy, principal)
}

// Resolved is a feature together with the decision for one principal. This is
// the shape served by GET /api/features and consumed by the SPA.
type Resolved struct {
	Feature
	// Policy is the effective policy, so the UI can explain a denial
	// ("needs 1200 rating") without a second request.
	Policy  Policy `json:"policy"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Snapshot resolves every feature for principal.
func (r *Resolver) Snapshot(ctx context.Context, principal Principal) ([]Resolved, error) {
	policies, err := r.load(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Resolved, 0, len(registry))
	for _, f := range registry {
		policy := f.Default
		if p, ok := policies[f.Key]; ok {
			policy = p
		}
		policy = Normalize(policy)

		p := principal
		if NeedsRating(policy) && !p.Anonymous && p.UserID != "" {
			p.Rating = r.rating(ctx, p.UserID)
		}
		d := Decide(f, policy, p)

		out = append(out, Resolved{
			Feature: f,
			Policy:  policy,
			Allowed: d.Allowed,
			Reason:  d.Reason,
			Message: d.Message,
		})
	}
	return out, nil
}

// rating reads the caller's rating, treating any failure as "unrated".
//
// Failing to 0 is the conservative choice: an unrated user is below every
// minimum, so a rating gate stays closed rather than opening because a query
// failed.
func (r *Resolver) rating(ctx context.Context, userID string) int {
	if r.profiles == nil {
		return 0
	}
	profile, err := r.profiles.GetProfile(ctx, userID)
	if err != nil || profile == nil {
		if err != nil {
			slog.Warn("feature: rating lookup failed", "user", userID, "error", err)
		}
		return 0
	}
	return profile.Rating
}

// load returns the cached policy map, reloading it when stale.
func (r *Resolver) load(ctx context.Context) (map[string]Policy, error) {
	r.mu.RLock()
	if r.loaded && r.now().Sub(r.loadedAt) < r.ttl {
		policies := r.policies
		r.mu.RUnlock()
		return policies, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// Another goroutine may have refreshed while we waited for the write lock.
	if r.loaded && r.now().Sub(r.loadedAt) < r.ttl {
		return r.policies, nil
	}

	settings, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}

	policies := make(map[string]Policy, len(registry))
	for _, s := range settings {
		if !strings.HasPrefix(s.Key, SettingKeyPrefix) {
			continue
		}
		key := strings.TrimPrefix(s.Key, SettingKeyPrefix)
		if _, ok := Lookup(key); !ok {
			// A row for a feature that no longer exists (renamed or removed).
			// Keep ignoring it rather than erroring the whole snapshot.
			continue
		}
		var p Policy
		if err := json.Unmarshal(s.Value, &p); err != nil {
			slog.Warn("feature: ignoring malformed policy", "key", s.Key, "error", err)
			continue
		}
		policies[key] = Normalize(p)
	}

	r.policies = policies
	r.loadedAt = r.now()
	r.loaded = true
	return policies, nil
}
