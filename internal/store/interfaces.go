package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// PendingSubmissionInfo is returned by ListPendingWithContext.
// It carries just enough context for the reclaimer to assign priority.
type PendingSubmissionInfo struct {
	ID           string
	ContestStart *time.Time
	ContestEnd   *time.Time
	// IsUpsolving distinguishes a post-contest upsolve from an in-window
	// contest submission. It is persisted on the row (migration 75) so the
	// distinction survives even when the contest window has since closed.
	IsUpsolving bool
}

// UserStore is the whole user surface. Prefer one of the roles below when a
// consumer needs only part of it; this union exists for the two consumers that
// genuinely span it (the auth flow and the admin console).
//
// The roles were measured, not guessed. Twenty consumers hold a UserStore and
// fourteen of them use one to five methods: three want GetByUsername alone, two
// want GetByID alone, one wants MarkEmailVerified alone. Depending on all 21
// forced every one of those to be tested through a 21-method fake, which is why
// the handlers carried their own one-method interfaces instead.
type UserStore interface {
	UserReader
	UserWriter
	UserAdminStore
	UserCredentialStore
}

// UserReader is the lookup side: identity by id, handle or address, plus the
// two profile projections the public pages render.
type UserReader interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetPublicProfile(ctx context.Context, username string) (*model.PublicProfile, error)
	GetProfile(ctx context.Context, userID string) (*model.UserProfile, error)
}

// UserWriter is account creation and self-service profile editing.
type UserWriter interface {
	Create(ctx context.Context, user *model.User) error
	UpdateProfile(ctx context.Context, userID string, p *model.UserProfile) error
	// UpdateRating is the rating job's write: a contest result moves the
	// account's rating and its historical maximum.
	UpdateRating(ctx context.Context, userID string, rating, maxRating, contestCount int) error
}

// UserAdminStore is the admin console's surface: the filtered listing, the
// detail view, and the moderation and role operations.
type UserAdminStore interface {
	ListUsers(ctx context.Context, offset, limit int) ([]model.User, int, error)
	// ListAdminUsers is the filtered/sorted/paginated listing the admin panel
	// uses. ListUsers is kept for the plain offset/limit callers.
	ListAdminUsers(ctx context.Context, f model.UserFilter) ([]model.AdminUserRow, int, error)
	// GetAdminUser returns one account with the profile and session facts the
	// admin detail view needs, or (nil, nil) when the id does not exist.
	GetAdminUser(ctx context.Context, id string) (*model.AdminUserDetail, error)
	// SetUserBan sets or clears the ban. A reason is only meaningful when
	// banning; unbanning clears both columns.
	SetUserBan(ctx context.Context, id string, banned bool, reason string) error
	// CountAdmins feeds the "never remove the last administrator" guard.
	CountAdmins(ctx context.Context) (int, error)
	UpdateRole(ctx context.Context, id, role string) error
	// SetEmailVerified lets an admin mark an address verified (support path for
	// a user whose mail never arrived) or un-verify it.
	SetEmailVerified(ctx context.Context, id string, verified bool) error
}

