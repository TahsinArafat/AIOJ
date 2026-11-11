package postgres

import (
	"context"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// The API-key flow end to end against a real database.
//
// The feature shipped half-built: keys could be minted but nothing verified them
// (GetByHash had zero callers), so this covers the whole path the middleware now
// walks — issue, hash, resolve, owner lookup — plus the ownership scoping that
// stops one user deleting another's key.
func TestAPIKeyLifecycleDB(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	keys := NewAPIKeyStore(db)

	uid := "00000000-0000-0000-0000-000000000000" // seeded admin

	raw, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// The issuer and the verifier must produce the same digest; they lived in
	// different packages with nothing tying them together before.
	if hash != model.HashAPIKey(raw) {
		t.Fatal("GenerateAPIKey and model.HashAPIKey disagree on the digest")
	}

	k := &model.APIKey{UserID: uid, KeyHash: hash, Name: "integration-probe", RateLimit: 100}
	if err := keys.Create(ctx, k); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM api_keys WHERE id=$1", k.ID)

	got, err := keys.GetByHash(ctx, model.HashAPIKey(raw))
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if got == nil || got.UserID != uid {
		t.Fatalf("GetByHash did not resolve the presented key to its owner: %+v", got)
	}

	// The middleware resolves the owner to build claims.
	owner, err := users.GetByID(ctx, got.UserID)
	if err != nil || owner == nil {
		t.Fatalf("owner lookup failed: %v", err)
	}

	// A different owner must not be able to delete the key.
	other := "11111111-1111-1111-1111-111111111111"
	if err := keys.Delete(ctx, k.ID, other); err != nil {
		t.Fatalf("delete as other: %v", err)
	}
	if again, _ := keys.GetByHash(ctx, hash); again == nil {
		t.Fatal("a foreign owner deleted the key")
	}

	// The real owner can.
	if err := keys.Delete(ctx, k.ID, uid); err != nil {
		t.Fatalf("delete as owner: %v", err)
	}
	if again, _ := keys.GetByHash(ctx, hash); again != nil {
		t.Fatal("the owner could not delete their own key")
	}
}
