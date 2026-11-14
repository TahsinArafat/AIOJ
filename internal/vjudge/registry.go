package vjudge

import (
	"fmt"
	"sort"
	"sync"

	"github.com/tahsinarafat/aioj/internal/store"
)

// BotDeps carries what a bot may need beyond its own config.
//
// Some platforms cannot be driven by HTTP scraping alone and are given a
// dedicated submit client (a small service that holds a real browser session);
// others need to read the per-platform language list from the database. Passing
// them as one struct keeps the factory signature fixed, so registering a bot
// never changes shape when a new platform needs one more collaborator.
type BotDeps struct {
	// CFSubmit and AtCoderSubmit are the optional out-of-process submit
	// clients. Nil means the platform must submit in-process.
	CFSubmit      *CFSubmitClient
	AtCoderSubmit *AtCoderSubmitClient
	// RemoteLang backs the bots that enumerate their platform's languages from
	// the database rather than from a hardcoded list.
	RemoteLang store.RemoteLanguageStore
}

// BotFactory builds one platform's bot. It takes the config by value so a
// factory cannot mutate the caller's copy, and it returns the interface rather
// than a concrete type so the registry holds no platform knowledge at all.
type BotFactory func(cfg BotConfig, deps BotDeps) Bot

var (
	botMu        sync.RWMutex
	botFactories = map[string]BotFactory{}
)

// RegisterBotFactory records how to build one platform's bot.
//
// Panics on a duplicate name, exactly as contest/format does and for the same
// reason: it can only happen from two init() functions claiming one platform, at
// startup, where a panic is the loud and correct outcome. A silent overwrite
// would leave whichever ran last quietly winning.
func RegisterBotFactory(platform string, factory BotFactory) {
	botMu.Lock()
	defer botMu.Unlock()
	if _, ok := botFactories[platform]; ok {
		panic("vjudge bot already registered: " + platform)
	}
	botFactories[platform] = factory
}

// NewBot builds the bot for platform, reporting whether one is registered.
//
// A bool rather than an error because the caller's decision is the same either
// way — skip this platform and carry on — and an unregistered platform is a
// configuration fact, not a failure. NewBotChecked exists for the paths that
// treat it as an error.
func NewBot(platform string, cfg BotConfig, deps BotDeps) (Bot, bool) {
	botMu.RLock()
	factory, ok := botFactories[platform]
	botMu.RUnlock()
	if !ok {
		return nil, false
	}
	return factory(cfg, deps), true
}

// NewBotChecked is NewBot for callers that must distinguish an unregistered
// platform from a working one, such as an admin action reporting why a
// platform cannot be configured.
func NewBotChecked(platform string, cfg BotConfig, deps BotDeps) (Bot, error) {
	bot, ok := NewBot(platform, cfg, deps)
	if !ok {
		return nil, fmt.Errorf("no bot registered for platform: %s", platform)
	}
	return bot, nil
}

// RegisteredPlatforms lists the platforms that have a bot, sorted so a caller
// can present a stable list.
func RegisteredPlatforms() []string {
	botMu.RLock()
	defer botMu.RUnlock()
	out := make([]string, 0, len(botFactories))
	for name := range botFactories {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
