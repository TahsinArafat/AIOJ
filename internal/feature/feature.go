// Package feature implements admin-controlled feature gating.
//
// Every optional surface of the product (contests, gym, IDE, blog, …) is
// declared once in the registry below. An admin then stores a Policy per
// feature and the rest of the system asks one question: "may this principal
// use feature X?". The same registry drives the API gate, the /api/features
// endpoint the SPA boots from, and the admin panel — so a feature cannot be
// half-wired: adding an entry is enough to make it toggleable everywhere.
//
// The design deliberately separates three things:
//
//   - Feature: the static description (key, label, default policy). Compiled in.
//   - Policy:  the admin's decision (enabled, roles, rating window, message).
//   - Decision: the answer for one principal at one moment.
//
// Policies live in the existing `system_settings` table under the key
// `feature.<key>`, so there is no new table and every change is already covered
// by the admin audit log.
package feature

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

// Policy is one admin decision about one feature.
//
// The zero value is deliberately "on for everybody": an admin who has never
// touched a feature should not accidentally turn it off.
type Policy struct {
	// Enabled is the master switch. Off means off for everyone, including
	// admins — that is what "disable this feature" has to mean.
	Enabled bool `json:"enabled"`
	// Roles restricts the feature to these roles. Empty means every role.
	// Admins bypass this list (see Decide) so they can always reach a surface
	// they are responsible for.
	Roles []string `json:"roles,omitempty"`
	// MinRating/MaxRating restrict the feature to a rating window. 0 means
	// unbounded. Admins bypass these bounds for the same reason as Roles.
	MinRating int `json:"min_rating,omitempty"`
	MaxRating int `json:"max_rating,omitempty"`
	// Message is shown to a user who is denied access, e.g. "Div. 2 opens
	// after the round on Sunday".
	Message string `json:"message,omitempty"`
}

// Unrestricted is the policy that gates nothing.
var Unrestricted = Policy{Enabled: true}

// Roles that a policy may name. Mirrors the users.role CHECK constraint.
var KnownRoles = []string{model.RoleAdmin, model.RoleSetter, model.RoleUser, model.RoleBot}

// Rating bounds accepted from the admin API. Ratings on AIOJ start at 800 and
// the scale is open-ended, so 5000 leaves generous headroom while still
// rejecting typos like 1000000.
const (
	MinRatingBound = 0
	MaxRatingBound = 5000
	maxMessageLen  = 280
)

// Validate reports whether an admin may store p.
//
// It is intentionally strict: a policy naming an unknown role would silently
// deny access to everyone except admins, which is very hard to notice from the
// admin UI. Rejecting it at the edge turns a silent misconfiguration into a
// 400 the admin can act on.
func Validate(p Policy) error {
	if p.MinRating < MinRatingBound || p.MinRating > MaxRatingBound {
		return fmt.Errorf("min_rating must be between %d and %d", MinRatingBound, MaxRatingBound)
	}
	if p.MaxRating < MinRatingBound || p.MaxRating > MaxRatingBound {
		return fmt.Errorf("max_rating must be between %d and %d", MinRatingBound, MaxRatingBound)
	}
	if p.MinRating > 0 && p.MaxRating > 0 && p.MinRating > p.MaxRating {
		return errors.New("min_rating must not exceed max_rating")
	}
	if len(p.Message) > maxMessageLen {
		return fmt.Errorf("message must be at most %d characters", maxMessageLen)
	}
	for _, r := range p.Roles {
		if !isKnownRole(r) {
			return fmt.Errorf("unknown role %q", r)
		}
	}
	return nil
}

func isKnownRole(role string) bool {
	for _, r := range KnownRoles {
		if r == role {
			return true
		}
	}
	return false
}

