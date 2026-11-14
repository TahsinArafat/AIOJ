package vjudge

import (
	"context"
	"errors"
)

type BotState string

const (
	StateIdle    BotState = "idle"
	StateRunning BotState = "running"
	StateError   BotState = "error"
)

type RemoteResult struct {
	RemoteID string `json:"remote_id"`
	// Verdict is the platform's answer, normalized. Typed rather than a string
	// so a bot cannot invent a spelling the mapper has never seen, and so
	// translating it to a stored status is Verdict.Status's job alone.
	Verdict    Verdict `json:"verdict"`
	Status     string  `json:"status"`
	TimeUsed   int     `json:"time_used_ms"`
	MemoryUsed int     `json:"memory_used_kb"`
	Score      float64 `json:"score"`
	Done       bool    `json:"done"`
}

type Bot interface {
	Name() string
	Submit(ctx context.Context, problemID, sourceCode, language string) (string, error)
	Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error)
	State() BotState
	Login(ctx context.Context) (map[string]string, error)
	IsLoggedIn(ctx context.Context) bool

	// Configure applies account credentials/settings to the bot before a
	// submission attempt.  Implementations that do not support session-based
	// configuration may treat this as a no-op.
	Configure(cfg BotConfig)

	// SetCookies injects session cookies into the bot's HTTP client.
	// Implementations that do not use cookies may treat this as a no-op.
	SetCookies(cookies map[string]string)

	// FetchLanguages retrieves the list of languages supported by the remote
	// OJ for this bot.  Implementations that cannot enumerate languages
	// should return (nil, nil).
	FetchLanguages(ctx context.Context) ([]RemoteLanguageItem, error)

	// SubmissionURL builds the browser-facing URL for a submission this bot
	// accepted, or "" when the platform has no stable per-submission page.
	//
	// This belongs on the bot because the URL shape is platform knowledge and
	// nothing else has it. The service used to hardcode one: it special-cased
	// platform == "codeforces" and left every other platform's remote_url
	// empty, so a Toph or QOJ submission offered no link back to the source
	// judge. problemRemoteID is the value the problem was imported with, which
	// some platforms need to address the submission.
	SubmissionURL(problemRemoteID, remoteID string) string
}

// BatchPoller is an optional Bot capability: a platform whose API can report
// the status of many submissions in one request.
//
// Codeforces offers /api/user.status, which returns the handle's recent
// submissions with verdicts for all of them. Polling that once per account is
// far cheaper than a request per submission, which is why the service used to
// branch on `platform == "codeforces"`. That branch is gone: a bot that can
// batch implements this interface, and the service asks whether it does.
//
// The capability is optional rather than part of Bot because four of the five
// platforms genuinely cannot do it; requiring a no-op implementation from each
// would be ceremony that hides which one can.
type BatchPoller interface {
	// PollBatch reports the verdict for every submission in want that the
	// platform can currently resolve, keyed by remoteID. Entries still being
	// judged, and ids the platform does not know, are simply absent. A
	// non-nil error means the whole batch failed and the caller should fall
	// back to per-submission polling.
	//
	// handle is the remote account whose submissions should be searched, passed
	// in rather than taken from the bot's own config because one bot serves
	// every account on its platform: the caller walks the account list and asks
	// this bot about each handle. Passing it as an argument is also what keeps
	// the call from mutating shared bot state, which is what a Configure-per-
	// account loop would have done.
	PollBatch(ctx context.Context, handle string, want []PendingResult) (map[string]*RemoteResult, error)
}

// CookieValidator is an optional Bot capability: a platform whose session
// cookies can be checked before they are stored.
//
// Codeforces can, by loading a page that requires a session and looking for the
// login form. The others cannot without a per-platform probe, so this is a
// capability rather than part of Bot — and it is what the admin console's
// validation ladder should test instead of the platform name.
// ErrCookieValidationUnsupported reports that a platform has no way to check
// cookies before they are stored. Callers that store them anyway should say so
// rather than reporting a validation that did not happen.
var ErrCookieValidationUnsupported = errors.New("cookie validation unsupported for this platform")

type CookieValidator interface {
	ValidateCookies(ctx context.Context, cookies map[string]string) error
}

// PendingResult is one submission a BatchPoller is asked about.
type PendingResult struct {
	// SubmissionID is this server's own id, carried so a caller can report
	// which row a result belongs to without a second lookup.
	SubmissionID string
	// RemoteID is the id the remote judge assigned.
	RemoteID string
}

type BotConfig struct {
	Username     string            `yaml:"username"`
	Password     string            `yaml:"password"`
	APIKey       string            `yaml:"api_key,omitempty"`
	APISecret    string            `yaml:"api_secret,omitempty"`
	BaseURL      string            `yaml:"base_url,omitempty"`
	Cookies      map[string]string `yaml:"cookies,omitempty"`
	UserAgent    string            `yaml:"user_agent,omitempty"`
	ProxyURL     string            `yaml:"proxy_url,omitempty"`
	ProxyEnabled bool              `yaml:"proxy_enabled,omitempty"`
}
