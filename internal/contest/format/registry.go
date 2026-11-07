package format

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type FormatFactory func(config json.RawMessage) (ContestFormat, error)

var (
	mu        sync.RWMutex
	factories = map[string]FormatFactory{}
)

// Register adds a format factory. A duplicate registration is a programming
// error and still panics: it can only happen from two init() functions racing
// for the same name, at startup, where a panic is the loud, correct outcome.
func Register(name string, factory FormatFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := factories[name]; ok {
		panic("contest format already registered: " + name)
	}
	factories[name] = factory
}

func Get(name string) (FormatFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := factories[name]
	return f, ok
}

// ErrUnknownFormat is returned when a format name has no registered factory.
var ErrUnknownFormat = errors.New("unknown contest format")

func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for n := range factories {
		names = append(names, n)
	}
	return names
}

// Create builds the format for name. An unknown name is an error, never a
// panic: the name comes from stored contest data, so it must be able to fail
// per request rather than take the process down.
func Create(name string, config json.RawMessage) (ContestFormat, error) {
	factory, ok := Get(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownFormat, name)
	}
	return factory(config)
}
