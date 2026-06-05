package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestClassStore_LifeCycle(t *testing.T) {
	dsn := "postgres://aioj:aioj_secret@localhost:5432/aioj?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	orgStore := NewOrganizationStore(db)
	classStore := NewClassStore(db)

	// Create test user
	userID := uuid.New().String()
	user := &model.User{
		ID:           userID,
		Username:     "class_test_user_" + uuid.New().String()[:8],
		Email:        "class_test_user_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	userStore.Create(ctx, user)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)

	// Create Organization
	org := &model.Organization{
		Name:        "Class Test Org",
		Description: "A test organization",
		CreatedBy:   userID,
	}
	orgStore.Create(ctx, org)
	defer db.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", org.ID)

	// 1. Create Class
	class := &model.Class{
		OrganizationID: org.ID,
		Name:           "CS101",
		Description:    "Intro to CS",
		InviteCode:     "INVITE12",
		CreatedBy:      userID,
	}
	if err := classStore.Create(ctx, class); err != nil {
		t.Fatalf("failed to create class: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM classes WHERE id = $1", class.ID)

	// 2. Get Class
	got, err := classStore.GetByID(ctx, class.ID)
	if err != nil {
		t.Fatalf("failed to get class: %v", err)
	}
	if got == nil {
		t.Fatal("expected class to be found, got nil")
	}
	if got.Name != class.Name {
		t.Errorf("got name %q, want %q", got.Name, class.Name)
	}

	// 3. Get Class by Invite Code
	gotByCode, err := classStore.GetByInviteCode(ctx, "INVITE12")
	if err != nil || gotByCode == nil {
		t.Fatalf("failed to get class by invite code: %v", err)
	}

	// 4. Add Class Member (Student)
	userID2 := uuid.New().String()
	user2 := &model.User{
		ID:           userID2,
		Username:     "class_test_user_2_" + uuid.New().String()[:8],
		Email:        "class_test_user_2_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	userStore.Create(ctx, user2)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID2)

	if err := classStore.AddMember(ctx, class.ID, userID2, "student"); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	isMem, err := classStore.IsMember(ctx, class.ID, userID2)
	if err != nil || !isMem {
		t.Errorf("expected user to be class member: isMem=%v, err=%v", isMem, err)
	}
}
