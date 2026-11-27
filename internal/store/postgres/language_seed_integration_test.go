package postgres

import (
	"context"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/testdb"
)

/**
 * The boot path, end to end: the shipped lang/*.yaml files must become rows.
 *
 * This is the step that decides whether a fresh install has any languages at
 * all. It is worth its own test because every part of it can fail quietly —
 * ShippedDefaults returning an empty map, Seed treating an empty map as a
 * successful no-op, or the migration never having run — and the symptom is an
 * empty submit dropdown rather than an error.
 *
 * The rows created here are removed again, and only the ones this test created:
 * a shipped language an operator has already customised must survive, which is
 * the whole point of INSERT-only seeding.
 */
func TestLanguageStore_SeedsTheShippedDefaults(t *testing.T) {
	db := testdb.Require(t)
	ctx := context.Background()
	store := NewLanguageStore(db)

	defaults, err := compiler.ShippedDefaults()
	if err != nil {
		t.Fatalf("ShippedDefaults: %v", err)
	}
	if len(defaults) == 0 {
		t.Fatal("ShippedDefaults returned nothing; a fresh install would have no languages")
	}

	// Note which keys already exist so the cleanup below cannot delete an
	// operator's row.
	pre := map[string]bool{}
	rows, err := db.QueryContext(ctx, `SELECT key FROM languages`)
	if err != nil {
		t.Fatalf("list existing: %v", err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			t.Fatalf("scan: %v", err)
		}
		pre[k] = true
	}
	rows.Close()

	created, err := store.Seed(ctx, defaults)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	t.Logf("seeded %d language(s), %d already present", created, len(defaults)-created)

	live, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	have := map[string]bool{}
	for _, l := range live {
		have[l.Key] = true
	}
	for key := range defaults {
		if !have[key] {
			t.Errorf("shipped language %q is not in the registry after seeding", key)
		}
	}

	// A second seed on the same data must create nothing.
	again, err := store.Seed(ctx, defaults)
	if err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	if again != 0 {
		t.Errorf("second seed created %d rows, want 0", again)
	}

	t.Cleanup(func() {
		for key := range defaults {
			if pre[key] {
				continue
			}
			if _, err := db.ExecContext(context.Background(),
				`DELETE FROM languages WHERE key = $1`, key); err != nil {
				t.Logf("cleanup %s: %v", key, err)
			}
		}
	})
}