// UserCredentialStore is everything the sign-in path touches: the password, the
// ban check that must stop the next token, and the email-verification flag.
//
// Separate from UserAdminStore because the auth flow must not be able to change
// a role or count admins, and the admin console must not be able to set a
// password without going through the reset path that flags it.
type UserCredentialStore interface {
	// IsBanned reports whether the account is suspended. Read on every login and
	// refresh: a ban that did not stop the next token would not be a ban.
	IsBanned(ctx context.Context, id string) (bool, error)
	// TouchLastLogin records a successful sign-in. Best effort: a login must
	// not fail because a bookkeeping write did.
	TouchLastLogin(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	// SetMustChangePassword re-flags an account whose password an administrator
	// just set, so the admin-chosen value cannot become permanent.
	SetMustChangePassword(ctx context.Context, id string, required bool) error
	MarkEmailVerified(ctx context.Context, userID string) error
	IsEmailVerified(ctx context.Context, userID string) (bool, error)
}

// EmailVerificationTokenStore persists hashed one-shot email-verify links.
type EmailVerificationTokenStore interface {
	Create(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error
	GetByHash(ctx context.Context, tokenHash string) (*model.EmailVerificationToken, error)
	MarkUsed(ctx context.Context, id string) error
}

type ProblemStore interface {
	Create(ctx context.Context, p *model.Problem) error
	GetByID(ctx context.Context, id string) (*model.Problem, error)
	GetBySlug(ctx context.Context, slug string) (*model.Problem, error)
	// List returns the public archive for ordinary callers. admin=true widens
	// it to every problem regardless of review state, including drafts, so an
	// admin can see the problems still waiting for approval.
	List(ctx context.Context, offset, limit int, admin bool) ([]model.ProblemListItem, int, error)
	ListWithFilter(ctx context.Context, offset, limit int, admin bool, difficulty string, tags []string, search string, source string, rating string, sortBy string) ([]model.ProblemListItem, int, error)
	ListByCreatedBy(ctx context.Context, userID string, offset, limit int) ([]model.ProblemListItem, int, error)
	// ListSelectable is the contest problem picker: the public archive plus the
	// problems this user may attach (created by them, or shared with them).
	ListSelectable(ctx context.Context, userID string, offset, limit int, search string) ([]model.ProblemListItem, int, error)
	GetAllTags(ctx context.Context) ([]string, error)
	// InUnreleasedContest reports whether the problem is being used by a contest
	// that is open right now, so public reads can withhold its difficulty metadata
	// and keep it out of the archive. An upcoming or unpublished round is false.
	InUnreleasedContest(ctx context.Context, problemID string) (bool, error)
	// StaffedByContest reports whether userID staffs (manager/judge/tester) a
	// contest this problem is attached to. This is what lets one judge open a
	// problem a different setter attached to the round they both run, without
	// granting anything on the public archive.
	StaffedByContest(ctx context.Context, problemID, userID string) (bool, error)
	UpdateCounts(ctx context.Context, id string, addSubmission, addAccepted int) error
	Update(ctx context.Context, id string, p *model.Problem) error
	Delete(ctx context.Context, id string) error
	// SetReviewState applies an admin verdict. It also derives visible from the
	// new status, so approval is the only path to a public problem.
	SetReviewState(ctx context.Context, id, status, note, reviewerID string) error
	// ListByStatus is the review queue (status = pending) and the setter's
	// filtered views.
	ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.ProblemListItem, int, error)
	AddPermission(ctx context.Context, problemID, userID, accessLevel string) error
	RemovePermission(ctx context.Context, problemID, userID string) error
	GetPermissions(ctx context.Context, problemID string) ([]model.ProblemPermission, error)
	// HasAccess reports whether the user holds one of requiredLevels on this
	// problem. The bare bool is deliberate and must stay: a store failure is
	// reported as false, which is the fail-closed answer for a permission check.
	// Returning (bool, error) would move that decision into every caller, where
	// the cheap mistake is to allow on error.
	HasAccess(ctx context.Context, problemID, userID string, requiredLevels ...string) bool
	// ListAccessLevels returns the user's access level per problem, omitting
	// problems they have no permission row for. One query for a whole page of
	// submissions instead of one HasAccess call per row.
	ListAccessLevels(ctx context.Context, userID string, problemIDs []string) (map[string]string, error)
	GetRecommendations(ctx context.Context, userID string, currentRating int) (*model.RecommendationsResponse, error)
}

// ProblemI18nStore persists per-problem translations of title/description.
// A missing translation is not an error -- Get returns (nil, nil) so callers
// fall back to the problem's default text, mirroring dmoj's Coalesce behaviour.
type ProblemI18nStore interface {
	Get(ctx context.Context, problemID, language string) (*model.ProblemI18n, error)
	ListForProblem(ctx context.Context, problemID string) ([]model.ProblemI18n, error)
	Upsert(ctx context.Context, t *model.ProblemI18n) error
	Delete(ctx context.Context, problemID, language string) error
}

// SubmissionStore is the whole submission surface. Prefer one of the narrower
// interfaces below when a consumer needs only part of it; this union exists for
// the one consumer that genuinely spans it (the submission handler).
//
// The roles are declared separately because they were measured, not guessed:
// the judge reclaimer needs two methods, the hack service one, and plagiarism
// one. Depending on all 30 forced every one of those consumers to be tested
// through a 30-method fake.
type SubmissionStore interface {
	SubmissionReader
	SubmissionWriter
	SubmissionJudgeQueue
	SubmissionRemoteJudge
	SubmissionStats
}

// SubmissionReader is the read side: the listings and lookups the HTTP layer
// serves from. A consumer that only renders submissions needs this and nothing
// else.
type SubmissionReader interface {
	GetByID(ctx context.Context, id string) (*model.Submission, error)
	ListByProblem(ctx context.Context, problemID string, offset, limit int) ([]model.Submission, int, error)
	ListByUser(ctx context.Context, userID string, offset, limit int, problemID, contestID string) ([]model.Submission, int, error)
	ListPublicByUser(ctx context.Context, userID string, offset, limit int) ([]model.Submission, int, error)
	ListByContest(ctx context.Context, contestID string, offset, limit int, filter model.SubmissionFilter) ([]model.Submission, int, error)
	// ListByProblemManage is the setter-facing problem listing: unlike
	// ListByProblem it carries the submitter's username and the problem's
	// slug/title, and honours the setter filters (user, contest, verdict, …).
	ListByProblemManage(ctx context.Context, problemID string, offset, limit int, filter model.SubmissionFilter) ([]model.Submission, int, error)
	// ListPublic is the site-wide status page. Rows the viewer may not see —
	// hidden problems, submissions from a contest that is still running — are
	// filtered in SQL so they never reach the response.
	ListPublic(ctx context.Context, offset, limit int, filter model.PublicSubmissionFilter) ([]model.Submission, int, error)
	// ListRecentFailures returns the newest System Error submissions with the
	// judge's failure text, for admins triaging a broken judge or problem.
	ListRecentFailures(ctx context.Context, limit int) ([]model.Submission, error)
}

// SubmissionWriter is the HTTP layer's write side: creating a submission,
// recording a verdict, and the rejudge/void operations an operator triggers.
type SubmissionWriter interface {
	Create(ctx context.Context, s *model.Submission) error
	UpdateStatus(ctx context.Context, id string, status model.SubmissionStatus)
	UpdateResult(ctx context.Context, id string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) error
	// RecomputeProblemCounts rebuilds problems.submission_count/accepted_count
	// from the submissions table. Rejudging does not touch those counters, so
	// they are recomputed once per batch instead of being double-counted.
	RecomputeProblemCounts(ctx context.Context, problemID string) error
	// ListIDsForProblemRejudge returns judged local submissions of one problem,
	// optionally narrowed to a contest, for a batch rejudge.
	ListIDsForProblemRejudge(ctx context.Context, problemID string, filter model.SubmissionFilter) ([]string, error)
	// ListForBulkRejudge returns IDs of all judged local submissions in a
	// contest (optionally filtered to one problem) suitable for rejudging.
	// It excludes submissions still pending/judging to avoid double-dispatch.
	ListForBulkRejudge(ctx context.Context, contestID, problemID string) ([]string, error)
	// ResetForRejudge atomically resets a batch of submissions to 'pending'
	// and clears their judge result, preparing them for re-execution.
	ResetForRejudge(ctx context.Context, ids []string) error
	// SetSkipped voids a submission so it stops counting as an attempt and stops
	// contributing points, leaving the verdict it earned on the row. It refuses a
	// submission that is pending, judging or already skipped, and reports whether
	// a row was changed so the caller can answer 404/409.
	SetSkipped(ctx context.Context, id string) (bool, error)
}

// SubmissionJudgeQueue is the claim protocol between the judge workers and the
// pending queue. Its whole point is the fencing: a result is only accepted
// while the writing worker still holds the claim.
type SubmissionJudgeQueue interface {
	// ClaimPending atomically acquires a pending submission for one judge
	// worker. The claim token fences late results from an older worker after a
	// stale claim has been reclaimed and re-judged.
	ClaimPending(ctx context.Context, id, claimToken string) (bool, error)
	// UpdateResultClaimed persists a result only while claimToken still owns
	// the judging row. It returns false when a newer claim has superseded it.
	UpdateResultClaimed(ctx context.Context, id, claimToken string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) (bool, error)
	// RequeueStale atomically moves submissions stuck in "judging" for longer
	// than olderThan back to "pending", clears their claim, and returns their
	// IDs. Concurrent reclaimer sweeps cannot return the same row twice.
	RequeueStale(ctx context.Context, olderThan time.Duration) ([]string, error)
	ListPending(ctx context.Context, limit int) ([]string, error)
	// ListPendingWithContext returns pending local submissions enriched with
	// their contest time window so the reclaimer can assign correct priority.
	ListPendingWithContext(ctx context.Context, limit int) ([]PendingSubmissionInfo, error)
}

// SubmissionRemoteJudge is the hand-off to a remote judge: what still needs
// submitting, and where its result landed. Separate from the local judge queue
// because the vjudge service never claims or fences anything.
type SubmissionRemoteJudge interface {
	UpdateRemoteID(ctx context.Context, id string, remoteID string, remoteURL string) error
	UpdateBotID(ctx context.Context, id string, botID string, botSlug string) error
	GetPendingRemoteSubmissions(ctx context.Context) ([]model.PendingRemoteSubmission, error)
	GetUnsubmittedRemoteSubmissions(ctx context.Context) ([]model.Submission, error)
}

// SubmissionStats is the aggregate read side. Its three methods are report
// queries, not row reads, which is why they sit apart from SubmissionReader.
type SubmissionStats interface {
	GetProblemStats(ctx context.Context, problemID string) (*model.ProblemStats, error)
	GetUserStats(ctx context.Context, userID string) (*model.UserProblemStats, error)
	GetPlatformStats(ctx context.Context) (*model.PlatformStats, error)
}

type RefreshTokenStore interface {
	Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	// Consume deletes the token and returns its owner, giving at-most-once use.
	// Refresh therefore rotates: a replayed token finds no row and is rejected.
	//
	// A miss must be reported as postgres.ErrRefreshTokenSpent. Any other error
	// means the store could not answer, which is not a verdict on the token.
	Consume(ctx context.Context, tokenHash string) (string, error)
	// RevokeAll invalidates every refresh token for a user. Used on password
	// change and reset so a stolen token cannot outlive the credential it came
	// from.
	RevokeAll(ctx context.Context, userID string) error
}

// TOTPSecretStore persists per-user TOTP secrets for 2FA.
type TOTPSecretStore interface {
	Upsert(ctx context.Context, userID, secret string) error
	Enable(ctx context.Context, userID string) error
	Disable(ctx context.Context, userID string) error
	Get(ctx context.Context, userID string) (*model.TOTPSecret, error)
}

// BackupCodeRow is a single unused 2FA recovery code (hash only).
type BackupCodeRow struct {
	ID   string
	Hash string
}

// BackupCodeStore stores one-shot SHA-256 hashed backup codes.
type BackupCodeStore interface {
	Create(ctx context.Context, id, userID, codeHash string) error
	ListActive(ctx context.Context, userID string) ([]BackupCodeRow, error)
	Consume(ctx context.Context, id string) error
}

// OAuthLinkStore maps external provider identities onto local users.
type OAuthLinkStore interface {
	Create(ctx context.Context, link *model.OAuthLink) error
	GetByProviderUser(ctx context.Context, provider, providerUserID string) (*model.OAuthLink, error)
	ListByUser(ctx context.Context, userID string) ([]model.OAuthLink, error)
	Delete(ctx context.Context, id string) error
}

// ContestStore is the whole contest surface. Prefer one of the roles below when
// a consumer needs only part of it; this union exists for the contest handler,
// which is the one consumer that genuinely spans it (26 methods).
//
// Measured: of fourteen consumers, five want GetByID alone and three more want
// GetByID plus the access checks. Those eight were each carrying their own
// one-or-two-method interface; with the roles declared here they can name one.
type ContestStore interface {
	ContestReader
	ContestWriter
	ContestProblemStore
	ContestAccessStore
	ContestResultStore
}

// ContestReader is the read side the HTTP layer serves contests from. GetByID is
// the single most-used method in the package: eight consumers need nothing else.
type ContestReader interface {
	GetByID(ctx context.Context, id string) (*model.Contest, error)
	GetBySlug(ctx context.Context, slug string) (*model.Contest, error)
	List(ctx context.Context, offset, limit int) ([]model.Contest, int, error)
	// ListForViewer is the public listing: it applies the per-caller visibility
	// predicate (published, not hidden from this viewer, division match) that
	// List does not, so the contest archive does not leak a draft or an
	// unlisted round. division is optional; a nil viewerID means anonymous.
	ListForViewer(ctx context.Context, offset, limit int, division *int, viewerID string) ([]model.Contest, int, error)
	ListByCreatedBy(ctx context.Context, userID string, offset, limit int) ([]model.Contest, int, error)
	// ListByStatus is the admin contest review queue.
	ListByStatus(ctx context.Context, status string, offset, limit int) ([]model.Contest, int, error)
}

// ContestWriter is authoring: create, edit, delete, and the review verdict.
type ContestWriter interface {
	Create(ctx context.Context, c *model.Contest) error
	Update(ctx context.Context, c *model.Contest) error
	Delete(ctx context.Context, id string) error
	// SetReviewState applies an admin verdict and derives visible from it, so a
	// proposal cannot publish itself.
	SetReviewState(ctx context.Context, id, status, note, reviewerID string) error
}

// ContestProblemStore is the roster of problems attached to a round, and the
// per-viewer progress the contest page shows.
//
// Named ContestProblemStore rather than ContestProblemAdmin because the same
// shape is what the contest-problem handler embeds when it narrows the store.
type ContestProblemStore interface {
	AddProblem(ctx context.Context, contestID, problemID, index string, score, sortOrder int) error
	UpdateProblem(ctx context.Context, contestID, problemID, index string, score, sortOrder int) error
	RemoveProblem(ctx context.Context, contestID, problemID string) error
	GetProblems(ctx context.Context, contestID string) ([]model.ContestProblem, error)
	// RenumberProblems rewrites every letter and sort_order from position,
	// closing any gap left by a removal. Returns the new ordering.
	RenumberProblems(ctx context.Context, contestID string) ([]model.ContestProblem, error)
	GetContestProblemByIndex(ctx context.Context, contestID, index string) (*model.Problem, error)
	// GetProblemProgress is the per-viewer solve state and round-wide solve count
	// for each problem, keyed by problem id. viewerID may be empty.
	GetProblemProgress(ctx context.Context, contestID, viewerID string, beforeTime *time.Time) (map[string]model.ContestProblemProgress, error)
}

// ContestAccessStore is "may this caller see this round", plus the per-round
// permission rows that answer it. Separate from the reader because access checks
// are consulted on every content surface and must be satisfiable by a test
// without a contest, a problem list or a scoreboard.
type ContestAccessStore interface {
	GetParticipants(ctx context.Context, contestID string) ([]string, error)
	IsParticipant(ctx context.Context, contestID, userID string) (bool, error)
	AddPermission(ctx context.Context, contestID, userID, accessLevel string) error
	RemovePermission(ctx context.Context, contestID, userID string) error
	GetPermissions(ctx context.Context, contestID string) ([]model.ContestPermission, error)
	// HasAccess reports whether the user holds one of requiredLevels on this
	// contest. Fails closed on a store error, for the reason given on the
	// problem store's HasAccess.
	HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool
	CheckGroupRestriction(ctx context.Context, contestID, userID string) (bool, error)
	// GetUsername resolves one user id to a display name, best effort. Prefer
	// GetUsernames where the ids are known up front.
	GetUsername(ctx context.Context, userID string) string
	// GetUsernames resolves many ids to display names in one query. Prefer this
	// over GetUsername wherever the ids are known up front: a standings page
	// carries up to 500 entries, and the per-row call was a query each.
	GetUsernames(ctx context.Context, userIDs []string) (map[string]string, error)
	// GetDisplayNames maps user id to the name a scoreboard should show. For a
	// generated contestant account that is the team it was minted for, not the
	// throwaway username. A missing key means "use the username".
	GetDisplayNames(ctx context.Context, contestID string, userIDs []string) (map[string]model.ContestDisplayName, error)
}

// ContestResultStore is the judged output: standings rows, statistics, and the
// team entry a round may register instead of individuals.
type ContestResultStore interface {
	// GetScoreboardRows is the judged rows the standings are computed from.
	// beforeTime, when set, applies the freeze cutoff.
	GetScoreboardRows(ctx context.Context, contestID string, beforeTime *time.Time) ([]model.ScoreboardRow, error)
	// GetContestStats is the jury's statistics tab: participant, submission and
	// per-problem aggregates in one round trip.
	GetContestStats(ctx context.Context, contestID string) (*model.ContestStats, error)
	// RegisterTeam / ListTeamRegistrations back team-mode entry. A round that
	// registers teams instead of individuals needs both, and the rating
	// recalculation reads the second.
	RegisterTeam(ctx context.Context, contestID, teamID string) (*model.TeamRegistration, error)
	ListTeamRegistrations(ctx context.Context, contestID string) ([]model.TeamRegistration, error)
}

// ContestGrantStore holds proof that a contestant typed a round's access
// password. Kept apart from ContestStore because the read side of access control
// needs only the one question (IsGranted) and the write side only ever happens
// from the unlock and password-change flows — folding it into ContestStore would
// hand every consumer three methods it must not call.
type ContestGrantStore interface {
	// Grant records an entitlement. Idempotent: unlocking twice is not an error.
	Grant(ctx context.Context, contestID, userID string) error
	IsGranted(ctx context.Context, contestID, userID string) (bool, error)
	// RevokeAll drops every entitlement for a round. Called when the password
	// changes, so an old password's grants die with it.
	RevokeAll(ctx context.Context, contestID string) error
}

type RatingStore interface {
	CreateHistory(ctx context.Context, h *model.RatingHistory) error
	GetByUser(ctx context.Context, userID string, limit int) ([]model.RatingHistory, error)
	GetByContest(ctx context.Context, contestID string) ([]model.RatingHistory, error)
	GetLatestByUser(ctx context.Context, userID string) (*model.RatingHistory, error)
}

type RegistrationStore interface {
	Register(ctx context.Context, contestID, userID string) error
	Unregister(ctx context.Context, contestID, userID string) error
	IsRegistered(ctx context.Context, contestID, userID string) (bool, error)
	GetRegistrations(ctx context.Context, contestID string) ([]model.ContestRegistration, error)
	GetRegistrationCount(ctx context.Context, contestID string) (int, error)
}

type SetterStore interface {
	CreateApplication(ctx context.Context, userID, reason string) error
	ListApplications(ctx context.Context) ([]model.SetterApplication, error)
	UpdateApplicationStatus(ctx context.Context, userID, status string) error
	GetApplication(ctx context.Context, userID string) (*model.SetterApplication, error)
}

type VirtualStore interface {
	Create(ctx context.Context, v *model.VirtualContest) error
	GetByID(ctx context.Context, id string) (*model.VirtualContest, error)
	GetActiveByUser(ctx context.Context, userID string) (*model.VirtualContest, error)
	Complete(ctx context.Context, id string) error
}

type GymStore interface {
	Create(ctx context.Context, g *model.GymContest) error
	GetByID(ctx context.Context, id string) (*model.GymContest, error)
	// GetByIDForViewer applies the per-caller visibility rule (a private gym is
	// visible to its owner and its members), where GetByID does not.
	GetByIDForViewer(ctx context.Context, id, viewUserID string) (*model.GymContest, error)
	List(ctx context.Context, offset, limit int, filter model.GymFilter) ([]model.GymContest, int, error)
	MarkSolved(ctx context.Context, gymID, userID string) error
	IsSolved(ctx context.Context, gymID, userID string) (bool, error)
}

type HackStore interface {
	Create(ctx context.Context, h *model.Hack) error
	GetByID(ctx context.Context, id string) (*model.Hack, error)
	UpdateStatus(ctx context.Context, id, status string, success bool) error
	GetByContest(ctx context.Context, contestID string) ([]model.Hack, error)
	GetHackableSubmissions(ctx context.Context, contestID, problemID string) ([]model.Submission, error)
}

type NotificationStore interface {
	Create(ctx context.Context, n *model.Notification) error
	GetByUser(ctx context.Context, userID string, unreadOnly bool, limit int) ([]model.Notification, error)
	MarkAsRead(ctx context.Context, id string) error
	MarkAllAsRead(ctx context.Context, userID string) error
	GetUnreadCount(ctx context.Context, userID string) (int, error)
	Delete(ctx context.Context, id string) error
	GetPreferences(ctx context.Context, userID string) (*model.NotificationPreferences, error)
	UpdatePreferences(ctx context.Context, userID string, prefs *model.NotificationPreferences) error
}

type GroupStore interface {
	Create(ctx context.Context, g *model.Group) error
	GetByID(ctx context.Context, id string) (*model.Group, error)
	GetByInviteCode(ctx context.Context, code string) (*model.Group, error)
	List(ctx context.Context, offset, limit int) ([]model.GroupListItem, int, error)
	ListByUser(ctx context.Context, userID string) ([]model.GroupListItem, error)
	Update(ctx context.Context, id string, g *model.Group) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, groupID, userID, role string) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	GetMembers(ctx context.Context, groupID string) ([]model.GroupMember, error)
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	GetMemberCount(ctx context.Context, groupID string) (int, error)
	AddContest(ctx context.Context, groupID, contestID string) error
	RemoveContest(ctx context.Context, groupID, contestID string) error
	GetContests(ctx context.Context, groupID string) ([]model.Contest, error)
	GetMemberRole(ctx context.Context, groupID, userID string) (string, error)
	UpdateMemberRole(ctx context.Context, groupID, userID, role string) error
	GetPendingMembers(ctx context.Context, groupID string) ([]model.GroupMember, error)
	GetUserPendingInvites(ctx context.Context, userID string) ([]model.GroupPendingInvite, error)
}

