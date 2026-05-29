package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type TrainingPlanStore struct {
	db *sql.DB
}

func NewTrainingPlanStore(db *sql.DB) *TrainingPlanStore {
	return &TrainingPlanStore{db: db}
}

func (s *TrainingPlanStore) Create(ctx context.Context, p *model.TrainingPlan) error {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO training_plans (title, description, organization_id, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`,
		p.Title, p.Description, p.OrganizationID, p.CreatedBy,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return err
}

func (s *TrainingPlanStore) GetByID(ctx context.Context, id string) (*model.TrainingPlan, error) {
	var p model.TrainingPlan
	var orgID sql.NullString
	var orgName sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT tp.id, tp.title, tp.description, tp.organization_id,
		        o.name, tp.created_by, u.username,
		        (SELECT COUNT(*) FROM training_plan_sections WHERE plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_problems tpp
		         JOIN training_plan_sections tps ON tpp.section_id = tps.id
		         WHERE tps.plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_enrollments WHERE plan_id = tp.id),
		        tp.created_at, tp.updated_at
		 FROM training_plans tp
		 JOIN users u ON tp.created_by = u.id
		 LEFT JOIN organizations o ON tp.organization_id = o.id
		 WHERE tp.id = $1`,
		id).Scan(&p.ID, &p.Title, &p.Description, &orgID,
		&orgName, &p.CreatedBy, &p.CreatorName,
		&p.SectionCount, &p.ProblemCount, &p.EnrolledCount,
		&p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if orgID.Valid {
		p.OrganizationID = &orgID.String
		p.IsPublic = false
	}
	if orgName.Valid {
		p.OrgName = orgName.String
	}
	if p.OrganizationID == nil {
		p.IsPublic = true
	}
	return &p, nil
}

func (s *TrainingPlanStore) List(ctx context.Context, offset, limit int, orgID *string, publicOnly bool) ([]model.TrainingPlan, int, error) {
	var total int
	var countQuery, listQuery string
	var args []interface{}

	if orgID != nil {
		countQuery = "SELECT COUNT(*) FROM training_plans WHERE organization_id = $1"
		listQuery = `SELECT tp.id, tp.title, tp.description, tp.organization_id,
		        COALESCE(o.name, ''), tp.created_by, u.username,
		        (SELECT COUNT(*) FROM training_plan_sections WHERE plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_problems tpp
		         JOIN training_plan_sections tps ON tpp.section_id = tps.id
		         WHERE tps.plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_enrollments WHERE plan_id = tp.id),
		        tp.created_at, tp.updated_at
		 FROM training_plans tp
		 JOIN users u ON tp.created_by = u.id
		 LEFT JOIN organizations o ON tp.organization_id = o.id
		 WHERE tp.organization_id = $1
		 ORDER BY tp.created_at DESC
		 OFFSET $2 LIMIT $3`
		args = []interface{}{*orgID, offset, limit}
		s.db.QueryRowContext(ctx, countQuery, *orgID).Scan(&total)
	} else if publicOnly {
		countQuery = "SELECT COUNT(*) FROM training_plans WHERE organization_id IS NULL"
		listQuery = `SELECT tp.id, tp.title, tp.description, tp.organization_id,
		        '', tp.created_by, u.username,
		        (SELECT COUNT(*) FROM training_plan_sections WHERE plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_problems tpp
		         JOIN training_plan_sections tps ON tpp.section_id = tps.id
		         WHERE tps.plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_enrollments WHERE plan_id = tp.id),
		        tp.created_at, tp.updated_at
		 FROM training_plans tp
		 JOIN users u ON tp.created_by = u.id
		 WHERE tp.organization_id IS NULL
		 ORDER BY tp.created_at DESC
		 OFFSET $1 LIMIT $2`
		args = []interface{}{offset, limit}
		s.db.QueryRowContext(ctx, countQuery).Scan(&total)
	} else {
		countQuery = "SELECT COUNT(*) FROM training_plans"
		listQuery = `SELECT tp.id, tp.title, tp.description, tp.organization_id,
		        COALESCE(o.name, ''), tp.created_by, u.username,
		        (SELECT COUNT(*) FROM training_plan_sections WHERE plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_problems tpp
		         JOIN training_plan_sections tps ON tpp.section_id = tps.id
		         WHERE tps.plan_id = tp.id),
		        (SELECT COUNT(*) FROM training_plan_enrollments WHERE plan_id = tp.id),
		        tp.created_at, tp.updated_at
		 FROM training_plans tp
		 JOIN users u ON tp.created_by = u.id
		 LEFT JOIN organizations o ON tp.organization_id = o.id
		 ORDER BY tp.created_at DESC
		 OFFSET $1 LIMIT $2`
		args = []interface{}{offset, limit}
		s.db.QueryRowContext(ctx, countQuery).Scan(&total)
	}

	rows, err := s.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.TrainingPlan
	for rows.Next() {
		var p model.TrainingPlan
		var orgIDVal sql.NullString
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &orgIDVal,
			&p.OrgName, &p.CreatedBy, &p.CreatorName,
			&p.SectionCount, &p.ProblemCount, &p.EnrolledCount,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if orgIDVal.Valid {
			p.OrganizationID = &orgIDVal.String
			p.IsPublic = false
		} else {
			p.IsPublic = true
		}
		items = append(items, p)
	}
	if items == nil {
		items = []model.TrainingPlan{}
	}
	return items, total, nil
}

func (s *TrainingPlanStore) Update(ctx context.Context, id string, p *model.TrainingPlan) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE training_plans SET title = $1, description = $2, updated_at = NOW() WHERE id = $3`,
		p.Title, p.Description, id)
	return err
}

func (s *TrainingPlanStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM training_plans WHERE id = $1", id)
	return err
}

func (s *TrainingPlanStore) CreateSection(ctx context.Context, sec *model.TrainingPlanSection) error {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO training_plan_sections (plan_id, title, description, sort_order)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		sec.PlanID, sec.Title, sec.Description, sec.SortOrder,
	).Scan(&sec.ID)
	return err
}

func (s *TrainingPlanStore) GetSections(ctx context.Context, planID string) ([]model.TrainingPlanSection, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, plan_id, title, description, sort_order
		 FROM training_plan_sections
		 WHERE plan_id = $1
		 ORDER BY sort_order`,
		planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sections []model.TrainingPlanSection
	for rows.Next() {
		var sec model.TrainingPlanSection
		if err := rows.Scan(&sec.ID, &sec.PlanID, &sec.Title, &sec.Description, &sec.SortOrder); err != nil {
			return nil, err
		}
		sec.Problems = []model.TrainingPlanProblem{}
		sections = append(sections, sec)
	}
	if sections == nil {
		sections = []model.TrainingPlanSection{}
	}
	return sections, nil
}

