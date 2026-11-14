package postgres

import "github.com/tahsinarafat/aioj/internal/store"

// The handler structs hold concrete *postgres.X types, so nothing forced these
// implementations to keep matching their interfaces. A delete signature was
// widened here and the interface kept the old shape, which compiled cleanly and
// would have gone unnoticed until someone swapped in a fake. These assertions
// make that impossible to miss.
var (
	_ store.APIKeyStore       = (*APIKeyStore)(nil)
	_ store.WebhookStore      = (*WebhookStore)(nil)
	_ store.TrainingPlanStore = (*TrainingPlanStore)(nil)
	_ store.ProblemStore      = (*ProblemStore)(nil)
	_ store.ContestStore      = (*ContestStore)(nil)
	_ store.OrganizationStore = (*OrganizationStore)(nil)
	_ store.TeamStore         = (*TeamStore)(nil)
	_ store.GymStore          = (*GymStore)(nil)
)

// The submission, contest and user stores are each split into roles, with the
// wide interface reduced to an embed of them. Assert each role separately: a
// signature that drifts on any one role must fail here, not at the one call site
// that happens not to be exercised.
var (
	_ store.SubmissionReader      = (*SubmissionStore)(nil)
	_ store.SubmissionWriter      = (*SubmissionStore)(nil)
	_ store.SubmissionJudgeQueue  = (*SubmissionStore)(nil)
	_ store.SubmissionRemoteJudge = (*SubmissionStore)(nil)
	_ store.SubmissionStats       = (*SubmissionStore)(nil)

	_ store.ProblemReader      = (*ProblemStore)(nil)
	_ store.ProblemWriter      = (*ProblemStore)(nil)
	_ store.ProblemAccessStore = (*ProblemStore)(nil)
	_ store.ProblemReviewStore = (*ProblemStore)(nil)

	_ store.ContestReader       = (*ContestStore)(nil)
	_ store.ContestWriter       = (*ContestStore)(nil)
	_ store.ContestProblemStore = (*ContestStore)(nil)
	_ store.ContestAccessStore  = (*ContestStore)(nil)
	_ store.ContestResultStore  = (*ContestStore)(nil)

	_ store.UserReader          = (*UserStore)(nil)
	_ store.UserWriter          = (*UserStore)(nil)
	_ store.UserAdminStore      = (*UserStore)(nil)
	_ store.UserCredentialStore = (*UserStore)(nil)

	_ store.AIGenerationAdminStore = (*AIGenerationStore)(nil)
	_ store.AIGenerationJobStore   = (*AIGenerationStore)(nil)
)
