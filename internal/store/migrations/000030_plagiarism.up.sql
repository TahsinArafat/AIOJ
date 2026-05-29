CREATE TABLE plagiarism_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    threshold DOUBLE PRECISION NOT NULL DEFAULT 0.70,
    total_pairs INTEGER NOT NULL DEFAULT 0,
    flagged_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE plagiarism_pairs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID NOT NULL REFERENCES plagiarism_reports(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    submission_a_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    submission_b_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    user_a_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    similarity DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'flagged', 'ignored')),
    matched_lines INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_plagiarism_reports_contest ON plagiarism_reports(contest_id);
CREATE INDEX idx_plagiarism_reports_status ON plagiarism_reports(status);
CREATE INDEX idx_plagiarism_pairs_report ON plagiarism_pairs(report_id);
CREATE INDEX idx_plagiarism_pairs_similarity ON plagiarism_pairs(similarity DESC);
CREATE INDEX idx_plagiarism_pairs_status ON plagiarism_pairs(status);
CREATE INDEX idx_plagiarism_pairs_users ON plagiarism_pairs(user_a_id, user_b_id);
