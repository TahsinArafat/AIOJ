package vjudge

import (
	"context"
	"fmt"
	"log/slog"
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

type Service struct {
	mu          sync.RWMutex
	bots        map[string]Bot
	subStore    store.SubmissionStore
	botAccStore store.BotAccountStore
}

func NewService(subStore store.SubmissionStore, botAccStore store.BotAccountStore) *Service {
	return &Service{bots: make(map[string]Bot), subStore: subStore, botAccStore: botAccStore}
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

func (s *Service) Submit(ctx context.Context, req SubmitRequest) error {
	s.mu.RLock()
	bot, ok := s.bots[req.RemoteOJ]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no bot for remote OJ: %s", req.RemoteOJ)
	}
	remoteID, err := bot.Submit(ctx, req.ProblemRemoteID, req.SourceCode, req.Language)
	if err != nil {
		s.subStore.UpdateStatus(ctx, req.ID, model.StatusSE)
		return fmt.Errorf("submit to %s: %w", req.RemoteOJ, err)
	}
	go s.poll(req.ID, remoteID, bot)
	return nil
}

func (s *Service) poll(submissionID, remoteID string, bot Bot) {
	ctx := context.Background()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-ticker.C:
			result, err := bot.Poll(ctx, remoteID)
			if err != nil {
				slog.Error("vjudge poll", "sub", submissionID, "err", err)
				continue
			}
			if result == nil || !result.Done {
				continue
			}
			var status model.SubmissionStatus
			switch result.Verdict {
			case "AC":
				status = model.StatusAC
			case "WA":
				status = model.StatusWA
			case "TLE":
				status = model.StatusTLE
			case "RE":
				status = model.StatusRE
			case "CE":
				status = model.StatusCE
			default:
				status = model.StatusWA
			}
			s.subStore.UpdateResult(ctx, submissionID, status, 0, result.TimeUsed, result.MemoryUsed, "", nil)
			slog.Info("vjudge judged", "sub", submissionID, "verdict", result.Verdict)
			return
		case <-timeout:
			slog.Warn("vjudge poll timeout", "sub", submissionID)
			return
		}
	}
}