type TeamStore interface {
	Create(ctx context.Context, t *model.Team) error
	GetByID(ctx context.Context, id string) (*model.Team, error)
	List(ctx context.Context, offset, limit int) ([]model.TeamListItem, int, error)
	Update(ctx context.Context, id string, t *model.Team) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, teamID, userID, role string) error
	RemoveMember(ctx context.Context, teamID, userID string) error
	GetMembers(ctx context.Context, teamID string) ([]model.TeamMember, error)
	IsMember(ctx context.Context, teamID, userID string) (bool, error)
	GetUserTeams(ctx context.Context, userID string) ([]model.TeamListItem, error)
	UpdateRating(ctx context.Context, teamID string, newRating int) error
	GetMemberRole(ctx context.Context, teamID, userID string) (string, error)
	UpdateMemberRole(ctx context.Context, teamID, userID, role string) error
	GetPendingMembers(ctx context.Context, teamID string) ([]model.TeamMember, error)
	GetUserPendingInvites(ctx context.Context, userID string) ([]model.PendingInvite, error)
}

type BlogStore interface {
	CreatePost(ctx context.Context, p *model.BlogPost) error
	GetPostByID(ctx context.Context, id string) (*model.BlogPost, error)
	ListPosts(ctx context.Context, offset, limit int, tag string) ([]model.BlogListItem, int, error)
	ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.BlogListItem, int, error)
	GetCommentsByUser(ctx context.Context, userID string, offset, limit int) ([]model.Comment, int, error)
	UpdatePost(ctx context.Context, id string, p *model.BlogPost) error
	DeletePost(ctx context.Context, id string) error
	CreateComment(ctx context.Context, c *model.Comment) error
	GetComments(ctx context.Context, parentType, parentID string) ([]model.Comment, error)
	DeleteComment(ctx context.Context, id string) error
	Vote(ctx context.Context, userID, targetType, targetID string, value int) error
	GetUserVote(ctx context.Context, userID, targetType, targetID string) (int, error)
}

