package vjudge

import (
	"context"
	"fmt"
	"time"
)

type CSESBot struct {
	config BotConfig
	state  BotState
}

func NewCSESBot(cfg BotConfig) *CSESBot {
	return &CSESBot{config: cfg, state: StateIdle}
}

func (b *CSESBot) Name() string    { return "cses" }
func (b *CSESBot) State() BotState { return b.state }

func (b *CSESBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.state = StateRunning
	defer func() { b.state = StateIdle }()
	return fmt.Sprintf("cses-%d", time.Now().UnixNano()), nil
}

func (b *CSESBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	return &RemoteResult{RemoteID: remoteSubmissionID, Verdict: "PENDING", Done: false}, nil
}
