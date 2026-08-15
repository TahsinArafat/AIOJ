package vjudge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

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

type Service struct {
	mu              sync.RWMutex
	bots            map[string]Bot
	subStore        store.SubmissionStore
	probStore       store.ProblemStore
	botAccStore     store.BotAccountStore
	remoteLangStore store.RemoteLanguageStore
	pollCancel      context.CancelFunc
	submitCancel    context.CancelFunc
	activeSubmits   sync.Map
}

func NewService(subStore store.SubmissionStore, probStore store.ProblemStore, botAccStore store.BotAccountStore, remoteLangStore store.RemoteLanguageStore) *Service {
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

func (s *Service) ValidateCookies(ctx context.Context, cookies map[string]string) error {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 15 * time.Second, Jar: jar}
	cfURL, _ := neturl.Parse("https://codeforces.com")
	jarCookies := make([]*http.Cookie, 0, len(cookies))
	for name, value := range cookies {
		jarCookies = append(jarCookies, &http.Cookie{Name: name, Value: value, Domain: "codeforces.com", Path: "/"})
	}
	client.Jar.SetCookies(cfURL, jarCookies)
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://codeforces.com/problemset/status?my=on", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "Codeforces: Enter") || strings.Contains(string(body), "handleOrEmail") {
		return fmt.Errorf("cookies expired or invalid - not logged in")
	}
	return nil
}

func (s *Service) Submit(ctx context.Context, req SubmitRequest) error {
	s.subStore.UpdateStatus(ctx, req.ID, model.StatusPending)

	sourceCode := req.SourceCode + "\n// AIOJ:" + req.ID + "\n"
	if s.remoteLangStore != nil {
		langs, err := s.remoteLangStore.ListByPlatform(context.Background(), req.RemoteOJ)
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

		remoteID, err := bot.Submit(context.Background(), req.ProblemRemoteID, sourceCode, req.Language)
		if err != nil {
			slog.Error("bot submit failed, trying next", "bot", acc.PlatformUser, "err", err)
			s.botAccStore.IncrementFailures(ctx, acc.ID)
			lastErr = err
			continue
		}

		remoteURL := ""
		if req.RemoteOJ == "codeforces" {
			parts := strings.Split(req.ProblemRemoteID, "/")
			if len(parts) > 0 {
				remoteURL = fmt.Sprintf("https://codeforces.com/contest/%s/submission/%s", parts[0], remoteID)
			}
		}
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
	for platform := range s.bots {
		go s.pollWorker(ctx, platform)
	}
	slog.Info("vjudge poll workers started")
}

func (s *Service) StartSubmitWorkers() {
	ctx, cancel := context.WithCancel(context.Background())
	s.submitCancel = cancel
	go s.submitWorker(ctx)
	slog.Info("vjudge submit workers started")
}

func (s *Service) submitWorker(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processUnsubmitted(ctx)
		}
	}
}

func (s *Service) processUnsubmitted(ctx context.Context) {
	subs, err := s.subStore.GetUnsubmittedRemoteSubmissions(ctx)
	if err != nil || len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		if _, loaded := s.activeSubmits.LoadOrStore(sub.ID, true); loaded {
			continue
		}

		go func(sub model.Submission) {
			defer s.activeSubmits.Delete(sub.ID)

			prob, err := s.probStore.GetByID(ctx, sub.ProblemID)
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
			err = s.Submit(ctx, vjReq)
			if err != nil {
				slog.Error("async submit: remote submission failed", "sub", sub.ID, "err", err)
				s.subStore.UpdateStatus(ctx, sub.ID, model.StatusSE)
			}
		}(sub)
	}
}

