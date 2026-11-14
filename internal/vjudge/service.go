package vjudge

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// remoteHTTPTimeout bounds every outbound call this package makes to a remote
// judge. The package previously used http.Get, whose default client has no
// timeout at all: a remote site that accepted the connection and then stalled
// pinned the poll goroutine indefinitely.
const remoteHTTPTimeout = 20 * time.Second

// submitAttemptTimeout bounds one whole submit attempt (all bot accounts in
// turn) so a single unhealthy submission cannot hold a goroutine forever.
const submitAttemptTimeout = 3 * time.Minute

// remoteHTTPClient is the shared client for remote-judge HTTP calls.
var remoteHTTPClient = &http.Client{Timeout: remoteHTTPTimeout}

type SubmitRequest struct {
	ID              string `json:"id"`
	ProblemRemoteID string `json:"problem_remote_id"`
	SourceCode      string `json:"source_code"`
	Language        string `json:"language"`
	RemoteOJ        string `json:"remote_oj"`
}

type RemoteLanguageItem struct {
	ID   string `json:"value"`
	Name string `json:"text"`
}

// remoteSubmissionStore is what the remote-judge hand-off touches: rows still
// needing a remote submit or a poll, and the verdict that comes back. Not the
// local judge queue — this service never claims or fences a row.
type remoteSubmissionStore interface {
	store.SubmissionRemoteJudge
	UpdateStatus(ctx context.Context, id string, status model.SubmissionStatus)
	UpdateResult(ctx context.Context, id string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) error
}

type Service struct {
	mu              sync.RWMutex
	bots            map[string]Bot
	subStore        remoteSubmissionStore
	probStore       problemLookup
	botAccStore     store.BotAccountStore
	remoteLangStore store.RemoteLanguageStore
	pollCancel      context.CancelFunc
	submitCancel    context.CancelFunc
	activeSubmits   sync.Map
}

func NewService(subStore remoteSubmissionStore, probStore store.ProblemStore, botAccStore store.BotAccountStore, remoteLangStore store.RemoteLanguageStore) *Service {
	return &Service{bots: make(map[string]Bot), subStore: subStore, probStore: probStore, botAccStore: botAccStore, remoteLangStore: remoteLangStore}
}

func (s *Service) RegisterBot(name string, bot Bot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bots[name] = bot
	slog.Info("vjudge bot registered", "name", name)
}

func (s *Service) GetBotNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.bots))
	for n := range s.bots {
		names = append(names, n)
	}
	return names
}

func (s *Service) Login(ctx context.Context, platform string) (map[string]string, error) {
	s.mu.RLock()
	bot, ok := s.bots[platform]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no bot for platform: %s", platform)
	}
	return bot.Login(ctx)
}

func (s *Service) SetCookies(platform string, cookies map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if bot, ok := s.bots[platform]; ok {
		bot.SetCookies(cookies)
	}
}

func (s *Service) UpdateCookies(platform string, cookies map[string]string) {
	s.SetCookies(platform, cookies)
}

// ValidateCookies checks a set of cookies for the bot behind platform.
//
// Kept on the service because the admin console reaches it through the service
// handle, but it no longer carries the Codeforces-specific probe: a bot that can
// validate implements CookieValidator and is asked, and a bot that cannot is
// reported as unvalidatable rather than silently accepted.
func (s *Service) ValidateCookies(ctx context.Context, platform string, cookies map[string]string) error {
	s.mu.RLock()
	bot, ok := s.bots[platform]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no bot for platform: %s", platform)
	}
	v, ok := bot.(CookieValidator)
	if !ok {
		return fmt.Errorf("%w: %s", ErrCookieValidationUnsupported, platform)
	}
	return v.ValidateCookies(ctx, cookies)
}

