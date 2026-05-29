package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestOrganizationStore_LifeCycle(t *testing.T) {
	dsn := "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	orgStore := NewOrganizationStore(db)

	// Create test user
	userID := uuid.New().String()
	user := &model.User{
		ID:           userID,
		Username:     "org_test_user_" + uuid.New().String()[:8],
		Email:        "org_test_user_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	if err := userStore.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)

	// 1. Create Organization
	org := &model.Organization{
		Name:        "Test Org",
		Description: "A test organization",
		CreatedBy:   userID,
	}
	if err := orgStore.Create(ctx, org); err != nil {
		t.Fatalf("failed to create organization: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", org.ID)

	// 2. Get Organization
	got, err := orgStore.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("failed to get organization: %v", err)
	}
	if got == nil {
		t.Fatal("expected organization to be found, got nil")
	}
	if got.Name != org.Name {
		t.Errorf("got name %q, want %q", got.Name, org.Name)
	}

	// 3. Verify owner is member
	role, err := orgStore.GetMemberRole(ctx, org.ID, userID)
	if err != nil {
		t.Fatalf("failed to get member role: %v", err)
	}
	if role != "owner" {
		t.Errorf("got role %q, want %q", role, "owner")
	}

	// 4. Update Organization
	org.Name = "Updated Name"
	if err := orgStore.Update(ctx, org.ID, org); err != nil {
		t.Fatalf("failed to update organization: %v", err)
	}
	got2, _ := orgStore.GetByID(ctx, org.ID)
	if got2.Name != "Updated Name" {
		t.Errorf("got name %q after update, want %q", got2.Name, "Updated Name")
	}

	// 5. Add Member
	userID2 := uuid.New().String()
	user2 := &model.User{
		ID:           userID2,
		Username:     "org_test_user_2_" + uuid.New().String()[:8],
		Email:        "org_test_user_2_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	userStore.Create(ctx, user2)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID2)

	if err := orgStore.AddMember(ctx, org.ID, userID2, "member"); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	isMem, err := orgStore.IsMember(ctx, org.ID, userID2)
	if err != nil || !isMem {
		t.Errorf("expected user to be member: isMem=%v, err=%v", isMem, err)
	}

	// 6. List Members
	members, err := orgStore.GetMembers(ctx, org.ID)
	if err != nil || len(members) != 2 {
		t.Errorf("expected 2 members, got %d, err=%v", len(members), err)
	}

	// 7. Remove Member
	if err := orgStore.RemoveMember(ctx, org.ID, userID2); err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}
	isMem2, _ := orgStore.IsMember(ctx, org.ID, userID2)
	if isMem2 {
		t.Error("expected user to no longer be member")
	}
}
