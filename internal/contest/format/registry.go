package format

import (
	"encoding/json"
	"fmt"
	"sync"
)

type FormatFactory func(config json.RawMessage) (ContestFormat, error)

var (
	mu        sync.RWMutex
	factories = map[string]FormatFactory{}
)

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

func MustGet(name string) FormatFactory {
	f, ok := Get(name)
	if !ok {
		panic("contest format not found: " + name)
	}
	return f
}

func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for n := range factories {
		names = append(names, n)
	}
	return names
}

func Create(name string, config json.RawMessage) (ContestFormat, error) {
	factory, ok := Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown contest format: %s", name)
	}
	return factory(config)
}

func MustCreate(name string, config json.RawMessage) ContestFormat {
	f, err := Create(name, config)
	if err != nil {
		panic(err)
	}
	return f
}
