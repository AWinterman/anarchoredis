package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/sourcegraph/conc/pool"
	"golang.org/x/sync/errgroup"
)

func TestServe(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()

	testErr := fmt.Errorf("oh no!")

	t.Run("if conn func returns error, exit", func(t *testing.T) {
		l, err := net.Listen("unix", path.Join(dir, "server"))
		if err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		s := Server{
			config: &Config{},
			l:      l,
			connFunc: func(ctx context.Context, conn net.Conn) error {
				t.Log("got conn")
				r := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
				line, _, err := r.ReadLine()
				if err != nil {
					return nil
				}
				if string(line) == "PING" {
					_, _ = r.WriteString("PONG\r\n")
				}

				return testErr
			},
			log: slog.Default(),
		}

		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error { return s.Serve(ctx) })
		g.Go(func() error {
			conn, err := net.Dial(l.Addr().Network(), l.Addr().String())
			t.Log("dialed", conn.RemoteAddr(), "with err=", err)
			if err != nil {
				return err
			}

			_, err = conn.Write([]byte("PING\n\r"))
			if err != nil {
				return err
			}

			return nil
		})

		err = g.Wait()
		t.Log("got error", err)
	})

	t.Run("can handle multiple conns at once", func(t *testing.T) {
		l, err := net.Listen("unix", path.Join(dir, "server"))
		if err != nil {
			t.Fatal(err)
		}

		is := is.New(t)
		ctx, cancel := context.WithTimeout(ctx, time.Millisecond*200)
		defer cancel()

		attempts := 100
		counter := atomic.Int32{}

		s := Server{
			&Config{},
			l,
			func(ctx context.Context, conn net.Conn) error {
				time.Sleep(1 * time.Millisecond)
				counter.Add(1)
				return nil
			},
			slog.Default(),
		}

		p := pool.New().WithErrors()
		p.Go(func() error {
			return s.Serve(ctx)
		})

		for i := 0; i < attempts; i++ {
			p.Go(func() error {
				conn, err := net.Dial(l.Addr().Network(), l.Addr().String())
				if err != nil {
					return err
				}
				defer conn.Close()
				_, err = conn.Write([]byte("ping\r\n"))
				if err != nil {
					return err
				}
				return nil
			})
		}

		err = p.Wait()
		is.True(errors.Is(err, context.DeadlineExceeded))
		is.Equal(counter.Load(), int32(attempts))
	})
}
