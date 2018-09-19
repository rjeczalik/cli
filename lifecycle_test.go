package zinc_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rjeczalik/zinc"
)

func funcs(fns ...func(context.Context) error) []func(context.Context) error { return fns }

func fn(d time.Duration, err error) func(context.Context) error {
	return func(ctx context.Context) error {
		<-ctx.Done()
		time.Sleep(d)
		return err
	}
}

func timeout(mili time.Duration) context.Context {
	ctx, _ := context.WithTimeout(context.Background(), mili*time.Millisecond)
	return ctx
}

func TestLifecycle(t *testing.T) {
	cases := map[string]struct {
		l     *zinc.Lifecycle
		funcs []func(context.Context) error
		err   error
	}{
		"early done": {
			&zinc.Lifecycle{Context: timeout(10)},
			funcs(fn(0, nil), fn(0, nil), fn(25*time.Millisecond, nil)),
			nil,
		},
		"timeout": {
			&zinc.Lifecycle{Context: timeout(50), Timeout: 100 * time.Millisecond},
			funcs(fn(0, nil), fn(0, nil), fn(150*time.Millisecond, nil)),
			zinc.ErrTimeout(100 * time.Millisecond),
		},
		"error": {
			&zinc.Lifecycle{Context: timeout(100)},
			funcs(fn(0, nil), fn(0, nil), fn(0, io.EOF)),
			io.EOF,
		},
	}

	for name, cas := range cases {
		t.Run(name, func(t *testing.T) {
			l := cas.l
			for _, fn := range cas.funcs {
				l.Go(fn)
			}
			err := l.Wait()
			switch {
			case cas.err != nil:
				if err == nil {
					t.Errorf("got %#v; want %#v", err, cas.err)
				}
			case err != nil:
				t.Errorf("Wait()=%s", err)
			}
		})
	}
}
