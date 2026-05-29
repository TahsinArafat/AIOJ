package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestTrainingPlanStore_LifeCycle(t *testing.T) {
	dsn := "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	probStore := NewProblemStore(db)
	orgStore := NewOrganizationStore(db)
	tpStore := NewTrainingPlanStore(db)

	// Create test user
	userID := uuid.New().String()
	user := &model.User{
		ID:           userID,
		Username:     "tp_test_user_" + uuid.New().String()[:8],
		Email:        "tp_test_user_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	userStore.Create(ctx, user)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)

	// Create Organization
	org := &model.Organization{
		Name:        "TP Test Org",
		Description: "A test organization",
		CreatedBy:   userID,
	}
	orgStore.Create(ctx, org)
	defer db.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", org.ID)

	// 1. Create Training Plan
	tp := &model.TrainingPlan{
		Title:          "Algo Path",
		Description:    "Learn algorithms",
		OrganizationID: &org.ID,
		CreatedBy:      userID,
	}
	if err := tpStore.Create(ctx, tp); err != nil {
		t.Fatalf("failed to create training plan: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM training_plans WHERE id = $1", tp.ID)

	// 2. Get Training Plan
	got, err := tpStore.GetByID(ctx, tp.ID)
	if err != nil {
		t.Fatalf("failed to get training plan: %v", err)
	}
	if got == nil {
		t.Fatal("expected training plan to be found, got nil")
	}
	if got.Title != tp.Title {
		t.Errorf("got title %q, want %q", got.Title, tp.Title)
	}

	// 3. Create Section
	sec := &model.TrainingPlanSection{
		PlanID:      tp.ID,
		Title:       "Sorting",
		Description: "Learn to sort",
		SortOrder:   0,
	}
	if err := tpStore.CreateSection(ctx, sec); err != nil {
		t.Fatalf("failed to create section: %v", err)
	}

	// 4. Create a problem
	p1ID := uuid.New().String()
	p1 := &model.Problem{
		ID:          p1ID,
		Slug:        "tp-test-prob-" + uuid.New().String()[:8],
		Title:       "Sort Array",
		Description: "Sort the array",
		TimeLimit:   1000,
		MemoryLimit: 262144,
		Difficulty:  "easy",
		Tags:        []string{},
		CreatedBy:   userID,
		Visible:     true,
	}
	if err := probStore.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", p1ID)

	// 5. Add Problem to Section
	if err := tpStore.AddProblem(ctx, sec.ID, p1ID, 0, 10); err != nil {
		t.Fatalf("failed to add problem to section: %v", err)
	}

	// 6. Enroll User
	if err := tpStore.Enroll(ctx, tp.ID, userID); err != nil {
		t.Fatalf("failed to enroll user: %v", err)
	}

	isEnrolled, err := tpStore.IsEnrolled(ctx, tp.ID, userID)
	if err != nil || !isEnrolled {
		t.Errorf("expected user to be enrolled: enrolled=%v, err=%v", isEnrolled, err)
	}

	// 7. Mark Problem Completed
	if err := tpStore.MarkProblemCompleted(ctx, tp.ID, userID, p1ID); err != nil {
		t.Fatalf("failed to mark problem completed: %v", err)
	}

	progress, err := tpStore.GetProgress(ctx, tp.ID, userID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if progress.CompletedProblems != 1 {
		t.Errorf("expected 1 completed problem, got %d", progress.CompletedProblems)
	}
}