// Normalize returns a copy of p with the invariants the rest of the code
// relies on: roles de-duplicated and sorted, unknown roles dropped, the rating
// window ordered. Callers validate first; this only removes noise so that two
// equivalent policies compare equal (and the admin UI does not show
// "customized" for a policy that is byte-different but semantically identical
// to the default).
func Normalize(p Policy) Policy {
	out := p
	out.Message = strings.TrimSpace(p.Message)

	seen := make(map[string]struct{}, len(p.Roles))
	roles := make([]string, 0, len(p.Roles))
	for _, r := range p.Roles {
		if !isKnownRole(r) {
			continue
		}
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		roles = append(roles, r)
	}
	sort.Strings(roles)
	if len(roles) == 0 {
		roles = nil
	}
	out.Roles = roles

	if out.MinRating < MinRatingBound {
		out.MinRating = 0
	}
	if out.MaxRating < MinRatingBound {
		out.MaxRating = 0
	}
	return out
}

// Equal reports whether two policies gate identically.
func Equal(a, b Policy) bool {
	a, b = Normalize(a), Normalize(b)
	if a.Enabled != b.Enabled || a.MinRating != b.MinRating || a.MaxRating != b.MaxRating || a.Message != b.Message {
		return false
	}
	if len(a.Roles) != len(b.Roles) {
		return false
	}
	for i := range a.Roles {
		if a.Roles[i] != b.Roles[i] {
			return false
		}
	}
	return true
}

// Category groups features in the admin UI.
type Category string

const (
	CategoryCore     Category = "core"
	CategoryPractice Category = "practice"
	CategorySocial   Category = "community"
	CategoryCompete  Category = "competition"
	CategoryPlatform Category = "platform"
	CategoryAuthor   Category = "authoring"
)

// Feature is the static description of a gateable surface.
type Feature struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Category    Category `json:"category"`
	// Default is the policy in force until an admin saves one.
	Default Policy `json:"default_policy"`
	// RequiresLogin forces authentication even when Default/Roles would allow
	// anonymous visitors. Used for surfaces that only make sense for an account.
	RequiresLogin bool `json:"requires_login"`
	// APIEnforced says whether the backend actually rejects requests, or
	// whether the gate is a UI-level hide. The admin UI shows this so nobody
	// believes they have locked down something that is only hidden.
	APIEnforced bool `json:"api_enforced"`
}

// Decision reasons. Empty reason means "allowed".
const (
	ReasonAllowed       = ""
	ReasonDisabled      = "disabled"
	ReasonLoginRequired = "login_required"
	ReasonRole          = "role"
	ReasonRating        = "rating"
)

// Principal is the caller a decision is made for.
type Principal struct {
	UserID    string
	Role      string
	Rating    int
	Anonymous bool
}

// IsAdmin reports whether the principal is an administrator.
func (p Principal) IsAdmin() bool { return p.Role == model.RoleAdmin }

// Decision is the answer for one principal.
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	// Message is the admin's explanation, when one is configured.
	Message string `json:"message,omitempty"`
}

// Denied builds a denial.
func Denied(reason, message string) Decision {
	return Decision{Allowed: false, Reason: reason, Message: message}
}

// Allowed builds an approval.
var Allowed = Decision{Allowed: true}

// Decide evaluates p for principal.
//
// Order matters and is part of the contract:
//
//  1. The master switch is absolute — a disabled feature is denied even to
//     admins, because "turn this off" must mean off.
//  2. Admins bypass the role list and the rating window. An admin has to be
//     able to reach the surface they are configuring, otherwise enabling a
//     rating gate would lock the only person able to undo it out of the page
//     that explains what happened.
//  3. RequiresLogin, then roles, then rating.
func Decide(f Feature, p Policy, principal Principal) Decision {
	p = Normalize(p)

	if !p.Enabled {
		return Denied(ReasonDisabled, p.Message)
	}
	if principal.IsAdmin() {
		return Allowed
	}
	if f.RequiresLogin && principal.Anonymous {
		return Denied(ReasonLoginRequired, p.Message)
	}
	if len(p.Roles) > 0 {
		if principal.Anonymous {
			// Anonymous callers have no role; telling them "your role is not
			// allowed" would be confusing, so ask them to sign in instead.
			return Denied(ReasonLoginRequired, p.Message)
		}
		allowed := false
		for _, r := range p.Roles {
			if r == principal.Role {
				allowed = true
				break
			}
		}
		if !allowed {
			return Denied(ReasonRole, p.Message)
		}
	}
	if p.MinRating > 0 || p.MaxRating > 0 {
		if principal.Anonymous {
			return Denied(ReasonLoginRequired, p.Message)
		}
		if p.MinRating > 0 && principal.Rating < p.MinRating {
			return Denied(ReasonRating, p.Message)
		}
		if p.MaxRating > 0 && principal.Rating > p.MaxRating {
			return Denied(ReasonRating, p.Message)
		}
	}
	return Allowed
}

