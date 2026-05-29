package vjudge

import (
	"context"
	"fmt"
	"time"
)

type QOJBot struct {
	config BotConfig
	state  BotState
}

func NewQOJBot(cfg BotConfig) *QOJBot {
	return &QOJBot{config: cfg, state: StateIdle}
}

func (b *QOJBot) Name() string    { return "qoj" }
func (b *QOJBot) State() BotState { return b.state }

func (b *QOJBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.state = StateRunning
	defer func() { b.state = StateIdle }()
	return fmt.Sprintf("qoj-%d", time.Now().UnixNano()), nil
}

func (b *QOJBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	return &RemoteResult{RemoteID: remoteSubmissionID, Verdict: "PENDING", Done: false}, nil
}
