package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/testdb"
)

func testCredentialBox(t *testing.T) *ai.CredentialBox {
	t.Helper()
	box, err := ai.NewCredentialBox([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("credential box: %v", err)
	}
	return box
}

func newTestBot(t *testing.T, db *sql.DB, userID string, secrets bool) *model.BotAccount {
	t.Helper()
	suffix := uuid.New().String()[:8]
	ba := &model.BotAccount{
		UserID:       userID,
		Platform:     "codeforces",
		PlatformUser: "bot_" + suffix,
		Status:       "active",
		RateLimitRPS: 1,
	}
	if secrets {
		ba.PlatformPass = "hunter2-" + suffix
		ba.APIKey = "api-key-" + suffix
		ba.APISecret = "api-secret-" + suffix
		ba.SessionData = map[string]string{"cf_clearance": "cookie-" + suffix}
	}
	return ba
}

// TestBotAccountCredentialsAreEncryptedAtRest is the regression test for the
// defect this change fixes: a bot account used to keep a working third-party
// login and API keys in plaintext columns, so a backup or a read replica handed
// them over. The raw row must be ciphertext; the store must hand the caller the
// plaintext back.
func TestBotAccountCredentialsAreEncryptedAtRest(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	user := newAdminTestUser(t, NewUserStore(db), "user")
	store := NewBotAccountStore(db, testCredentialBox(t))

	ba := newTestBot(t, db, user.ID, true)
	if err := store.Create(ctx, ba); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM bot_accounts WHERE id = $1", ba.ID)
	})

	var pass, key, secret, session string
	if err := db.QueryRowContext(ctx,
		`SELECT platform_pass, api_key, api_secret, session_data::text FROM bot_accounts WHERE id=$1`,
		ba.ID).Scan(&pass, &key, &secret, &session); err != nil {
		t.Fatalf("read raw row: %v", err)
	}
	for name, tc := range map[string]struct{ got, plain string }{
		"platform_pass": {pass, "hunter2-"},
		"api_key":       {key, "api-key-"},
		"api_secret":    {secret, "api-secret-"},
	} {
		if !strings.HasPrefix(tc.got, "enc:v1:") {
			t.Errorf("%s is not encrypted at rest: %q", name, tc.got)
		}
		if strings.Contains(tc.got, tc.plain) {
			t.Errorf("%s still contains the plaintext", name)
		}
	}
	// session_data is JSONB, so ciphertext travels as a JSON string.
	if !strings.HasPrefix(session, `"enc:v1:`) {
		t.Errorf("session_data is not encrypted at rest: %q", session)
	}
	if strings.Contains(session, "cf_clearance") {
		t.Errorf("session_data still contains the cookie name: %q", session)
	}

	got, err := store.GetByID(ctx, ba.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PlatformPass != ba.PlatformPass || got.APIKey != ba.APIKey || got.APISecret != ba.APISecret {
		t.Errorf("secrets did not round-trip: got %+v", got)
	}
	if got.SessionData["cf_clearance"] != ba.SessionData["cf_clearance"] {
		t.Errorf("session_data did not round-trip: %v", got.SessionData)
	}

	// The path the vjudge pollers actually use must decrypt too.
	list, err := store.ListByPlatform(ctx, ba.Platform)
	if err != nil {
		t.Fatalf("list by platform: %v", err)
	}
	found := false
	for _, item := range list {
		if item.ID != ba.ID {
			continue
		}
		found = true
		if item.PlatformPass != ba.PlatformPass || item.SessionData["cf_clearance"] != ba.SessionData["cf_clearance"] {
			t.Errorf("ListByPlatform returned the wrong values: %+v", item)
		}
	}
	if !found {
		t.Fatal("ListByPlatform did not return the account")
	}
}

// TestBotAccountLegacyPlaintextIsMigrated covers the rows written before this
// change. The backfill must encrypt them in place, be safe to run twice, and
// leave reads working throughout.
func TestBotAccountLegacyPlaintextIsMigrated(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	user := newAdminTestUser(t, NewUserStore(db), "user")

	ba := newTestBot(t, db, user.ID, false)
	if err := db.QueryRowContext(ctx,
		`INSERT INTO bot_accounts (user_id, platform, platform_user, platform_pass, api_key, api_secret, session_data, status, rate_limit_rps)
		 VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,'active',1) RETURNING id`,
		user.ID, ba.Platform, ba.PlatformUser, "legacy-pass", "legacy-key", "legacy-secret",
		`{"cf_clearance":"legacy-cookie"}`).Scan(&ba.ID); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM bot_accounts WHERE id = $1", ba.ID)
	})

	store := NewBotAccountStore(db, testCredentialBox(t))
	if err := store.EncryptLegacyCredentials(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var pass, session string
	if err := db.QueryRowContext(ctx,
		`SELECT platform_pass, session_data::text FROM bot_accounts WHERE id=$1`, ba.ID,
	).Scan(&pass, &session); err != nil {
		t.Fatalf("read raw row: %v", err)
	}
	if !strings.HasPrefix(pass, "enc:v1:") || pass == "legacy-pass" {
		t.Errorf("platform_pass was not migrated: %q", pass)
	}
	if !strings.HasPrefix(session, `"enc:v1:`) {
		t.Errorf("session_data was not migrated: %q", session)
	}

	got, err := store.GetByID(ctx, ba.ID)
	if err != nil {
		t.Fatalf("get after migrate: %v", err)
	}
	if got.PlatformPass != "legacy-pass" || got.APIKey != "legacy-key" ||
		got.APISecret != "legacy-secret" || got.SessionData["cf_clearance"] != "legacy-cookie" {
		t.Errorf("migrated row did not decrypt to the original values: %+v", got)
	}

	// A second startup must be a no-op, not a double-encryption.
	if err := store.EncryptLegacyCredentials(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var after string
	if err := db.QueryRowContext(ctx, `SELECT platform_pass FROM bot_accounts WHERE id=$1`, ba.ID).Scan(&after); err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if after != pass {
		t.Errorf("second migration changed the ciphertext: %q -> %q", pass, after)
	}
}

// TestBotAccountWriteWithoutKeyFailsClosed pins the deployment contract: a
// secret cannot be stored when no key is configured. An account with no secrets
// must still save, or an empty bot would be impossible to create.
func TestBotAccountWriteWithoutKeyFailsClosed(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	user := newAdminTestUser(t, NewUserStore(db), "user")
	store := NewBotAccountStore(db) // deliberately no box

	withSecret := newTestBot(t, db, user.ID, true)
	if err := store.Create(ctx, withSecret); !errors.Is(err, ai.ErrEncryptionNotConfigured) {
		t.Fatalf("expected ErrEncryptionNotConfigured, got %v", err)
	}

	withoutSecret := newTestBot(t, db, user.ID, false)
	withoutSecret.PlatformUser = "nos_" + uuid.New().String()[:8]
	if err := store.Create(ctx, withoutSecret); err != nil {
		t.Fatalf("an account with no secrets should save without a key: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM bot_accounts WHERE id = $1", withoutSecret.ID)
	})
}

