package vjudge

import (
	"context"
	"fmt"
	"time"
)

type TophBot struct {
	config BotConfig
	state  BotState
}

func NewTophBot(cfg BotConfig) *TophBot {
	return &TophBot{config: cfg, state: StateIdle}
}

func (b *TophBot) Name() string    { return "toph" }
func (b *TophBot) State() BotState { return b.state }

func (b *TophBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.state = StateRunning
	defer func() { b.state = StateIdle }()
	return fmt.Sprintf("toph-%d", time.Now().UnixNano()), nil
}

func (b *TophBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	return &RemoteResult{RemoteID: remoteSubmissionID, Verdict: "PENDING", Done: false}, nil
}

func (b *TophBot) Login(ctx context.Context) (map[string]string, error) {
	return nil, fmt.Errorf("toph login not implemented")
}

func (b *TophBot) IsLoggedIn(ctx context.Context) bool {
	return false
}