func (s *Service) Submit(ctx context.Context, req SubmitRequest) error {
	s.subStore.UpdateStatus(ctx, req.ID, model.StatusPending)

	sourceCode := req.SourceCode + "\n// AIOJ:" + req.ID + "\n"
	if s.remoteLangStore != nil {
		// Use the caller's context, not Background: a cancelled submit must not
		// leave a query running against the database with no deadline.
		langs, err := s.remoteLangStore.ListByPlatform(ctx, req.RemoteOJ)
		if err == nil {
			for _, l := range langs {
				if l.LocalID == req.Language && l.InlineCommentPrefix != "" {
					sourceCode = req.SourceCode + "\n" + l.InlineCommentPrefix + " AIOJ:" + req.ID + "\n"
					break
				}
			}
		}
	}

	s.mu.RLock()
	bot, ok := s.bots[req.RemoteOJ]
	s.mu.RUnlock()
	if !ok {
		s.subStore.UpdateStatus(ctx, req.ID, model.StatusSE)
		return fmt.Errorf("no bot for remote OJ: %s", req.RemoteOJ)
	}

	accounts, err := s.botAccStore.ListByPlatform(ctx, req.RemoteOJ)
	if err != nil || len(accounts) == 0 {
		s.subStore.UpdateStatus(ctx, req.ID, model.StatusSE)
		return fmt.Errorf("no bot accounts for %s", req.RemoteOJ)
	}

	var lastErr error
	for _, acc := range accounts {
		if acc.Status != "active" || acc.ConsecutiveFailures >= 3 {
			continue
		}

		cfg := BotConfig{
			Username:     acc.PlatformUser,
			Password:     acc.PlatformPass,
			APIKey:       acc.APIKey,
			APISecret:    acc.APISecret,
			Cookies:      acc.SessionData,
			ProxyURL:     acc.ProxyURL,
			ProxyEnabled: acc.ProxyEnabled,
		}

		bot.Configure(cfg)

		// Same reason: the caller's deadline bounds the whole retry loop across
		// bot accounts. Background here meant a stuck bot held the goroutine
		// forever, long after the triggering request had gone.
		remoteID, err := bot.Submit(ctx, req.ProblemRemoteID, sourceCode, req.Language)
		if err != nil {
			slog.Error("bot submit failed, trying next", "bot", acc.PlatformUser, "err", err)
			s.botAccStore.IncrementFailures(ctx, acc.ID)
			lastErr = err
			continue
		}

		// The link back to the source judge, built by the bot because the URL
		// shape is platform knowledge. This used to be a Codeforces-only branch,
		// so every other platform stored an empty remote_url and the submission
		// page offered no way back to the remote judge.
		remoteURL := bot.SubmissionURL(req.ProblemRemoteID, remoteID)
		s.subStore.UpdateRemoteID(ctx, req.ID, remoteID, remoteURL)
		s.subStore.UpdateBotID(ctx, req.ID, acc.ID, acc.PlatformUser)
		s.botAccStore.ResetFailures(ctx, acc.ID)
		s.botAccStore.MarkUsed(ctx, acc.ID)
		slog.Info("vjudge submitted via bot", "sub", req.ID, "bot", acc.PlatformUser, "remote", remoteID)
		return nil
	}

	s.subStore.UpdateStatus(ctx, req.ID, model.StatusSE)
	if lastErr != nil {
		return fmt.Errorf("all bots failed for %s: %w", req.RemoteOJ, lastErr)
	}
	return fmt.Errorf("no healthy bot available for %s", req.RemoteOJ)
}

func (s *Service) FetchLanguages(ctx context.Context, platform string) ([]RemoteLanguageItem, error) {
	s.mu.RLock()
	bot, ok := s.bots[platform]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no bot for platform: %s", platform)
	}
	langs, err := bot.FetchLanguages(ctx)
	if err != nil {
		return nil, err
	}
	if langs == nil {
		return nil, fmt.Errorf("submit client not configured for platform: %s", platform)
	}
	return langs, nil
}

func (s *Service) StartPollWorkers() {
	ctx, cancel := context.WithCancel(context.Background())
	s.pollCancel = cancel
	started := 0
	for platform := range s.bots {
		// Skip platforms that have no active bot account configured.  The poll
		// worker would hit GetPendingRemoteSubmissions every few seconds for
		// nothing, and the submit worker would similarly query in vain.  If an
		// account is added later the process must be restarted — bot accounts
		// are admin-only configuration, not runtime user data.
		accounts, err := s.botAccStore.ListByPlatform(ctx, platform)
		if err != nil || !hasActiveBotAccount(accounts) {
			slog.Info("vjudge: skipping poll worker — no active bot account", "platform", platform)
			continue
		}
		go s.pollWorker(ctx, platform)
		started++
	}
	slog.Info("vjudge poll workers started", "count", started)
}

// hasActiveBotAccount reports whether any account in the slice is marked active
// and has credentials configured.
func hasActiveBotAccount(accounts []model.BotAccount) bool {
	for _, a := range accounts {
		if a.Status == "active" && a.PlatformUser != "" {
			return true
		}
	}
	return false
}

func (s *Service) StartSubmitWorkers() {
	ctx, cancel := context.WithCancel(context.Background())
	s.submitCancel = cancel
	go s.submitWorker(ctx)
	slog.Info("vjudge submit workers started")
}

