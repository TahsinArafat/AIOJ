package authmethod

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// fakeStore is an in-memory SettingsStore. A nil row means "no admin decision
// has ever been saved", which is the state of a fresh deployment.
type fakeStore struct {
	row      *model.SystemSetting
	getErr   error
	setCalls int
}

func (f *fakeStore) Get(_ context.Context, key string) (*model.SystemSetting, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.row == nil || f.row.Key != key {
		return nil, nil
	}
	return f.row, nil
}

func (f *fakeStore) Set(_ context.Context, setting *model.SystemSetting) error {
	f.setCalls++
	cp := *setting
	f.row = &cp
	return nil
}

func (f *fakeStore) Delete(_ context.Context, _ string) error {
	f.row = nil
	return nil
}

func storeWith(t *testing.T, raw string) *fakeStore {
	t.Helper()
	return &fakeStore{row: &model.SystemSetting{Key: SettingKey, Value: json.RawMessage(raw)}}
}

func TestDefaultLeavesEveryMethodOn(t *testing.T) {
	d := Default()
	if !d.PasswordRegistration || !d.Google || !d.GitHub {
		t.Fatalf("default must leave every method on, got %+v", d)
	}
}

func TestAvailabilityIntersectsSwitchAndCredentials(t *testing.T) {
	tests := []struct {
		name       string
		stored     string
		configured Configured
		want       Availability
	}{
		{
			name:       "no stored setting uses the defaults",
			configured: Configured{Google: true, GitHub: true},
			want:       Availability{PasswordRegistration: true, Google: true, GitHub: true},
		},
		{
			name:   "a provider switched on without credentials stays hidden",
			stored: `{"password_registration":true,"google":true}`,
			// The deployment never set GOOGLE_CLIENT_ID. Offering the button
			// would send the user to a provider error page.
			configured: Configured{Google: false, GitHub: true},
			want:       Availability{PasswordRegistration: true, Google: false, GitHub: true},
		},
		{
			name:       "an admin can switch a configured provider off",
			stored:     `{"password_registration":true,"google":false,"github":true}`,
			configured: Configured{Google: true, GitHub: true},
			want:       Availability{PasswordRegistration: true, Google: false, GitHub: true},
		},
		{
			name:       "password registration can be closed",
			stored:     `{"password_registration":false,"google":true,"github":true}`,
			configured: Configured{Google: true, GitHub: true},
			want:       Availability{PasswordRegistration: false, Google: true, GitHub: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var store SettingsStore = &fakeStore{}
			if tc.stored != "" {
				store = storeWith(t, tc.stored)
			}
			r := NewResolver(store, tc.configured)
			if got := r.Availability(context.Background()); got != tc.want {
				t.Fatalf("availability = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestPartialStoredRowKeepsOtherMethodsOn(t *testing.T) {
	// A row that names only one method must not switch the others off. This is
	// the failure mode that makes pointer fields worth the trouble: decoding
	// `{"google":false}` into a plain struct would set password_registration
	// to false as well and silently close sign-ups.
	r := NewResolver(storeWith(t, `{"google":false}`), Configured{Google: true, GitHub: true})

	s := r.Settings(context.Background())
	if !s.PasswordRegistration || !s.GitHub {
		t.Fatalf("unspecified methods must keep their default (on), got %+v", s)
	}
	if s.Google {
		t.Fatal("google was explicitly switched off")
	}
}

func TestMalformedSettingsFallBackToDefaults(t *testing.T) {
	r := NewResolver(storeWith(t, `{not json`), Configured{Google: true, GitHub: true})
	if got := r.Settings(context.Background()); got != Default() {
		t.Fatalf("malformed row must fall back to defaults, got %+v", got)
	}
}

func TestStoreFailureFallsBackToDefaults(t *testing.T) {
	// An unreachable settings table must not lock everyone out of signing in.
	r := NewResolver(&fakeStore{getErr: errors.New("db down")}, Configured{Google: true})
	got := r.Availability(context.Background())
	if !got.PasswordRegistration || !got.Google {
		t.Fatalf("store failure must not close a method, got %+v", got)
	}
}

func TestAvailableRejectsUnknownMethods(t *testing.T) {
	r := NewResolver(&fakeStore{}, Configured{Google: true, GitHub: true})
	if r.Available(context.Background(), "facebook") {
		t.Fatal("an unknown provider must never be available")
	}
	if !r.Available(context.Background(), KeyPasswordRegistration) {
		t.Fatal("password registration is on by default")
	}
}

func TestSavePersistsAndInvalidatesCache(t *testing.T) {
	store := &fakeStore{}
	r := NewResolver(store, Configured{Google: true})
	ctx := context.Background()

	// Prime the cache with the defaults.
	if !r.Available(ctx, KeyGoogle) {
		t.Fatal("google should start available")
	}

	if err := r.Save(ctx, Settings{PasswordRegistration: true, Google: false, GitHub: true}, nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	if store.setCalls != 1 {
		t.Fatalf("expected one write, got %d", store.setCalls)
	}

	// The cached value must be dropped, not merely written: the same process
	// serves the next request.
	if r.Available(ctx, KeyGoogle) {
		t.Fatal("save must invalidate the cache immediately")
	}
}

func TestSettingsCacheRespectsTTL(t *testing.T) {
	store := &fakeStore{}
	r := NewResolver(store, Configured{Google: true, GitHub: true})
	ctx := context.Background()

	now := time.Now()
	r.now = func() time.Time { return now }
	r.SetTTL(10 * time.Second)

	if !r.Settings(ctx).Google {
		t.Fatal("expected google on by default")
	}

	// Write behind the resolver's back, as another replica would.
	store.row = &model.SystemSetting{Key: SettingKey, Value: json.RawMessage(`{"google":false}`)}

	if !r.Settings(ctx).Google {
		t.Fatal("the cached value should still be in force inside the TTL")
	}

	now = now.Add(11 * time.Second)
	if r.Settings(ctx).Google {
		t.Fatal("the cache should have expired after the TTL")
	}
}

func TestAdminViewReportsCustomized(t *testing.T) {
	r := NewResolver(&fakeStore{}, Configured{Google: false})
	view := r.AdminView(context.Background())
	if view.Customized {
		t.Fatal("no stored row means not customized")
	}
	if view.Configured.Google {
		t.Fatal("configured must mirror the deployment, not the switch")
	}

	custom := NewResolver(storeWith(t, `{"google":false}`), Configured{Google: true})
	if !custom.AdminView(context.Background()).Customized {
		t.Fatal("a stored row differing from the default is customized")
	}
}