func (s *TrainingPlanStore) DeleteSection(ctx context.Context, sectionID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM training_plan_sections WHERE id = $1", sectionID)
	return err
}

func (s *TrainingPlanStore) AddProblem(ctx context.Context, sectionID, problemID string, sortOrder, points int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO training_plan_problems (section_id, problem_id, sort_order, points)
		 VALUES ($1, $2, $3, $4)`,
		sectionID, problemID, sortOrder, points)
	return err
}

func (s *TrainingPlanStore) RemoveProblem(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM training_plan_problems WHERE id = $1", id)
	return err
}

func (s *TrainingPlanStore) GetProblems(ctx context.Context, sectionID string) ([]model.TrainingPlanProblem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, section_id, problem_id, sort_order, points
		 FROM training_plan_problems
		 WHERE section_id = $1
		 ORDER BY sort_order`,
		sectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var problems []model.TrainingPlanProblem
	for rows.Next() {
		var p model.TrainingPlanProblem
		if err := rows.Scan(&p.ID, &p.SectionID, &p.ProblemID, &p.SortOrder, &p.Points); err != nil {
			return nil, err
		}
		problems = append(problems, p)
	}
	if problems == nil {
		problems = []model.TrainingPlanProblem{}
	}
	return problems, nil
}

func (s *TrainingPlanStore) Enroll(ctx context.Context, planID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO training_plan_enrollments (plan_id, user_id)
		 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		planID, userID)
	return err
}

