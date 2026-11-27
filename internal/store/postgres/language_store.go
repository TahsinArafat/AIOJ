package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/tahsinarafat/aioj/internal/model"
)

// LanguageStore holds the live judge language configs.
//
// The config itself is an opaque JSONB document here: the shape is
// compiler.LangConfig, which the handler validates before writing and the
// judge's loader decodes after reading. Keeping the store unaware of the fields
// means adding one to a language needs no migration and no change in this file.
type LanguageStore struct {
	db *sql.DB
}

func NewLanguageStore(db *sql.DB) *LanguageStore {
	return &LanguageStore{db: db}
}

func (s *LanguageStore) List(ctx context.Context) ([]model.Language, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, config, updated_at, updated_by
		   FROM languages
		  WHERE deleted_at IS NULL
		  ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Language
	for rows.Next() {
		var l model.Language
		if err := rows.Scan(&l.Key, &l.Config, &l.UpdatedAt, &l.UpdatedBy); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	if items == nil {
		items = []model.Language{}
	}
	return items, rows.Err()
}

func (s *LanguageStore) GetByKey(ctx context.Context, key string) (*model.Language, error) {
	var l model.Language
	err := s.db.QueryRowContext(ctx,
		`SELECT key, config, updated_at, updated_by
		   FROM languages
		  WHERE key = $1 AND deleted_at IS NULL`, key).Scan(
		&l.Key, &l.Config, &l.UpdatedAt, &l.UpdatedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// Upsert writes one language, reviving it if it had been soft-deleted.
//
// deleted_at is cleared on conflict on purpose: a row an operator removed is
// gone from the judge, so the only way it comes back is an explicit write of a
// new config, which is exactly what this is. The seed path uses Seed instead and
// deliberately does NOT clear it.
func (s *LanguageStore) Upsert(ctx context.Context, key string, config json.RawMessage, updatedBy string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO languages (key, config, updated_at, updated_by)
		 VALUES ($1, $2, NOW(), NULLIF($3, '')::uuid)
		 ON CONFLICT (key) DO UPDATE
		    SET config = $2, updated_at = NOW(), updated_by = NULLIF($3, '')::uuid,
		        deleted_at = NULL`,
		key, []byte(config), updatedBy)
	return err
}

func (s *LanguageStore) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE languages SET deleted_at = NOW(), updated_at = NOW()
		  WHERE key = $1 AND deleted_at IS NULL`, key)
	return err
}

// Seed inserts the shipped defaults, leaving any existing row untouched.
//
// DO NOTHING rather than DO UPDATE is the property that makes this safe to run
// on every boot: a language an operator renamed or retuned keeps its row. It
// also covers deletion — a soft-deleted row still occupies the primary key, so
// the conflict fires and the language stays gone rather than being resurrected
// by a restart.
func (s *LanguageStore) Seed(ctx context.Context, langs map[string]json.RawMessage) (int, error) {
	if len(langs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	created := 0
	for key, cfg := range langs {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO languages (key, config)
			 VALUES ($1, $2)
			 ON CONFLICT (key) DO NOTHING`,
			key, []byte(cfg))
		if err != nil {
			return 0, err
		}
		if n, err := res.RowsAffected(); err == nil {
			created += int(n)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return created, nil
}

// ListLanguageConfigs satisfies compiler.ConfigSource.
//
// It exists so the judge's loader can take this store without either package
// importing the other: the method is matched structurally, so `postgres` stays
// unaware of `compiler` and `compiler` stays free of the database.
func (s *LanguageStore) ListLanguageConfigs(ctx context.Context) (map[string]json.RawMessage, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(items))
	for _, l := range items {
		out[l.Key] = l.Config
	}
	return out, nil
}
