package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tahsinarafat/aioj/internal/testdb"
)

/**
 * The registry has to survive a redeploy, and it has to survive a restart
 * without the seeder undoing an operator's work. Those are two different
 * properties and only the first is obvious.
 *
 * The bug this table fixes was that an admin#languages edit lived in the
 * container's writable layer, so the next deploy restored the image's copy. That
 * is a persistence question, and a row answers it.
 *
 * The second property is subtler: the seeder runs on EVERY boot, so if it used
 * ON CONFLICT DO UPDATE it would overwrite every operator edit on every restart —
 * the same data loss, moved from deploy time to restart time. INSERT-only is what
 * makes seeding safe, and it is asserted here because nothing else would notice.
 *
 * Deletion is the case that makes INSERT-only non-trivial. A hard DELETE would
 * free the key, the next boot would find the shipped language "missing", and
 * seed it straight back — an operator's deletion silently reverting. The soft
 * delete keeps the key occupied so the conflict fires, and that is why these
 * tests delete a SHIPPED key rather than a made-up one.
 */

func langDoc(t *testing.T, name, compile string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"key":     "cpp-gpp-64",
		"name":    name,
		"compile": compile,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestLanguageStore_SeedIsInsertOnly(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	store := NewLanguageStore(db)

	const key = "cpp-gpp-64"
	defer db.ExecContext(ctx, "DELETE FROM languages WHERE key = $1", key)
	if _, err := db.ExecContext(ctx, "DELETE FROM languages WHERE key = $1", key); err != nil {
		t.Fatalf("clean: %v", err)
	}

	// First boot seeds the shipped config.
	created, err := store.Seed(ctx, map[string]json.RawMessage{key: langDoc(t, "C++", "/usr/bin/g++ -O2")})
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if created != 1 {
		t.Fatalf("first seed created %d rows, want 1", created)
	}

	// The operator renames it.
	if err := store.Upsert(ctx, key, langDoc(t, "C++ 20 (edited)", "/usr/bin/g++ -std=c++20"), ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// The next boot seeds again. It must not touch the edit.
	created, err = store.Seed(ctx, map[string]json.RawMessage{key: langDoc(t, "C++", "/usr/bin/g++ -O2")})
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if created != 0 {
		t.Errorf("second seed created %d rows, want 0 — seeding is overwriting existing languages", created)
	}

	row, err := store.GetByKey(ctx, key)
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if row == nil {
		t.Fatal("the language disappeared")
	}
	var cfg map[string]any
	if err := json.Unmarshal(row.Config, &cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg["name"] != "C++ 20 (edited)" {
		t.Errorf("name = %v after a second seed, want the operator's edit preserved", cfg["name"])
	}
}

func TestLanguageStore_SoftDeleteSurvivesReseed(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	store := NewLanguageStore(db)

	const key = "cpp-gpp-64"
	defer db.ExecContext(ctx, "DELETE FROM languages WHERE key = $1", key)
	if _, err := db.ExecContext(ctx, "DELETE FROM languages WHERE key = $1", key); err != nil {
		t.Fatalf("clean: %v", err)
	}

	if _, err := store.Seed(ctx, map[string]json.RawMessage{key: langDoc(t, "C++", "/usr/bin/g++")}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// It must be gone from the judge's view immediately.
	if row, err := store.GetByKey(ctx, key); err != nil {
		t.Fatalf("GetByKey after delete: %v", err)
	} else if row != nil {
		t.Error("a deleted language is still returned by GetByKey")
	}
	items, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, l := range items {
		if l.Key == key {
			t.Error("a deleted language is still returned by List")
		}
	}

	// A restart must not resurrect it. This is the property a hard DELETE would
	// break: the key would be free, so the seeder would happily re-create it.
	if _, err := store.Seed(ctx, map[string]json.RawMessage{key: langDoc(t, "C++", "/usr/bin/g++")}); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	if row, err := store.GetByKey(ctx, key); err != nil {
		t.Fatalf("GetByKey after reseed: %v", err)
	} else if row != nil {
		t.Error("a restart resurrected a deleted language")
	}
}

func TestLanguageStore_UpsertRevivesADeletedLanguage(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	store := NewLanguageStore(db)

	const key = "cpp-gpp-64"
	defer db.ExecContext(ctx, "DELETE FROM languages WHERE key = $1", key)
	if _, err := db.ExecContext(ctx, "DELETE FROM languages WHERE key = $1", key); err != nil {
		t.Fatalf("clean: %v", err)
	}

	if _, err := store.Seed(ctx, map[string]json.RawMessage{key: langDoc(t, "C++", "/usr/bin/g++")}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// An explicit write is the only way back, which is what the admin panel's
	// Create does.
	if err := store.Upsert(ctx, key, langDoc(t, "C++ restored", "/usr/bin/g++"), ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	row, err := store.GetByKey(ctx, key)
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if row == nil {
		t.Fatal("an explicit upsert did not revive the language")
	}
}
