package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/arcgolabs/configx"
	"github.com/samber/oops"
	"github.com/spf13/pflag"
)

const fallbackWatchInterval = time.Second

type Watcher struct {
	mu       sync.RWMutex
	current  Config
	handlers []func(Config, error)
	start    func(context.Context) error
	close    func() error
}

func NewWatcherFromFlags(flags *pflag.FlagSet, configFile string, opts ...configx.Option) (*Watcher, error) {
	if configFile != "" {
		return newConfigxWatcher(flags, configFile, opts...)
	}
	return newPollingWatcher(flags, configFile, opts...)
}

func (w *Watcher) Config() Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current
}

func (w *Watcher) OnChange(fn func(Config, error)) {
	if fn == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers = append(w.handlers, fn)
}

func (w *Watcher) Start(ctx context.Context) error {
	if w == nil || w.start == nil {
		<-ctx.Done()
		return fmt.Errorf("agent config watcher stopped: %w", ctx.Err())
	}
	return w.start(ctx)
}

func (w *Watcher) Close() error {
	if w == nil || w.close == nil {
		return nil
	}
	return w.close()
}

func (w *Watcher) update(cfg Config) {
	w.mu.Lock()
	w.current = cfg
	w.mu.Unlock()
}

func (w *Watcher) notify(cfg Config, err error) {
	w.mu.RLock()
	handlers := append([]func(Config, error){}, w.handlers...)
	w.mu.RUnlock()

	for _, handler := range handlers {
		handler(cfg, err)
	}
}

func newConfigxWatcher(flags *pflag.FlagSet, configFile string, opts ...configx.Option) (*Watcher, error) {
	loadOptions := append(defaultOptions(), configx.WithFlagSet(flags), configx.WithFiles(configFile))
	loadOptions = append(loadOptions, opts...)

	raw, err := configx.NewWatcherT[Config](loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("create agent config watcher: %w", err)
	}

	cfg, err := finalizeConfig(raw.Config())
	if err != nil {
		if closeErr := raw.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close agent config watcher: %w", closeErr))
		}
		return nil, fmt.Errorf("finalize agent config watcher: %w", err)
	}

	watcher := &Watcher{
		current: cfg,
		start:   raw.Start,
		close:   raw.Close,
	}
	raw.OnChange(func(cfg Config, err error) {
		if err != nil {
			watcher.notify(Config{}, err)
			return
		}
		cfg, err = finalizeConfig(cfg)
		if err != nil {
			watcher.notify(Config{}, err)
			return
		}
		watcher.update(cfg)
		watcher.notify(cfg, nil)
	})
	return watcher, nil
}

func newPollingWatcher(flags *pflag.FlagSet, configFile string, opts ...configx.Option) (*Watcher, error) {
	cfg, err := LoadFromFlags(flags, configFile, opts...)
	if err != nil {
		return nil, err
	}

	poller := &pollingWatcher{
		flags:      flags,
		configFile: configFile,
		opts:       append([]configx.Option{}, opts...),
		signature:  readFileSignature(configFile),
	}
	watcher := &Watcher{
		current: cfg,
	}
	watcher.start = func(ctx context.Context) error {
		return poller.start(ctx, watcher)
	}
	return watcher, nil
}

type pollingWatcher struct {
	flags      *pflag.FlagSet
	configFile string
	opts       []configx.Option
	signature  fileSignature
}

type fileSignature struct {
	modTime time.Time
	size    int64
	ok      bool
}

func (w *pollingWatcher) start(ctx context.Context, watcher *Watcher) error {
	if w.configFile == "" {
		<-ctx.Done()
		return fmt.Errorf("agent config watcher stopped: %w", ctx.Err())
	}

	ticker := time.NewTicker(fallbackWatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("agent config watcher stopped: %w", ctx.Err())
		case <-ticker.C:
			w.reloadIfChanged(watcher)
		}
	}
}

func (w *pollingWatcher) reloadIfChanged(watcher *Watcher) {
	signature := readFileSignature(w.configFile)
	if signature == w.signature {
		return
	}
	w.signature = signature

	cfg, err := LoadFromFlags(w.flags, w.configFile, w.opts...)
	if err != nil {
		watcher.notify(Config{}, oops.Wrapf(err, "reload agent config"))
		return
	}
	watcher.update(cfg)
	watcher.notify(cfg, nil)
}

func readFileSignature(path string) fileSignature {
	if path == "" {
		return fileSignature{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return fileSignature{}
	}
	return fileSignature{modTime: info.ModTime(), size: info.Size(), ok: true}
}
