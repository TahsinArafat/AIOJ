package vjudge

import (
	"context"
	"fmt"
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

func (b *AtCoderBot) Submit(_ context.Context, _, _, _ string) (string, error) {
	return "", fmt.Errorf("atcoder bot: not yet implemented")
}

func (b *AtCoderBot) Poll(_ context.Context, _ string) (*RemoteResult, error) {
	return nil, fmt.Errorf("atcoder bot: not yet implemented")
}
