ALTER TABLE submissions 
    ADD COLUMN submission_type VARCHAR(16) NOT NULL DEFAULT 'code';

ALTER TABLE submissions 
    ADD CONSTRAINT submissions_type_check 
    CHECK (submission_type IN ('code', 'output'));