type EditorialStore interface {
	Create(ctx context.Context, e *model.Editorial) error
	GetByID(ctx context.Context, id string) (*model.Editorial, error)
	GetByProblem(ctx context.Context, problemID string) ([]model.Editorial, error)
	List(ctx context.Context, offset, limit int) ([]model.Editorial, int, error)
	UpsertOfficial(ctx context.Context, e *model.Editorial) error
}

type APIKeyStore interface {
	Create(ctx context.Context, k *model.APIKey) error
	GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error)
	GetByUser(ctx context.Context, userID string) ([]model.APIKey, error)
	// Delete is scoped to the owner so one user cannot delete another's key.
	Delete(ctx context.Context, id, userID string) error
	UpdateLastUsed(ctx context.Context, id string) error
	IncrementRequestCount(ctx context.Context, keyID string, windowStart time.Time) error
	GetRequestCount(ctx context.Context, keyID string, windowStart time.Time) (int, error)
}

type PasswordResetTokenStore interface {
	Create(ctx context.Context, tokenID, userID, tokenHash string, expiresAt time.Time) error
	GetByHash(ctx context.Context, tokenHash string) (*model.PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string) error
}

type WebhookStore interface {
	Create(ctx context.Context, w *model.Webhook) error
	GetByUser(ctx context.Context, userID string) ([]model.Webhook, error)
	// Delete is scoped to the owner for the same reason as API keys.
	Delete(ctx context.Context, id, userID string) error
	GetByEvent(ctx context.Context, eventType string) ([]model.Webhook, error)
	// RecordDelivery writes one delivery attempt, so a webhook that never fires
	// leaves a trace instead of failing silently.
	RecordDelivery(ctx context.Context, d *model.WebhookDelivery) error
}

