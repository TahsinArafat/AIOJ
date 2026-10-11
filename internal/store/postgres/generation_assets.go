package postgres

import (
	"context"
	"database/sql"
	"github.com/tahsinarafat/aioj/internal/model"
)

type GenerationAssetStore struct{ db *sql.DB }

func NewGenerationAssetStore(db *sql.DB) *GenerationAssetStore { return &GenerationAssetStore{db: db} }
func (s *GenerationAssetStore) Create(ctx context.Context, a *model.ProblemGenerationAsset) error {
	return s.db.QueryRowContext(ctx, `INSERT INTO problem_generation_assets(problem_id,job_id,generator_language,generator_source,reference_solution_language,reference_solution_source,seed_manifest,validation_summary,testdata_path) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at`, a.ProblemID, a.JobID, a.GeneratorLanguage, a.GeneratorSource, a.ReferenceSolutionLanguage, a.ReferenceSolutionSource, a.SeedManifest, a.ValidationSummary, a.TestdataPath).Scan(&a.ID, &a.CreatedAt)
}
