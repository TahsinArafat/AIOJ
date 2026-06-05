package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestTrainingPlanStore_LifeCycle(t *testing.T) {
	dsn := "postgres://aioj:aioj_secret@localhost:5432/aioj?sslmode=disable"
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

	org := &model.Organization{
		Name:        "TP Test Org",
		Description: "A test organization",
		CreatedBy:   userID,
	}
	orgStore.Create(ctx, org)
	defer db.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", org.ID)

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

	sec := &model.TrainingPlanSection{
		PlanID:      tp.ID,
		Title:       "Sorting",
		Description: "Learn to sort",
		SortOrder:   0,
	}
	if err := tpStore.CreateSection(ctx, sec); err != nil {
		t.Fatalf("failed to create section: %v", err)
	}

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

	if err := tpStore.AddProblem(ctx, sec.ID, p1ID, 0, 10); err != nil {
		t.Fatalf("failed to add problem to section: %v", err)
	}

	if err := tpStore.Enroll(ctx, tp.ID, userID); err != nil {
		t.Fatalf("failed to enroll user: %v", err)
	}

	isEnrolled, err := tpStore.IsEnrolled(ctx, tp.ID, userID)
	if err != nil || !isEnrolled {
		t.Errorf("expected user to be enrolled: enrolled=%v, err=%v", isEnrolled, err)
	}

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

func TestTrainingPlanStore_AutoCompleteOnAC(t *testing.T) {
	dsn := "postgres://aioj:aioj_secret@localhost:5432/aioj?sslmode=disable"
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
	subStore := NewSubmissionStore(db)

	userID := uuid.New().String()
	user := &model.User{
		ID:           userID,
		Username:     "tp_ac_user_" + uuid.New().String()[:8],
		Email:        "tp_ac_user_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	userStore.Create(ctx, user)
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)

	org := &model.Organization{
		Name:        "TP AC Org",
		Description: "A test organization",
		CreatedBy:   userID,
	}
	orgStore.Create(ctx, org)
	defer db.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", org.ID)

	tp := &model.TrainingPlan{
		Title:          "Algo Path AC",
		Description:    "Learn algorithms",
		OrganizationID: &org.ID,
		CreatedBy:      userID,
	}
	tpStore.Create(ctx, tp)
	defer db.ExecContext(ctx, "DELETE FROM training_plans WHERE id = $1", tp.ID)

	sec := &model.TrainingPlanSection{
		PlanID:      tp.ID,
		Title:       "Sorting AC",
		Description: "Learn to sort",
		SortOrder:   0,
	}
	tpStore.CreateSection(ctx, sec)

	p1ID := uuid.New().String()
	p1 := &model.Problem{
		ID:          p1ID,
		Slug:        "tp-ac-prob-" + uuid.New().String()[:8],
		Title:       "Sort Array AC",
		Description: "Sort the array",
		TimeLimit:   1000,
		MemoryLimit: 262144,
		Difficulty:  "easy",
		Tags:        []string{},
		CreatedBy:   userID,
		Visible:     true,
	}
	probStore.Create(ctx, p1)
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", p1ID)

	tpStore.AddProblem(ctx, sec.ID, p1ID, 0, 10)

	tpStore.Enroll(ctx, tp.ID, userID)

	subID := uuid.New().String()
	sub := &model.Submission{
		ID:             subID,
		ProblemID:      p1ID,
		UserID:         userID,
		Language:       "cpp",
		SourceCode:     "int main() {}",
		Status:         model.StatusPending,
		SubmissionType: model.SubmissionTypeCode,
	}
	if err := subStore.Create(ctx, sub); err != nil {
		t.Fatalf("failed to create submission: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE id = $1", subID)

	err = subStore.UpdateResult(ctx, subID, model.StatusAC, 100, 10, 2048, "", []model.TestCaseResult{})
	if err != nil {
		t.Fatalf("failed to update result to AC: %v", err)
	}

	progress, err := tpStore.GetProgress(ctx, tp.ID, userID)
	if err != nil {
		t.Fatalf("failed to get progress: %v", err)
	}
	if progress.CompletedProblems != 1 {
		t.Errorf("expected 1 completed problem automatically via AC trigger, got %d", progress.CompletedProblems)
	}
}
