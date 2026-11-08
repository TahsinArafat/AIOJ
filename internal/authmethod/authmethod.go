// Package authmethod decides which sign-in methods an instance offers.
//
// Two independent facts make a method usable:
//
//  1. Whether the deployment can perform it at all. For email/password that is
//     always true; for an OAuth provider it means an id and secret were
//     configured. This is process state read once at startup, not a setting.
//  2. Whether an admin has left it switched on. That is a stored decision, so
//     it survives restarts and is covered by the admin audit log.
//
// Availability intersects the two. That intersection is the whole point: a
// login page must never render a button the deployment cannot honour, and an
// admin must not be able to advertise a provider the operator never configured.
//
// The package deliberately does not reuse internal/feature. Feature policies
// carry role and rating restrictions, which are meaningless for a sign-in
// method and would be a trap: restricting "google" to a role would deny it to
// every signed-out visitor, which is exactly who needs it.
package authmethod

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// Method keys. These double as the JSON field names for the public API, the
// admin API and the stored setting, so a typo shows up as an empty checkbox
// rather than a silently ignored switch.
const (
	KeyPasswordRegistration = "password_registration"
	KeyGoogle               = "google"
	KeyGitHub               = "github"
)

// SettingKey is the `system_settings` row holding the admin's decisions.
const SettingKey = "auth.methods"

// DefaultTTL bounds how stale a cached decision can be. Writes invalidate the
// cache on the instance that served them, so a single-replica deployment picks
// a change up immediately; this is the worst case with several replicas.
const DefaultTTL = 15 * time.Second

// Settings is the admin's decision, one switch per method.
//
// A method switched on here is still unavailable when the deployment has no
// credentials for it — see Availability.
type Settings struct {
	// PasswordRegistration allows creating an account with an email address
	// and a password. It does not affect signing in, so existing accounts keep
	// working when an operator closes registration.
	PasswordRegistration bool `json:"password_registration"`
	Google               bool `json:"google"`
	GitHub               bool `json:"github"`
}

// Default leaves every method on.
//
// "On" is the right default for both meanings: an operator who has never
// opened this panel should get the behaviour the README describes, and a
// forgotten setting should not silently close a deployment's only way in.
func Default() Settings {
	return Settings{PasswordRegistration: true, Google: true, GitHub: true}
}

// Configured reports whether the deployment holds credentials for each OAuth
// provider. Read once at startup from the environment.
type Configured struct {
	Google bool `json:"google"`
	GitHub bool `json:"github"`
}

// Availability is the public answer: what the UI is allowed to offer.
//
// It is intentionally a flat, boolean-only shape. The login and register pages
// render from it directly, and nothing in it is specific to the caller, so it
// is safe to serve anonymously.
type Availability struct {
	PasswordRegistration bool `json:"password_registration"`
	Google               bool `json:"google"`
	GitHub               bool `json:"github"`
}

// AdminView is what the admin panel edits. It carries the raw provider state
// so the panel can explain "switched on, but no credentials" instead of
// showing a switch that appears to do nothing.
type AdminView struct {
	Settings   Settings   `json:"settings"`
	Configured Configured `json:"configured"`
	Default    Settings   `json:"default"`
	// Customized reports whether a stored row differs from Default.
	Customized bool `json:"customized"`
}

// SettingsStore is the slice of the system-settings store this package needs.
// Declared here so tests can drive the resolver without a database.
type SettingsStore interface {
	Get(ctx context.Context, key string) (*model.SystemSetting, error)
	Set(ctx context.Context, setting *model.SystemSetting) error
	Delete(ctx context.Context, key string) error
}

// stored is the wire shape of the persisted row.
//
// The fields are pointers so a partial or hand-edited row cannot silently turn
// a method off: a missing field keeps the default. Without this, persisting
// `{"google":false}` would read back as password_registration=false too, which
// would close registration as a side effect of disabling Google.
type stored struct {
	PasswordRegistration *bool `json:"password_registration"`
	Google               *bool `json:"google"`
	GitHub               *bool `json:"github"`
}