func (s *Service) ForcePoll(ctx context.Context, platform, remoteID, submissionID string) {
	s.mu.RLock()
	bot, botOK := s.bots[platform]
	s.mu.RUnlock()

	if !botOK {
		return
	}

	if platform == "codeforces" {
		accounts, _ := s.botAccStore.ListByPlatform(ctx, platform)
		for _, acc := range accounts {
			if acc.Status != "active" || acc.PlatformUser == "" {
				continue
			}
			u := fmt.Sprintf("https://codeforces.com/api/user.status?handle=%s&count=20", acc.PlatformUser)
			resp, err := http.Get(u)
			if err != nil {
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var parsed struct {
				Status string                   `json:"status"`
				Result []map[string]interface{} `json:"result"`
			}
			if json.Unmarshal(body, &parsed) != nil || parsed.Status != "OK" {
				continue
			}
			for _, sub := range parsed.Result {
				if fmt.Sprintf("%.0f", sub["id"]) == remoteID {
					verdict := sub["verdict"]
					if verdict == nil || verdict == "TESTING" {
						slog.Info("vjudge force poll: still judging", "sub", submissionID)
						return
					}
					var status model.SubmissionStatus
					switch verdict {
					case "OK": status = model.StatusAC
					case "WRONG_ANSWER": status = model.StatusWA
					case "TIME_LIMIT_EXCEEDED": status = model.StatusTLE
					case "COMPILATION_ERROR": status = model.StatusCE
					case "RUNTIME_ERROR": status = model.StatusRE
					case "MEMORY_LIMIT_EXCEEDED": status = model.StatusMLE
					default: status = model.StatusWA
					}
					timeUsed := 0
					memUsed := 0
					if t, ok := sub["timeConsumedMillis"].(float64); ok { timeUsed = int(t) }
					if m, ok := sub["memoryConsumedBytes"].(float64); ok { memUsed = int(m / 1024) }
					s.subStore.UpdateResult(ctx, submissionID, status, 0, timeUsed, memUsed, "", nil)
					slog.Info("vjudge judged", "sub", submissionID, "verdict", verdict, "time", timeUsed, "mem", memUsed)
					return
				}
			}
		}
		return
	}

	result, err := bot.Poll(ctx, remoteID)
	if err != nil || result == nil || !result.Done {
		if err != nil { slog.Error("vjudge force poll", "err", err) }
		return
	}
	var status model.SubmissionStatus
	switch result.Verdict {
	case "AC": status = model.StatusAC
	case "WA": status = model.StatusWA
	case "TLE": status = model.StatusTLE
	case "RE": status = model.StatusRE
	case "CE": status = model.StatusCE
	default: status = model.StatusWA
	}
	s.subStore.UpdateResult(ctx, submissionID, status, 0, result.TimeUsed, result.MemoryUsed, "", nil)
	slog.Info("vjudge judged", "sub", submissionID, "verdict", result.Verdict)
}

func (s *Service) pollWorker(ctx context.Context, platform string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollPlatform(ctx, platform)
		}
	}
}

func (s *Service) pollPlatform(ctx context.Context, platform string) {
	s.mu.RLock()
	bot, botOK := s.bots[platform]
	s.mu.RUnlock()

	subs, err := s.subStore.GetPendingRemoteSubmissions(ctx)
	if err != nil || len(subs) == 0 {
		return
	}

	if platform == "codeforces" {
		s.pollCF(ctx, subs)
	} else if botOK {
		s.pollGeneric(ctx, subs, bot)
	}
}

func (s *Service) pollCF(ctx context.Context, subs []model.PendingRemoteSubmission) {
	accounts, err := s.botAccStore.ListByPlatform(ctx, "codeforces")
	if err != nil {
		return
	}
	for _, acc := range accounts {
		if acc.Status != "active" || acc.PlatformUser == "" {
			continue
		}
		u := fmt.Sprintf("https://codeforces.com/api/user.status?handle=%s&count=20", acc.PlatformUser)
		resp, err := http.Get(u)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var parsed struct {
			Status string                   `json:"status"`
			Result []map[string]interface{} `json:"result"`
		}
		if json.Unmarshal(body, &parsed) != nil || parsed.Status != "OK" {
			continue
		}
		for _, sub := range parsed.Result {
			id := fmt.Sprintf("%.0f", sub["id"])
			for _, ps := range subs {
				if ps.RemoteID == id {
					verdict := sub["verdict"]
					if verdict == nil || verdict == "TESTING" {
						continue
					}
					var status model.SubmissionStatus
					switch verdict {
					case "OK": status = model.StatusAC
					case "WRONG_ANSWER": status = model.StatusWA
					case "TIME_LIMIT_EXCEEDED": status = model.StatusTLE
					case "COMPILATION_ERROR": status = model.StatusCE
					case "RUNTIME_ERROR": status = model.StatusRE
					case "MEMORY_LIMIT_EXCEEDED": status = model.StatusMLE
					default: status = model.StatusWA
					}
					timeUsed := 0
					memUsed := 0
					if t, ok := sub["timeConsumedMillis"].(float64); ok { timeUsed = int(t) }
					if m, ok := sub["memoryConsumedBytes"].(float64); ok { memUsed = int(m / 1024) }
					s.subStore.UpdateResult(ctx, ps.ID, status, 0, timeUsed, memUsed, "", nil)
					slog.Info("vjudge judged", "sub", ps.ID, "verdict", verdict, "time", timeUsed, "mem", memUsed)
				}
			}
		}
	}
}

func (s *Service) pollGeneric(ctx context.Context, subs []model.PendingRemoteSubmission, bot Bot) {
	for _, ps := range subs {
		result, err := bot.Poll(ctx, ps.RemoteID)
		if err != nil {
			continue
		}
		if result == nil || !result.Done {
			continue
		}
		var status model.SubmissionStatus
		switch result.Verdict {
		case "AC": status = model.StatusAC
		case "WA": status = model.StatusWA
		case "TLE": status = model.StatusTLE
		case "RE": status = model.StatusRE
		case "CE": status = model.StatusCE
		default: status = model.StatusWA
		}
		s.subStore.UpdateResult(ctx, ps.ID, status, 0, result.TimeUsed, result.MemoryUsed, "", nil)
		slog.Info("vjudge judged", "sub", ps.ID, "verdict", result.Verdict)
	}
}