type LanguageLimitStore interface {
	Set(ctx context.Context, limit *model.LanguageLimit) error
	Get(ctx context.Context, problemID, languageID string) (*model.LanguageLimit, error)
	GetByProblem(ctx context.Context, problemID string) ([]*model.LanguageLimit, error)
	Delete(ctx context.Context, problemID, languageID string) error
}

type OrganizationStore interface {
	Create(ctx context.Context, o *model.Organization) error
	GetByID(ctx context.Context, id string) (*model.Organization, error)
	List(ctx context.Context, offset, limit int) ([]model.OrganizationListItem, int, error)
	Update(ctx context.Context, id string, o *model.Organization) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, orgID, userID, role string) error
	RemoveMember(ctx context.Context, orgID, userID string) error
	GetMembers(ctx context.Context, orgID string) ([]model.OrganizationMember, error)
	IsMember(ctx context.Context, orgID, userID string) (bool, error)
	GetMemberRole(ctx context.Context, orgID, userID string) (string, error)
	GetMemberCount(ctx context.Context, orgID string) (int, error)
	ListByUser(ctx context.Context, userID string) ([]model.OrganizationListItem, error)
}

type ClassStore interface {
	Create(ctx context.Context, c *model.Class) error
	GetByID(ctx context.Context, id string) (*model.Class, error)
	GetByInviteCode(ctx context.Context, code string) (*model.Class, error)
	List(ctx context.Context, orgID string, offset, limit int) ([]model.ClassListItem, int, error)
	Update(ctx context.Context, id string, c *model.Class) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, classID, userID, role string) error
	RemoveMember(ctx context.Context, classID, userID string) error
	GetMembers(ctx context.Context, classID string) ([]model.ClassMember, error)
	IsMember(ctx context.Context, classID, userID string) (bool, error)
	GetMemberCount(ctx context.Context, classID string) (int, error)
}