func (s *TrainingPlanStore) Unenroll(ctx context.Context, planID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`DELETE FROM training_plan_enrollments WHERE plan_id = $1 AND user_id = $2`,
		planID, userID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`DELETE FROM training_plan_progress WHERE plan_id = $1 AND user_id = $2`,
		planID, userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *TrainingPlanStore) IsEnrolled(ctx context.Context, planID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM training_plan_enrollments WHERE plan_id = $1 AND user_id = $2)`,
		planID, userID).Scan(&exists)
	return exists, err
}

func (s *TrainingPlanStore) GetEnrollments(ctx context.Context, planID string) ([]model.TrainingPlanEnrollment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.plan_id, e.user_id, u.username, e.enrolled_at
		 FROM training_plan_enrollments e
		 JOIN users u ON e.user_id = u.id
		 WHERE e.plan_id = $1
		 ORDER BY e.enrolled_at`,
		planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var enrollments []model.TrainingPlanEnrollment
	for rows.Next() {
		var e model.TrainingPlanEnrollment
		if err := rows.Scan(&e.PlanID, &e.UserID, &e.Username, &e.EnrolledAt); err != nil {
			return nil, err
		}
		enrollments = append(enrollments, e)
	}
	if enrollments == nil {
		enrollments = []model.TrainingPlanEnrollment{}
	}
	return enrollments, nil
}

func (s *TrainingPlanStore) MarkProblemCompleted(ctx context.Context, planID, userID, problemID string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO training_plan_progress (plan_id, user_id, problem_id, completed, completed_at)
		 VALUES ($1, $2, $3, true, $4)
		 ON CONFLICT (plan_id, user_id, problem_id)
		 DO UPDATE SET completed = true, completed_at = $4
		 WHERE NOT training_plan_progress.completed`,
		planID, userID, problemID, now)
	return err
}

func (s *TrainingPlanStore) GetProgress(ctx context.Context, planID, userID string) (*model.PlanProgressSummary, error) {
	var total, completed int
	err := s.db.QueryRowContext(ctx,
		`SELECT
		    (SELECT COUNT(*) FROM training_plan_problems tpp
		     JOIN training_plan_sections tps ON tpp.section_id = tps.id
		     WHERE tps.plan_id = $1),
		    (SELECT COUNT(*) FROM training_plan_progress
		     WHERE plan_id = $1 AND user_id = $2 AND completed = true)`,
		planID, userID).Scan(&total, &completed)
	if err != nil {
		return nil, err
	}
	pct := 0.0
	if total > 0 {
		pct = float64(completed) / float64(total) * 100
	}
	return &model.PlanProgressSummary{
		TotalProblems:     total,
		CompletedProblems: completed,
		Percentage:        pct,
	}, nil
}

func (s *TrainingPlanStore) GetDetail(ctx context.Context, planID, userID string) (*model.TrainingPlanDetail, error) {
	plan, err := s.GetByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}

	sections, err := s.GetSections(ctx, planID)
	if err != nil {
		return nil, err
	}

	for i := range sections {
		problems, err := s.GetProblems(ctx, sections[i].ID)
		if err != nil {
			return nil, err
		}
		sections[i].Problems = problems
	}

	detail := &model.TrainingPlanDetail{
		TrainingPlan: *plan,
		Sections:     sections,
		Enrolled:     false,
	}

	if userID != "" {
		enrolled, _ := s.IsEnrolled(ctx, planID, userID)
		detail.Enrolled = enrolled
		if enrolled {
			progress, _ := s.GetProgress(ctx, planID, userID)
			detail.Progress = progress
		}
	}

	return detail, nil
}

func (s *TrainingPlanStore) FindByProblem(ctx context.Context, problemID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT tps.plan_id
		 FROM training_plan_problems tpp
		 JOIN training_plan_sections tps ON tpp.section_id = tps.id
		 WHERE tpp.problem_id = $1`,
		problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var planIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		planIDs = append(planIDs, id)
	}
	return planIDs, nil
}
