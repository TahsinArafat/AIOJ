package vjudge

import (
	"context"
	"fmt"
	"time"
)

type AtCoderBot struct {
	config BotConfig
	state  BotState
}

func NewAtCoderBot(cfg BotConfig) *AtCoderBot {
	return &AtCoderBot{config: cfg, state: StateIdle}
}

func (b *AtCoderBot) Name() string    { return "atcoder" }
func (b *AtCoderBot) State() BotState { return b.state }

func (b *AtCoderBot) Submit(ctx context.Context, problemID, sourceCode, language string) (string, error) {
	b.state = StateRunning
	defer func() { b.state = StateIdle }()
	return fmt.Sprintf("atcoder-%d", time.Now().UnixNano()), nil
}

func (b *AtCoderBot) Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error) {
	return &RemoteResult{RemoteID: remoteSubmissionID, Verdict: "PENDING", Done: false}, nil
}