func (s stored) resolve() Settings {
	out := Default()
	if s.PasswordRegistration != nil {
		out.PasswordRegistration = *s.PasswordRegistration
	}
	if s.Google != nil {
		out.Google = *s.Google
	}
	if s.GitHub != nil {
		out.GitHub = *s.GitHub
	}
	return out
}

// Resolver answers "may this sign-in method be used right now?".
type Resolver struct {
	store      SettingsStore
	configured Configured
	ttl        time.Duration
	now        func() time.Time

	mu       sync.RWMutex
	settings Settings
	loadedAt time.Time
	loaded   bool
}

// NewResolver builds a resolver. store may be nil in tests that only exercise
// Availability with defaults.
func NewResolver(store SettingsStore, configured Configured) *Resolver {
	return &Resolver{
		store:      store,
		configured: configured,
		ttl:        DefaultTTL,
		now:        time.Now,
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

// Invalidate drops the cached settings so the next read reloads them. Called
// after an admin write so the change takes effect on this instance at once.
func (r *Resolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded = false
}

// Settings returns the admin's decisions.
//
// A load failure falls back to Default rather than denying every method: an
// unreachable settings table must not lock everyone out of their own instance.
func (r *Resolver) Settings(ctx context.Context) Settings {
	if r.store == nil {
		return Default()
	}
	r.mu.RLock()
	if r.loaded && r.now().Sub(r.loadedAt) < r.ttl {
		s := r.settings
		r.mu.RUnlock()
		return s
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded && r.now().Sub(r.loadedAt) < r.ttl {
		return r.settings
	}

	s := Default()
	setting, err := r.store.Get(ctx, SettingKey)
	switch {
	case err != nil:
		slog.Warn("authmethod: settings load failed, using defaults", "error", err)
	case setting != nil:
		var raw stored
		if err := json.Unmarshal(setting.Value, &raw); err != nil {
			slog.Warn("authmethod: ignoring malformed settings", "error", err)
		} else {
			s = raw.resolve()
		}
	}

	r.settings = s
	r.loadedAt = r.now()
	r.loaded = true
	return s
}

// Availability intersects the admin's decisions with what the deployment can
// actually perform.
func (r *Resolver) Availability(ctx context.Context) Availability {
	s := r.Settings(ctx)
	cfg := r.configured
	return Availability{
		PasswordRegistration: s.PasswordRegistration,
		// An OAuth provider needs both: switched on, and credentials present.
		Google: s.Google && cfg.Google,
		GitHub: s.GitHub && cfg.GitHub,
	}
}

// Available reports whether one method may be used right now.
//
// Unknown methods are refused: the caller is a provider name taken from a URL
// path, so anything not on the list must not reach an exchange.
func (r *Resolver) Available(ctx context.Context, method string) bool {
	a := r.Availability(ctx)
	switch method {
	case KeyPasswordRegistration:
		return a.PasswordRegistration
	case KeyGoogle:
		return a.Google
	case KeyGitHub:
		return a.GitHub
	default:
		return false
	}
}

// AdminView returns what the admin panel edits.
func (r *Resolver) AdminView(ctx context.Context) AdminView {
	s := r.Settings(ctx)
	return AdminView{
		Settings:   s,
		Configured: r.configured,
		Default:    Default(),
		Customized: s != Default(),
	}
}

// Save stores the admin's decisions and drops the cache so the next request
// sees them.
func (r *Resolver) Save(ctx context.Context, settings Settings, updatedBy *string) error {
	// Marshal the full struct, so a later read cannot confuse "off" with
	// "absent" (see the stored type).
	value, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if err := r.store.Set(ctx, &model.SystemSetting{
		Key:         SettingKey,
		Value:       value,
		Description: "Sign-in methods: email/password registration and OAuth providers",
		UpdatedBy:   updatedBy,
	}); err != nil {
		return err
	}
	r.Invalidate()
	return nil
}