// TestBotAccountWrongKeyFailsLoudly makes sure a rotated key is an error, not a
// silent empty password that the pollers would present to the remote judge.
func TestBotAccountWrongKeyFailsLoudly(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	user := newAdminTestUser(t, NewUserStore(db), "user")

	ba := newTestBot(t, db, user.ID, true)
	if err := NewBotAccountStore(db, testCredentialBox(t)).Create(ctx, ba); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM bot_accounts WHERE id = $1", ba.ID)
	})

	other, err := ai.NewCredentialBox([]byte("ffffffffffffffffffffffffffffffff"))
	if err != nil {
		t.Fatalf("second box: %v", err)
	}
	if _, err := NewBotAccountStore(db, other).GetByID(ctx, ba.ID); err == nil {
		t.Fatal("expected a decryption error with the wrong key, got nil")
	}
}

// TestBotAccountUpdateReEncrypts covers the write path the admin console uses
// most. Create was tested; Update was not, and it is the one that would
// silently overwrite ciphertext with plaintext — a rotated password would land
// in the clear while every read continued to work.
func TestBotAccountUpdateReEncrypts(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	user := newAdminTestUser(t, NewUserStore(db), "user")
	store := NewBotAccountStore(db, testCredentialBox(t))

	ba := newTestBot(t, db, user.ID, true)
	if err := store.Create(ctx, ba); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM bot_accounts WHERE id = $1", ba.ID)
	})

	// Rotate every credential through Update, the way the console does.
	ba.PlatformPass = "rotated-pass"
	ba.APIKey = "rotated-key"
	ba.APISecret = "rotated-secret"
	ba.SessionData = map[string]string{"cf_clearance": "rotated-cookie"}
	if err := store.Update(ctx, ba.ID, ba); err != nil {
		t.Fatalf("update: %v", err)
	}

	var pass, key, secret, session string
	if err := db.QueryRowContext(ctx,
		`SELECT platform_pass, api_key, api_secret, session_data::text FROM bot_accounts WHERE id=$1`,
		ba.ID).Scan(&pass, &key, &secret, &session); err != nil {
		t.Fatalf("read raw row: %v", err)
	}
	for name, got := range map[string]string{
		"platform_pass": pass, "api_key": key, "api_secret": secret,
	} {
		if !strings.HasPrefix(got, "enc:v1:") {
			t.Errorf("%s was written back as plaintext by Update: %q", name, got)
		}
	}
	if strings.Contains(pass, "rotated-pass") || strings.Contains(key, "rotated-key") {
		t.Error("Update wrote the rotated plaintext into the column")
	}
	if !strings.HasPrefix(session, `"enc:v1:`) {
		t.Errorf("session_data was written back unencrypted by Update: %q", session)
	}

	got, err := store.GetByID(ctx, ba.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.PlatformPass != "rotated-pass" || got.APIKey != "rotated-key" ||
		got.APISecret != "rotated-secret" || got.SessionData["cf_clearance"] != "rotated-cookie" {
		t.Errorf("rotated credentials did not round-trip: %+v", got)
	}
}

// TestBotAccountPresenceFlags pins what the admin console is allowed to know:
// that a secret exists, never its value. The flags are derived in the store so
// no handler has to read a secret to answer the question.
func TestBotAccountPresenceFlags(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	user := newAdminTestUser(t, NewUserStore(db), "user")
	store := NewBotAccountStore(db, testCredentialBox(t))

	withSecrets := newTestBot(t, db, user.ID, true)
	if err := store.Create(ctx, withSecrets); err != nil {
		t.Fatalf("create with secrets: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM bot_accounts WHERE id = $1", withSecrets.ID)
	})

	got, err := store.GetByID(ctx, withSecrets.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.HasAPIKey || !got.HasAPISecret {
		t.Errorf("presence flags not set for an account with secrets: %+v", got)
	}

	bare := newTestBot(t, db, user.ID, false)
	bare.PlatformUser = "bare_" + uuid.New().String()[:8]
	if err := store.Create(ctx, bare); err != nil {
		t.Fatalf("create without secrets: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM bot_accounts WHERE id = $1", bare.ID)
	})
	gotBare, err := store.GetByID(ctx, bare.ID)
	if err != nil {
		t.Fatalf("get bare: %v", err)
	}
	if gotBare.HasAPIKey || gotBare.HasAPISecret {
		t.Errorf("presence flags set for an account with no secrets: %+v", gotBare)
	}
}