type TrainingPlanStore interface {
	Create(ctx context.Context, p *model.TrainingPlan) error
	GetByID(ctx context.Context, id string) (*model.TrainingPlan, error)
	List(ctx context.Context, offset, limit int, orgID *string, publicOnly bool) ([]model.TrainingPlan, int, error)
	Update(ctx context.Context, id string, p *model.TrainingPlan) error
	Delete(ctx context.Context, id string) error
	CreateSection(ctx context.Context, s *model.TrainingPlanSection) error
	GetSections(ctx context.Context, planID string) ([]model.TrainingPlanSection, error)
	// GetSectionByID and GetProblemRowByID exist so a handler given a bare
	// section/problem id can walk up to the plan before authorizing it.
	GetSectionByID(ctx context.Context, sectionID string) (*model.TrainingPlanSection, error)
	GetProblemRowByID(ctx context.Context, id string) (*model.TrainingPlanProblem, error)
	// DeleteSection and RemoveProblem are scoped to the planId the caller was
	// authorized against, so a stale id cannot touch another plan.
	DeleteSection(ctx context.Context, sectionID, planID string) error
	AddProblem(ctx context.Context, sectionID, problemID string, sortOrder, points int) error
	RemoveProblem(ctx context.Context, id, planID string) error
	GetProblems(ctx context.Context, sectionID string) ([]model.TrainingPlanProblem, error)
	Enroll(ctx context.Context, planID, userID string) error
	Unenroll(ctx context.Context, planID, userID string) error
	IsEnrolled(ctx context.Context, planID, userID string) (bool, error)
	GetEnrollments(ctx context.Context, planID string) ([]model.TrainingPlanEnrollment, error)
	MarkProblemCompleted(ctx context.Context, planID, userID, problemID string) error
	GetProgress(ctx context.Context, planID, userID string) (*model.PlanProgressSummary, error)
	GetDetail(ctx context.Context, planID, userID string) (*model.TrainingPlanDetail, error)
	FindByProblem(ctx context.Context, problemID string) ([]string, error)
}