// NeedsRating reports whether deciding for f/p requires the caller's rating.
// The resolver uses this to avoid a profile query on the hot paths.
func NeedsRating(p Policy) bool {
	p = Normalize(p)
	return p.Enabled && (p.MinRating > 0 || p.MaxRating > 0)
}

// registry is the single source of truth for gateable features.
//
// Adding a feature here is all that is needed for it to appear in the admin
// panel and in /api/features. Wire the corresponding gate with middleware
// .FeatureGate(feature.KeyX) (and set APIEnforced) for server-side enforcement.
var registry = []Feature{
	// ── Core: the parts of an online judge that cannot really be optional,
	// but that an operator may still want to close during maintenance. ──
	{
		Key: "problems", Label: "Problem archive", Category: CategoryCore,
		Description: "Browsing problems and their statements.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "submissions", Label: "Submissions", Category: CategoryCore,
		Description: "Submitting solutions, custom runs and viewing submissions.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "contests", Label: "Contests", Category: CategoryCore,
		Description: "Contest listing, registration, scoreboards and contest problems.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "rankings", Label: "Rankings", Category: CategoryCore,
		Description: "The global rating leaderboard.",
		Default:     Unrestricted, APIEnforced: true,
	},

	// ── Practice ──
	{
		Key: "gym", Label: "Gym", Category: CategoryPractice,
		Description: "Practice contests and gym problem sets.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "practice", Label: "Practice page", Category: CategoryPractice,
		Description: "The personalised practice dashboard.",
		Default:     Unrestricted, RequiresLogin: true,
	},
	{
		Key: "virtual_contests", Label: "Virtual contests", Category: CategoryPractice,
		Description: "Running a past contest against the clock.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},
	{
		Key: "ide", Label: "IDE", Category: CategoryPractice,
		Description: "The standalone code editor and runner.",
		Default:     Unrestricted,
	},
	{
		Key: "recommendations", Label: "Recommendations", Category: CategoryPractice,
		Description: "Suggested problems based on rating and history.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},

	// ── Community ──
	{
		Key: "blog", Label: "Blog", Category: CategorySocial,
		Description: "Community posts, comments and votes.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "editorials", Label: "Editorials", Category: CategorySocial,
		Description: "Problem editorials and solution write-ups.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "groups", Label: "Groups", Category: CategorySocial,
		Description: "Groups, invitations and group contest collections.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "teams", Label: "Teams", Category: CategorySocial,
		Description: "Team creation, invitations and team contests.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "organizations", Label: "Organizations", Category: CategorySocial,
		Description: "Organizations and their member management.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "classes", Label: "Classes", Category: CategorySocial,
		Description: "Classes inside an organization, with join codes.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "training", Label: "Training plans", Category: CategorySocial,
		Description: "Structured training plans and enrolment.",
		Default:     Unrestricted, APIEnforced: true,
	},

	// ── Competition extras ──
	{
		Key: "hacks", Label: "Hacking", Category: CategoryCompete,
		Description: "Submitting counter-tests against other contestants' solutions.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},
	{
		Key: "plagiarism", Label: "Plagiarism reports", Category: CategoryCompete,
		Description: "Similarity checks and the plagiarism report for a contest.",
		Default:     Unrestricted, APIEnforced: true,
	},

	// ── Platform ──
	{
		Key: "notifications", Label: "Notifications", Category: CategoryPlatform,
		Description: "The notification centre and its preferences.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},
	{
		Key: "api_keys", Label: "API keys", Category: CategoryPlatform,
		Description: "Personal API keys for programmatic access.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},
	{
		Key: "webhooks", Label: "Webhooks", Category: CategoryPlatform,
		Description: "Outbound webhooks on judge events.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},
	{
		Key: "media_uploads", Label: "Media uploads", Category: CategoryPlatform,
		Description: "Serving and uploading images used in statements and posts.",
		Default:     Unrestricted, APIEnforced: true,
	},
	{
		Key: "vjudge", Label: "Remote judge", Category: CategoryPlatform,
		Description: "Submitting to and importing from other online judges.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},

	// ── Authoring ──
	{
		Key: "setter_workspace", Label: "Setter workspace", Category: CategoryAuthor,
		Description: "Problem and contest authoring: create, edit, testcases, permissions and imports.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},
	{
		Key: "ai_generation", Label: "AI problem generation", Category: CategoryAuthor,
		Description: "Generating problems, statements and editorials with a model.",
		Default:     Unrestricted, RequiresLogin: true, APIEnforced: true,
	},
	{
		// Importing pulls third-party problems into the set, so who may do it is
		// an admin decision rather than a side effect of holding the setter role.
		// Defaults to setters; an admin can narrow it to only themselves by
		// clearing the role list, or open it to everyone, from the features
		// panel. Admins always pass, per feature.Decide.
		Key: "problem_import", Label: "Problem import", Category: CategoryAuthor,
		Description: "Importing problems from Codeforces, CSES, AtCoder, Toph and QOJ.",
		Default:     Policy{Enabled: true, Roles: []string{model.RoleSetter}}, RequiresLogin: true, APIEnforced: true,
	},
	{
		// Voiding a submission is an accusation: it says a submission should not
		// count, and it moves a public leaderboard. So it is an admin decision
		// who may do it, not a side effect of holding the setter role. Defaults
		// to setters; an admin can narrow it to only themselves, or open it to
		// everyone, from the feature panel. Admins always pass, per feature.Decide.
		//
		// The role list is the whole gate. Being permitted is still not
		// sufficient: the caller must also be able to manage the submission's
		// problem (its owner or co-author) or the round it was made in (manager
		// or judge). See SubmissionHandler.canSkip.
		Key: "skip_submission", Label: "Void a submission", Category: CategoryCompete,
		Description: "Marking a submission as skipped so it stops counting as an attempt: a resubmit-rule breach or a copied solution.",
		Default:     Policy{Enabled: true, Roles: []string{model.RoleSetter}}, RequiresLogin: true, APIEnforced: true,
	},
}

// All returns every declared feature, in registry order.
func All() []Feature {
	out := make([]Feature, len(registry))
	copy(out, registry)
	return out
}

// Lookup returns the feature with key.
func Lookup(key string) (Feature, bool) {
	for _, f := range registry {
		if f.Key == key {
			return f, true
		}
	}
	return Feature{}, false
}

// Keys returns every declared feature key.
func Keys() []string {
	out := make([]string, 0, len(registry))
	for _, f := range registry {
		out = append(out, f.Key)
	}
	return out
}

// Feature keys, as constants so the router and handlers cannot drift from the
// registry through a typo.
const (
	KeyProblems        = "problems"
	KeySubmissions     = "submissions"
	KeyContests        = "contests"
	KeyRankings        = "rankings"
	KeyGym             = "gym"
	KeyPractice        = "practice"
	KeyVirtualContests = "virtual_contests"
	KeyIDE             = "ide"
	KeyRecommendations = "recommendations"
	KeyBlog            = "blog"
	KeyEditorials      = "editorials"
	KeyGroups          = "groups"
	KeyTeams           = "teams"
	KeyOrganizations   = "organizations"
	KeyClasses         = "classes"
	KeyTraining        = "training"
	KeyHacks           = "hacks"
	KeyPlagiarism      = "plagiarism"
	KeyNotifications   = "notifications"
	KeyAPIKeys         = "api_keys"
	KeyWebhooks        = "webhooks"
	KeyMediaUploads    = "media_uploads"
	KeyVJudge          = "vjudge"
	KeySetterWorkspace = "setter_workspace"
	KeyAIGeneration    = "ai_generation"
	KeyProblemImport   = "problem_import"
	KeySkipSubmission  = "skip_submission"
)
