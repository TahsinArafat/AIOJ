package model

// User roles — must match CHECK constraint in 000001_init.up.sql.
const (
	RoleAdmin  = "admin"
	RoleSetter = "setter"
	RoleUser   = "user"
	RoleBot    = "bot"
	// RoleContestant is the account a contest setter generates in bulk (the
	// "Onsite Teams" panel). It exists for one round only: it may reach that
	// contest and nothing else, so it is deliberately not an archive citizen.
	RoleContestant = "contestant"
)

// Problem/contest collaboration access levels. These must match the CHECK
// constraints in migration 000002 (problem_permissions) and 000039
// (contest_permissions): co_author is an underscore there, and every handler plus
// the collaborator picker already spell it that way. The constant said
// "co-author" — a value the column rejects — so anything that used the constant
// instead of a literal would have failed the insert.
const (
	AccessLevelOwner    = "owner"
	AccessLevelCoAuthor = "co_author"
	AccessLevelManager  = "manager"
	AccessLevelJudge    = "judge"
	AccessLevelTester   = "tester"
)

// Contest types — must match CHECK constraint in 000001_init.up.sql.
const (
	ContestTypeACM         = "acm"
	ContestTypeOI          = "oi"
	ContestTypeIOI         = "ioi"
	ContestTypePractice    = "practice"
	ContestTypeEducational = "educational"
)

// Problem difficulty — must match CHECK constraint in 000001_init.up.sql.
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

// Submission status strings (mirrors the verdict enum in DB).
const (
	StatusStringPending = "pending"
	StatusStringJudging = "judging"
	StatusStringAC      = "ac"
	StatusStringWA      = "wa"
	StatusStringTLE     = "tle"
	StatusStringMLE     = "mle"
	StatusStringRE      = "re"
	StatusStringSE      = "se"
	StatusStringCE      = "ce"
)
