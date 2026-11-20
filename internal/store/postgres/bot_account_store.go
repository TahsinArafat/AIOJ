package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/model"
)

// BotAccountStore is the PostgreSQL implementation of store.BotAccountStore.
//
// The credential columns are encrypted at rest. A bot account holds a working
// login for a third-party judge (platform_pass), or API keys for one, and those
// used to sit in plaintext next to the rest of the row — a database backup, a
// read replica, or one injection bug handed over live accounts. The same
// AES-256-GCM box that protects AI provider keys protects these, and the
// startup migration rewrites any row written before that was true.
type BotAccountStore struct {
	db          *sql.DB
	credentials *ai.CredentialBox
}

// NewBotAccountStore creates a BotAccountStore backed by db.
//
// The box is variadic so existing call sites (and tests that only exercise
// non-secret columns) keep compiling; pass one to enable encryption.
func NewBotAccountStore(db *sql.DB, boxes ...*ai.CredentialBox) *BotAccountStore {
	var box *ai.CredentialBox
	if len(boxes) > 0 {
		box = boxes[0]
	}
	return &BotAccountStore{db: db, credentials: box}
}

// EncryptLegacyCredentials rewrites rows whose secrets are still plaintext.
//
// It runs at startup, next to the equivalent AI-model migration, so an existing
// deployment is converted once instead of on every read. A row is "legacy" when
// a credential column is non-empty and lacks the enc:v1: prefix; session_data is
// legacy when it is a JSON object rather than the JSON string that holds
// ciphertext.
func (s *BotAccountStore) EncryptLegacyCredentials(ctx context.Context) error {
	// The SELECT and the UPDATE must be one transaction, and the SELECT must
	// lock the rows, or a bot that is edited while the backfill runs can be
	// written back as plaintext by the older of the two writers. FOR UPDATE
	// makes the edit wait for this transaction; a writer that already committed
	// is either seen here as ciphertext (and skipped) or overwrites with
	// ciphertext of its own, because Update encrypts before it writes.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, platform_pass, api_key, api_secret, session_data::text
		FROM bot_accounts
		WHERE (platform_pass <> '' AND platform_pass NOT LIKE 'enc:v1:%')
		   OR (api_key       <> '' AND api_key       NOT LIKE 'enc:v1:%')
		   OR (api_secret    <> '' AND api_secret    NOT LIKE 'enc:v1:%')
		   OR (session_data <> '{}'::jsonb
		       AND session_data <> 'null'::jsonb
		       AND session_data::text NOT LIKE '"enc:v1:%')
		FOR UPDATE`)
	if err != nil {
		return err
	}

	type legacy struct {
		id                         string
		pass, key, secret, session string
	}
	var items []legacy
	for rows.Next() {
		var v legacy
		if err := rows.Scan(&v.id, &v.pass, &v.key, &v.secret, &v.session); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	// Close before the UPDATEs: a *sql.Tx owns a single connection, so issuing
	// another statement while these rows are open deadlocks.
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	if s.credentials == nil {
		return fmt.Errorf("%d bot account(s) still store credentials in plaintext but "+
			"AI_CREDENTIAL_MASTER_KEY is not configured", len(items))
	}

	for _, v := range items {
		pass, err := s.seal(v.pass)
		if err != nil {
			return err
		}
		key, err := s.seal(v.key)
		if err != nil {
			return err
		}
		secret, err := s.seal(v.secret)
		if err != nil {
			return err
		}
		session, err := s.sealSessionJSON(v.session)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE bot_accounts SET platform_pass=$2, api_key=$3, api_secret=$4, session_data=$5::jsonb WHERE id=$1`,
			v.id, pass, key, secret, session); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// seal encrypts a credential for storage. Empty stays empty: a bot with no
// password should not fail to save because no master key is configured.
func (s *BotAccountStore) seal(plain string) (string, error) {
	if plain == "" || ai.IsEncryptedCredential(plain) {
		return plain, nil
	}
	if s.credentials == nil {
		return "", ai.ErrEncryptionNotConfigured
	}
	return s.credentials.Encrypt(plain)
}

// open decrypts a credential read from the database. A legacy plaintext value
// is returned unchanged so an un-migrated row still works; the startup
// migration is what makes that temporary.
func (s *BotAccountStore) open(stored string) (string, error) {
	if stored == "" || !ai.IsEncryptedCredential(stored) {
		return stored, nil
	}
	if s.credentials == nil {
		return "", errors.New("bot credentials are encrypted but AI_CREDENTIAL_MASTER_KEY is not configured")
	}
	return s.credentials.Decrypt(stored)
}

// sealSessionJSON turns a session_data map into the JSONB value to store.
//
// A non-empty map is marshalled, encrypted, and stored as a JSON *string*
// ("enc:v1:…"), because the column is JSONB and cannot hold raw ciphertext.
// nil and {} stay as they are: there is no secret to protect and the column
// default is already '{}'.
func (s *BotAccountStore) sealSessionJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "{}" || strings.HasPrefix(raw, `"enc:v1:`) {
		if raw == "" || raw == "null" {
			return "{}", nil
		}
		return raw, nil
	}
	enc, err := s.seal(raw)
	if err != nil {
		return "", err
	}
	if enc == "" {
		return "{}", nil
	}
	quoted, err := json.Marshal(enc)
	if err != nil {
		return "", err
	}
	return string(quoted), nil
}

// openSessionJSON is the read half of sealSessionJSON. It accepts both the
// encrypted JSON-string form and the legacy JSON-object form.
func (s *BotAccountStore) openSessionJSON(raw []byte) (map[string]string, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return nil, nil
	}
	if raw[0] == '"' {
		var enc string
		if err := json.Unmarshal(raw, &enc); err != nil {
			return nil, fmt.Errorf("session_data: %w", err)
		}
		plain, err := s.open(enc)
		if err != nil {
			return nil, err
		}
		raw = []byte(plain)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("session_data: %w", err)
	}
	return m, nil
}

