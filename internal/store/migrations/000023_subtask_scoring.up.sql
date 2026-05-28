ALTER TABLE problems 
    ADD COLUMN scoring_mode VARCHAR(16) NOT NULL DEFAULT 'complete',
    ADD COLUMN subtask_aggregation VARCHAR(8) NOT NULL DEFAULT 'min';

ALTER TABLE problems 
    ADD CONSTRAINT problems_scoring_mode_check 
    CHECK (scoring_mode IN ('complete', 'partial'));

ALTER TABLE problems 
    ADD CONSTRAINT problems_subtask_aggregation_check 
    CHECK (subtask_aggregation IN ('min', 'sum'));