func (s *Service) submitWorker(ctx context.Context) {
	// Exponential backoff: start at 3 s, double on each empty poll up to
	// 60 s, reset immediately when work is found.  This cuts the DB round-
	// trips from ~1 200/hr (fixed 3 s) to at most 60/hr when idle — a 20×
	// reduction at no cost to submit latency when submissions arrive.
	const (
		minInterval = 3 * time.Second
		maxInterval = 60 * time.Second
	)
	interval := minInterval
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			found := s.processUnsubmitted(ctx)
			if found {
				interval = minInterval
			} else if interval < maxInterval {
				interval *= 2
				if interval > maxInterval {
					interval = maxInterval
				}
			}
		}
	}
}

func (s *Service) processUnsubmitted(ctx context.Context) bool {
	subs, err := s.subStore.GetUnsubmittedRemoteSubmissions(ctx)
	if err != nil || len(subs) == 0 {
		return false
	}

	for _, sub := range subs {
		if _, loaded := s.activeSubmits.LoadOrStore(sub.ID, true); loaded {
			continue
		}

		go func(sub model.Submission) {
			defer s.activeSubmits.Delete(sub.ID)

			// One submission's submit attempt gets its own deadline. Without it
			// a single stalled remote site kept a goroutine (and its activeSubmits
			// entry) alive for the life of the process.
			subCtx, cancel := context.WithTimeout(ctx, submitAttemptTimeout)
			defer cancel()

			prob, err := s.probStore.GetByID(subCtx, sub.ProblemID)
			if err != nil || prob == nil {
				slog.Error("async submit: problem not found", "sub", sub.ID, "prob", sub.ProblemID)
				s.subStore.UpdateStatus(ctx, sub.ID, model.StatusSE)
				return
			}

			vjReq := SubmitRequest{
				ID:              sub.ID,
				ProblemRemoteID: prob.RemoteID,
				SourceCode:      sub.SourceCode,
				Language:        sub.Language,
				RemoteOJ:        prob.Source,
			}

			slog.Info("async submit: submitting to remote OJ", "sub", sub.ID, "platform", prob.Source, "remote_id", prob.RemoteID)
			err = s.Submit(subCtx, vjReq)
			if err != nil {
				slog.Error("async submit: remote submission failed", "sub", sub.ID, "err", err)
				// Record the failure with a background context: subCtx is very
				// likely the thing that just timed out, so reusing it here would
				// drop the status update and leave the row pending forever.
				s.subStore.UpdateStatus(context.WithoutCancel(ctx), sub.ID, model.StatusSE)
			}
		}(sub)
	}
	return true
}

// ForcePoll asks a remote judge about one submission right now, bypassing the
// poll worker's backoff. The admin console and the "refresh" button on a
// submission both call it.
//
// Goes through the same Bot seam as the workers. It used to special-case
// Codeforces with a hand-rolled user.status fetch, which meant the one backend
// that had a Bot implementation was also the one reached without it.
func (s *Service) ForcePoll(ctx context.Context, platform, remoteID, submissionID string) {
	s.mu.RLock()
	bot, botOK := s.bots[platform]
	s.mu.RUnlock()
	if !botOK {
		return
	}

	// A batch-capable bot is worth asking even for a single submission: on
	// Codeforces the batch call is one request against the account list, where
	// Poll would be one request per account until the right one answers.
	if batch, ok := bot.(BatchPoller); ok {
		accounts, err := s.botAccStore.ListByPlatform(ctx, platform)
		if err == nil {
			want := []PendingResult{{SubmissionID: submissionID, RemoteID: remoteID}}
			for _, acc := range accounts {
				if acc.Status != "active" || acc.PlatformUser == "" {
					continue
				}
				results, err := batch.PollBatch(ctx, acc.PlatformUser, want)
				if err != nil {
					continue
				}
				if r, ok := results[remoteID]; ok && r != nil && r.Done {
					s.finishRemote(ctx, submissionID, r)
					return
				}
			}
			// Nothing settled yet. Still judging is not an error, and the
			// caller's poll worker will ask again.
			return
		}
	}

	result, err := bot.Poll(ctx, remoteID)
	if err != nil {
		slog.Error("vjudge force poll", "platform", platform, "err", err)
		return
	}
	if result == nil || !result.Done {
		return
	}
	s.finishRemote(ctx, submissionID, result)
}

