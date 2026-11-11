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