type PlagiarismStore interface {
	CreateReport(ctx context.Context, r *model.PlagiarismReport) error
	GetReportByID(ctx context.Context, id string) (*model.PlagiarismReport, error)
	GetReportByContest(ctx context.Context, contestID string) (*model.PlagiarismReport, error)
	UpdateReportStatus(ctx context.Context, id string, status model.PlagiarismReportStatus, errMsg string) error
	UpdateReportCounts(ctx context.Context, id string, totalPairs, flaggedCount int) error
	CreatePair(ctx context.Context, p *model.PlagiarismPair) error
	GetPairByID(ctx context.Context, id string) (*model.PlagiarismPair, error)
	ListPairsByReport(ctx context.Context, reportID string, offset, limit int) ([]model.PlagiarismPairDetail, int, error)
	UpdatePairStatus(ctx context.Context, id string, status model.PlagiarismPairStatus) error
	GetPairsByUser(ctx context.Context, userID string) ([]model.PlagiarismPairDetail, error)
}

type BalloonStore interface {
	CreateRequest(ctx context.Context, contestID, submissionID, userID, problemID string) error
	ListByContest(ctx context.Context, contestID string) ([]model.BalloonRequest, error)
	Dispatch(ctx context.Context, id string) error
}

type PrintStore interface {
	Create(ctx context.Context, contestID, userID, filename, content string) error
	ListByContest(ctx context.Context, contestID string) ([]model.PrintRequest, error)
	UpdateStatus(ctx context.Context, id, status string) error
}

type BotAccountStore interface {
	List(ctx context.Context, offset, limit int) ([]model.BotAccount, int, error)
	ListByPlatform(ctx context.Context, platform string) ([]model.BotAccount, error)
	GetByID(ctx context.Context, id string) (*model.BotAccount, error)
	Create(ctx context.Context, ba *model.BotAccount) error
	Update(ctx context.Context, id string, ba *model.BotAccount) error
	Delete(ctx context.Context, id string) error
	IncrementFailures(ctx context.Context, id string) error
	ResetFailures(ctx context.Context, id string) error
	MarkUsed(ctx context.Context, id string) error
	UpdateLastPoll(ctx context.Context, id string) error
}

type SystemSettingsStore interface {
	List(ctx context.Context) ([]model.SystemSetting, error)
	Get(ctx context.Context, key string) (*model.SystemSetting, error)
	Set(ctx context.Context, s *model.SystemSetting) error
	Delete(ctx context.Context, key string) error
}

type RemoteLanguageStore interface {
	ListByPlatform(ctx context.Context, platform string) ([]model.RemoteLanguage, error)
	GetByID(ctx context.Context, id string) (*model.RemoteLanguage, error)
	Create(ctx context.Context, rl *model.RemoteLanguage) error
	Update(ctx context.Context, id string, rl *model.RemoteLanguage) error
	Delete(ctx context.Context, id string) error
	BulkUpsert(ctx context.Context, platform string, langs []model.RemoteLanguage) error
}