func (s *Service) pollWorker(ctx context.Context, platform string) {
	// Same exponential-backoff pattern as submitWorker: start at 5 s, back
	// off to 60 s when no pending remote submissions exist, reset on work.
	const (
		minInterval = 5 * time.Second
		maxInterval = 60 * time.Second
	)
	interval := minInterval
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			found := s.pollPlatform(ctx, platform)
			if found {
				interval = minInterval
			} else if interval < maxInterval {
				interval *= 2
				if interval > maxInterval {
					interval = maxInterval
				}
			}
		}
	}
}

func (s *Service) pollPlatform(ctx context.Context, platform string) bool {
	s.mu.RLock()
	bot, botOK := s.bots[platform]
	s.mu.RUnlock()
	if !botOK {
		return false
	}

	subs, err := s.subStore.GetPendingRemoteSubmissions(ctx)
	if err != nil || len(subs) == 0 {
		return false
	}

	// Only this platform's own submissions. A batch poller asks one remote
	// account about its submissions, so handing it another platform's ids would
	// waste the request and, worse, invite a false match on an id that happens
	// to collide across judges.
	mine := make([]model.PendingRemoteSubmission, 0, len(subs))
	for _, ps := range subs {
		if ps.Platform == platform {
			mine = append(mine, ps)
		}
	}
	if len(mine) == 0 {
		return false
	}

	// A bot that can report many verdicts per request is asked to; the rest are
	// polled one at a time. This replaces a test on the platform NAME, which is
	// why the capability is an interface instead: adding a batch-capable judge
	// is now implementing BatchPoller, not editing this function.
	if batch, ok := bot.(BatchPoller); ok {
		if s.pollBatch(ctx, platform, batch, mine) {
			return true
		}
		// The batch call failed as a whole. Fall through to per-submission
		// polling rather than reporting no progress: one malformed response
		// must not stall every pending submission on the platform.
	}
	s.pollOneByOne(ctx, bot, mine)
	return true
}

// pollBatch asks a BatchPoller about every pending submission, once per active
// account, and writes back what it learned. Returns false when no account could
// answer, so the caller can fall back.
func (s *Service) pollBatch(ctx context.Context, platform string, batch BatchPoller, subs []model.PendingRemoteSubmission) bool {
	accounts, err := s.botAccStore.ListByPlatform(ctx, platform)
	if err != nil {
		return false
	}

	want := make([]PendingResult, 0, len(subs))
	for _, ps := range subs {
		want = append(want, PendingResult{SubmissionID: ps.ID, RemoteID: ps.RemoteID})
	}
	byRemote := make(map[string]model.PendingRemoteSubmission, len(subs))
	for _, ps := range subs {
		byRemote[ps.RemoteID] = ps
	}

	answered := false
	for _, acc := range accounts {
		if acc.Status != "active" || acc.PlatformUser == "" {
			continue
		}
		results, err := batch.PollBatch(ctx, acc.PlatformUser, want)
		if err != nil {
			slog.Warn("vjudge batch poll failed", "platform", platform, "handle", acc.PlatformUser, "err", err)
			continue
		}
		answered = true
		for remoteID, r := range results {
			ps, ok := byRemote[remoteID]
			if !ok || r == nil || !r.Done {
				continue
			}
			s.finishRemote(ctx, ps.ID, r)
			// One account reporting a verdict is enough; the others are only
			// queried because the submission may have been made by any of them.
			delete(byRemote, remoteID)
		}
		if len(byRemote) == 0 {
			break
		}
	}
	return answered
}

// pollOneByOne is the generic path: one request per pending submission.
func (s *Service) pollOneByOne(ctx context.Context, bot Bot, subs []model.PendingRemoteSubmission) {
	for _, ps := range subs {
		result, err := bot.Poll(ctx, ps.RemoteID)
		if err != nil || result == nil || !result.Done {
			continue
		}
		s.finishRemote(ctx, ps.ID, result)
	}
}

// finishRemote records one remote verdict against a local submission. The only
// place in this package that turns a verdict into a stored status, so the
// mapping cannot drift between the batch and one-by-one paths.
func (s *Service) finishRemote(ctx context.Context, submissionID string, r *RemoteResult) {
	s.subStore.UpdateResult(ctx, submissionID, r.Verdict.Status(), 0, r.TimeUsed, r.MemoryUsed, "", nil)
	slog.Info("vjudge judged", "sub", submissionID, "verdict", r.Verdict,
		"time", r.TimeUsed, "mem", r.MemoryUsed)
}

// problemLookup is the one problem query the remote judge makes: resolve the
// local problem a remote submission belongs to.
type problemLookup interface {
	GetByID(ctx context.Context, id string) (*model.Problem, error)
}