func (s *BotAccountStore) List(ctx context.Context, offset, limit int) ([]model.BotAccount, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bot_accounts").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, platform, platform_user, status, rate_limit_rps, last_used_at, created_at, proxy_url, proxy_enabled
		 FROM bot_accounts ORDER BY platform, created_at DESC OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.BotAccount
	for rows.Next() {
		var ba model.BotAccount
		if err := rows.Scan(&ba.ID, &ba.Platform, &ba.PlatformUser, &ba.Status, &ba.RateLimitRPS, &ba.LastUsedAt, &ba.CreatedAt, &ba.ProxyURL, &ba.ProxyEnabled); err != nil {
			return nil, 0, err
		}
		items = append(items, ba)
	}
	if items == nil {
		items = []model.BotAccount{}
	}
	return items, total, nil
}

func (s *BotAccountStore) ListByPlatform(ctx context.Context, platform string) ([]model.BotAccount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, platform, platform_user, platform_pass, api_key, api_secret, session_data, status, rate_limit_rps, last_used_at, created_at, proxy_url, proxy_enabled
		 FROM bot_accounts WHERE platform = $1 AND status = 'active' ORDER BY last_used_at ASC NULLS FIRST`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.BotAccount
	for rows.Next() {
		ba, err := s.scanBot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, ba)
	}
	if items == nil {
		items = []model.BotAccount{}
	}
	return items, rows.Err()
}

func (s *BotAccountStore) GetByID(ctx context.Context, id string) (*model.BotAccount, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, platform, platform_user, platform_pass, api_key, api_secret, session_data, status, rate_limit_rps, last_used_at, created_at, proxy_url, proxy_enabled
		 FROM bot_accounts WHERE id = $1`, id)
	ba, err := s.scanBot(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ba, nil
}

// scanner is the subset of *sql.Row and *sql.Rows scanBot needs.
type scanner interface {
	Scan(dest ...any) error
}

// scanBot reads one full row and decrypts its secrets.
func (s *BotAccountStore) scanBot(sc scanner) (model.BotAccount, error) {
	var ba model.BotAccount
	var pass, key, secret string
	var sd []byte
	if err := sc.Scan(&ba.ID, &ba.Platform, &ba.PlatformUser, &pass, &key, &secret, &sd,
		&ba.Status, &ba.RateLimitRPS, &ba.LastUsedAt, &ba.CreatedAt, &ba.ProxyURL, &ba.ProxyEnabled); err != nil {
		return ba, err
	}
	var err error
	if ba.PlatformPass, err = s.open(pass); err != nil {
		return ba, err
	}
	if ba.APIKey, err = s.open(key); err != nil {
		return ba, err
	}
	if ba.APISecret, err = s.open(secret); err != nil {
		return ba, err
	}
	if ba.SessionData, err = s.openSessionJSON(sd); err != nil {
		return ba, err
	}
	return ba, nil
}

func (s *BotAccountStore) Create(ctx context.Context, ba *model.BotAccount) error {
	pass, err := s.seal(ba.PlatformPass)
	if err != nil {
		return err
	}
	key, err := s.seal(ba.APIKey)
	if err != nil {
		return err
	}
	secret, err := s.seal(ba.APISecret)
	if err != nil {
		return err
	}
	sd, err := s.sealSessionMap(ba.SessionData)
	if err != nil {
		return err
	}
	return s.db.QueryRowContext(ctx,
		`INSERT INTO bot_accounts (user_id, platform, platform_user, platform_pass, api_key, api_secret, session_data, status, rate_limit_rps, proxy_url, proxy_enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11)
		 RETURNING id, created_at`,
		ba.UserID, ba.Platform, ba.PlatformUser, pass, key, secret, sd, ba.Status, ba.RateLimitRPS, ba.ProxyURL, ba.ProxyEnabled,
	).Scan(&ba.ID, &ba.CreatedAt)
}

func (s *BotAccountStore) Update(ctx context.Context, id string, ba *model.BotAccount) error {
	pass, err := s.seal(ba.PlatformPass)
	if err != nil {
		return err
	}
	key, err := s.seal(ba.APIKey)
	if err != nil {
		return err
	}
	secret, err := s.seal(ba.APISecret)
	if err != nil {
		return err
	}
	sd, err := s.sealSessionMap(ba.SessionData)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET platform=$1, platform_user=$2, platform_pass=$3, api_key=$4, api_secret=$5, session_data=$6::jsonb, status=$7, rate_limit_rps=$8, proxy_url=$9, proxy_enabled=$10 WHERE id=$11`,
		ba.Platform, ba.PlatformUser, pass, key, secret, sd, ba.Status, ba.RateLimitRPS, ba.ProxyURL, ba.ProxyEnabled, id)
	return err
}

// sealSessionMap marshals and encrypts a session map for storage.
func (s *BotAccountStore) sealSessionMap(m map[string]string) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return s.sealSessionJSON(string(raw))
}

func (s *BotAccountStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bot_accounts WHERE id=$1`, id)
	return err
}

func (s *BotAccountStore) IncrementFailures(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET consecutive_failures = consecutive_failures + 1, last_error_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *BotAccountStore) ResetFailures(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET consecutive_failures = 0, last_error_at = NULL WHERE id = $1`, id)
	return err
}

func (s *BotAccountStore) MarkUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *BotAccountStore) UpdateLastPoll(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET last_poll_at = NOW() WHERE id = $1`, id)
	return err
}