type OnsiteUserStore interface {
	CreateBatch(ctx context.Context, contestID string, users []model.BatchUserRequest) ([]model.OnsiteBatchUser, error)
	GetByUsername(ctx context.Context, username string) (*model.OnsiteBatchUser, error)
	MarkUsed(ctx context.Context, id string, userID string) error
	ListByContest(ctx context.Context, contestID string) ([]model.OnsiteBatchUser, error)
	DeleteByContest(ctx context.Context, contestID string) error
	// DeleteByID is scoped to the contest the caller was authorized against.
	DeleteByID(ctx context.Context, contestID, id string) error
	AutoRegister(ctx context.Context, contestID, userID string) error
}

type ClarificationStore interface {
	Create(ctx context.Context, c *model.Clarification) error
	GetByID(ctx context.Context, id string) (*model.Clarification, error)
	// ListByContest returns what the caller may see: their own questions plus
	// public answers, or everything for the jury (userID nil). problemID and
	// answered narrow the queue; both nil means no narrowing.
	ListByContest(ctx context.Context, contestID string, userID *string, problemID *string, answered *bool) ([]model.Clarification, error)
	Answer(ctx context.Context, id string, answer string, answeredBy string, isPublic bool) error
	AnswerMany(ctx context.Context, ids []string, answer string, answeredBy string, isPublic bool) error
}

type ContestNoticeStore interface {
	Create(ctx context.Context, n *model.ContestNotice) error
	ListByContest(ctx context.Context, contestID string) ([]model.ContestNotice, error)
	Delete(ctx context.Context, id string) error
}

// AIModelStore manages AI endpoint configurations.
// List omits the api_key; GetByID and GetDefault include it.
type AIModelStore interface {
	List(ctx context.Context) ([]model.AIModel, error)
	GetByID(ctx context.Context, id string) (*model.AIModel, error)
	// GetDefault returns the highest-priority enabled model, or (nil, nil) if none exists.
	GetDefault(ctx context.Context) (*model.AIModel, error)
	Create(ctx context.Context, am *model.AIModel) error
	Update(ctx context.Context, id string, am *model.AIModel) error
	Delete(ctx context.Context, id string) error
	SetEnabled(ctx context.Context, id string, enabled bool) error
}

type AIGenerationStore interface {
	Create(ctx context.Context, job *model.AIGenerationJob) error
	GetByID(ctx context.Context, id string) (*model.AIGenerationJob, error)
	ListByProblem(ctx context.Context, problemID string, offset, limit int) ([]model.AIGenerationJob, error)
	// ListByActor returns jobs created by the given actor, newest first.
	ListByActor(ctx context.Context, actorID string, offset, limit int) ([]model.AIGenerationJob, int, error)
	// ListAll returns all jobs visible to an admin, optionally filtered by status
	// and/or actorID (empty string = no filter). Newest first.
	ListAll(ctx context.Context, actorID, status string, offset, limit int) ([]model.AIGenerationJob, int, error)
	// AdminRequestCancel cancels any job regardless of actor (admin use only).
	AdminRequestCancel(ctx context.Context, id string) (bool, error)
	// CountActiveByActor reports the actor's in-flight jobs. Jobs the actor has
	// already asked to cancel are excluded: they no longer hold a slot.
	CountActiveByActor(ctx context.Context, actorID string) (running, queued int, err error)
	// ActiveJobByActor returns the in-flight job holding the actor's slot, so a
	// limit error can name it. Nil when nothing is in flight.
	ActiveJobByActor(ctx context.Context, actorID string) (*model.AIGenerationJob, error)
	ClaimNext(ctx context.Context, claimToken string) (*model.AIGenerationJob, error)
	Heartbeat(ctx context.Context, id, claimToken, stage string, progress int) error
	Complete(ctx context.Context, id, claimToken string, proposal, validation json.RawMessage, promptTokens, completionTokens, latencyMS int) error
	Fail(ctx context.Context, id, claimToken, code, message string, validation json.RawMessage) error
	RequestCancel(ctx context.Context, id, actorID string) (bool, error)
	MarkCancelled(ctx context.Context, id, claimToken string) error
	IsCancellationRequested(ctx context.Context, id string) (bool, error)
	MarkDiscarded(ctx context.Context, id, actorID string) (bool, error)
	MarkApplied(ctx context.Context, id string) error
	RequeueStale(ctx context.Context, olderThan time.Duration) ([]string, error)
	// PurgeCompletedBefore deletes terminal (complete/failed/cancelled/
	// discarded/applied) jobs older than the cutoff and returns how many rows
	// were removed. Active jobs are never deleted, however old, so a long run
	// cannot be purged out from under the worker holding its claim.
	PurgeCompletedBefore(ctx context.Context, cutoff time.Time) (int, error)
}

// ContestAdjustmentStore is the manual-correction ledger for contest standings.
//
// A correction is a signed delta layered over the computed standings, so these
// methods never touch submissions: the judge's verdict stays what it was, and
// the human decision lives in its own row with a reason attached.
type ContestAdjustmentStore interface {
	Create(ctx context.Context, a *model.ContestAdjustment) error
	ListByContest(ctx context.Context, contestID string) ([]model.ContestAdjustment, error)
	Delete(ctx context.Context, contestID, id string) error
	// TotalsByUser returns the summed deltas per participant, which is what the
	// scoreboard applies after the format has scored the submissions.
	TotalsByUser(ctx context.Context, contestID string) (map[string]model.AdjustmentTotals, error)
}
