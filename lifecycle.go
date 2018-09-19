package zinc // import github.com/rjeczalik/zinc

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var globalLifecycle = &Lifecycle{
	SetupFunc: SetupProcessLifecycle,
	Timeout:   15 * time.Second,
}

type ErrTimeout time.Duration

func (e ErrTimeout) Error() string {
	return "timed out after " + time.Duration(e).String()
}

type Lifecycle struct {
	Context   context.Context // termination signals
	Timeout   time.Duration
	SetupFunc func(*Lifecycle) // called once lazily on first use if non-nil

	setupOnce sync.Once
	waitOnce  sync.Once
	wg, waitg errgroup.Group
	err       error
}

func (l *Lifecycle) Go(fn func(context.Context) error) {
	l.setupOnce.Do(l.setup)
	if l.Context == nil {
		panic("lifecycle's context is nil")
	}
	l.wg.Go(func() error { return fn(l.Context) })
}

func (l *Lifecycle) Wait() error {
	l.waitOnce.Do(l.wait)
	return l.waitg.Wait()
}

func (l *Lifecycle) setup() {
	if l.SetupFunc != nil {
		l.SetupFunc(l)
	}
}

func (l *Lifecycle) wait() {
	ch := make(chan error)
	done := l.Context.Done()
	l.waitg.Go(func() error {
		var timeout <-chan time.Time
		for {
			select {
			case err := <-ch:
				return err
			case <-done:
				done = nil
				if l.Timeout > 0 {
					timeout = time.After(l.Timeout)
				}
			case <-timeout:
				return ErrTimeout(l.Timeout)
			}
		}
	})
	go func() {
		select {
		case <-done:
		case ch <- l.wg.Wait():
		}
	}()
}

func Go(fn func(context.Context) error) {
	globalLifecycle.Go(fn)
}

func Wait() error {
	return globalLifecycle.Wait()
}

func SetupProcessLifecycle(l *Lifecycle) {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal)

	go func() {
		for range c {
			cancel()
		}
	}()

	signal.Notify(c, os.Interrupt, os.Kill)

	l.Context = ctx
}
