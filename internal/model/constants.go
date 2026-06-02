package model

// User roles — must match CHECK constraint in 000001_init.up.sql.
const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"
	RoleUser    = "user"
	RoleBot     = "bot"
)

// Problem/contest collaboration access levels.
const (
	AccessLevelOwner    = "owner"
	AccessLevelCoAuthor = "co-author"
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
