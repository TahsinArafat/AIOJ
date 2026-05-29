package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type PlagiarismStore struct {
	db *sql.DB
}

func NewPlagiarismStore(db *sql.DB) *PlagiarismStore {
	return &PlagiarismStore{db: db}
}

func (s *PlagiarismStore) CreateReport(ctx context.Context, r *model.PlagiarismReport) error {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO plagiarism_reports (contest_id, threshold, created_by)
		 VALUES ($1, $2, $3) RETURNING id, status, created_at`,
		r.ContestID, r.Threshold, r.CreatedBy,
	).Scan(&r.ID, &r.Status, &r.CreatedAt)
	return err
}

func (s *PlagiarismStore) GetReportByID(ctx context.Context, id string) (*model.PlagiarismReport, error) {
	var r model.PlagiarismReport
	var compAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, contest_id, status, threshold, total_pairs, flagged_count,
		        COALESCE(error_message, ''), created_by, created_at, completed_at
		 FROM plagiarism_reports WHERE id = $1`, id).Scan(
		&r.ID, &r.ContestID, &r.Status, &r.Threshold, &r.TotalPairs, &r.FlaggedCount,
		&r.ErrorMessage, &r.CreatedBy, &r.CreatedAt, &compAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if compAt.Valid {
		r.CompletedAt = &compAt.Time
	}
	return &r, nil
}

func (s *PlagiarismStore) GetReportByContest(ctx context.Context, contestID string) (*model.PlagiarismReport, error) {
	var r model.PlagiarismReport
	var compAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, contest_id, status, threshold, total_pairs, flagged_count,
		        COALESCE(error_message, ''), created_by, created_at, completed_at
		 FROM plagiarism_reports WHERE contest_id = $1 ORDER BY created_at DESC LIMIT 1`, contestID).Scan(
		&r.ID, &r.ContestID, &r.Status, &r.Threshold, &r.TotalPairs, &r.FlaggedCount,
		&r.ErrorMessage, &r.CreatedBy, &r.CreatedAt, &compAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if compAt.Valid {
		r.CompletedAt = &compAt.Time
	}
	return &r, nil
}

func (s *PlagiarismStore) UpdateReportStatus(ctx context.Context, id string, status model.PlagiarismReportStatus, errMsg string) error {
	var compAt *time.Time
	if status == model.PlagiarismStatusCompleted || status == model.PlagiarismStatusFailed {
		now := time.Now()
		compAt = &now
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE plagiarism_reports SET status = $1, error_message = $2, completed_at = $3 WHERE id = $4`,
		status, errMsg, compAt, id)
	return err
}

func (s *PlagiarismStore) UpdateReportCounts(ctx context.Context, id string, totalPairs, flaggedCount int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE plagiarism_reports SET total_pairs = $1, flagged_count = $2 WHERE id = $3`,
		totalPairs, flaggedCount, id)
	return err
}

func (s *PlagiarismStore) CreatePair(ctx context.Context, p *model.PlagiarismPair) error {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO plagiarism_pairs (report_id, problem_id, submission_a_id, submission_b_id,
		                              user_a_id, user_b_id, similarity, matched_lines)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, status, created_at, updated_at`,
		p.ReportID, p.ProblemID, p.SubmissionAID, p.SubmissionBID,
		p.UserAID, p.UserBID, p.Similarity, p.MatchedLines,
	).Scan(&p.ID, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return err
}

func (s *PlagiarismStore) GetPairByID(ctx context.Context, id string) (*model.PlagiarismPair, error) {
	var p model.PlagiarismPair
	err := s.db.QueryRowContext(ctx,
		`SELECT id, report_id, problem_id, submission_a_id, submission_b_id,
		        user_a_id, user_b_id, similarity, status, matched_lines, created_at, updated_at
		 FROM plagiarism_pairs WHERE id = $1`, id).Scan(
		&p.ID, &p.ReportID, &p.ProblemID, &p.SubmissionAID, &p.SubmissionBID,
		&p.UserAID, &p.UserBID, &p.Similarity, &p.Status, &p.MatchedLines, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PlagiarismStore) ListPairsByReport(ctx context.Context, reportID string, offset, limit int) ([]model.PlagiarismPairDetail, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM plagiarism_pairs WHERE report_id = $1", reportID).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT pp.id, pp.report_id, pp.problem_id, pp.submission_a_id, pp.submission_b_id,
		        pp.user_a_id, pp.user_b_id, pp.similarity, pp.status, pp.matched_lines, pp.created_at, pp.updated_at,
		        prob.title, ua.username, ub.username, sa.language, sb.language
		 FROM plagiarism_pairs pp
		 JOIN problems prob ON pp.problem_id = prob.id
		 JOIN users ua ON pp.user_a_id = ua.id
		 JOIN users ub ON pp.user_b_id = ub.id
		 JOIN submissions sa ON pp.submission_a_id = sa.id
		 JOIN submissions sb ON pp.submission_b_id = sb.id
		 WHERE pp.report_id = $1
		 ORDER BY pp.similarity DESC
		 OFFSET $2 LIMIT $3`,
		reportID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var pairs []model.PlagiarismPairDetail
	for rows.Next() {
		var p model.PlagiarismPairDetail
		err := rows.Scan(
			&p.ID, &p.ReportID, &p.ProblemID, &p.SubmissionAID, &p.SubmissionBID,
			&p.UserAID, &p.UserBID, &p.Similarity, &p.Status, &p.MatchedLines, &p.CreatedAt, &p.UpdatedAt,
			&p.ProblemTitle, &p.UserAUsername, &p.UserBUsername, &p.SubmissionALang, &p.SubmissionBLang)
		if err != nil {
			return nil, 0, err
		}
		pairs = append(pairs, p)
	}
	if pairs == nil {
		pairs = []model.PlagiarismPairDetail{}
	}
	return pairs, total, nil
}

func (s *PlagiarismStore) UpdatePairStatus(ctx context.Context, id string, status model.PlagiarismPairStatus) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE plagiarism_pairs SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

func (s *PlagiarismStore) GetPairsByUser(ctx context.Context, userID string) ([]model.PlagiarismPairDetail, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pp.id, pp.report_id, pp.problem_id, pp.submission_a_id, pp.submission_b_id,
		        pp.user_a_id, pp.user_b_id, pp.similarity, pp.status, pp.matched_lines, pp.created_at, pp.updated_at,
		        prob.title, ua.username, ub.username, sa.language, sb.language
		 FROM plagiarism_pairs pp
		 JOIN problems prob ON pp.problem_id = prob.id
		 JOIN users ua ON pp.user_a_id = ua.id
		 JOIN users ub ON pp.user_b_id = ub.id
		 JOIN submissions sa ON pp.submission_a_id = sa.id
		 JOIN submissions sb ON pp.submission_b_id = sb.id
		 WHERE pp.user_a_id = $1 OR pp.user_b_id = $1
		 ORDER BY pp.similarity DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []model.PlagiarismPairDetail
	for rows.Next() {
		var p model.PlagiarismPairDetail
		err := rows.Scan(
			&p.ID, &p.ReportID, &p.ProblemID, &p.SubmissionAID, &p.SubmissionBID,
			&p.UserAID, &p.UserBID, &p.Similarity, &p.Status, &p.MatchedLines, &p.CreatedAt, &p.UpdatedAt,
			&p.ProblemTitle, &p.UserAUsername, &p.UserBUsername, &p.SubmissionALang, &p.SubmissionBLang)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, p)
	}
	if pairs == nil {
		pairs = []model.PlagiarismPairDetail{}
	}
	return pairs, nil
}
