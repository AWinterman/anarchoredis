package anarchoredis

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
)

type testLog struct {
}

func (t testLog) Append(ctx context.Context, msg *protocol.Message, database string) error {
	return nil
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	})))
}

func TestTransactor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	conf := &Conf{}
	conf.LoadEnv()

	transactor, err := NewTransactor(ctx, conf, &testLog{})

	if err != nil {
		t.Fatal(err)
	}

	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		t.Log("starting transactor")
		return transactor.Transact(ctx, p2)
	})
	go func() {
		<-ctx.Done()
		p1.Close()
		p2.Close()
	}()

	g.Go(func() error {
		connection := protocol.NewConnection(p1)
		for ctx.Err() == nil {
			s := time.Now().String()
			set := protocol.NewOutgoingCommand("SET", s, s, "PX", "1000")
			get := protocol.NewOutgoingCommand("GET", s)

			t.Log("writing", "msg", set)
			_, err := connection.Write(set)
			if err != nil {
				return err
			}
			_, err = connection.Write(get)
			if err != nil {
				return err
			}
			err = connection.Flush()
			if err != nil {
				return err
			}
			setrespoonse, err := connection.Read()
			if err != nil {
				return err
			}

			readresponse, err := connection.Read()
			if err != nil {
				return err
			}

			assert.Equal(t, setrespoonse.Str, "OK")
			assert.Equal(t, readresponse.Str, s)
			time.Sleep(time.Millisecond * 1000)
		}
		return ctx.Err()
	})

	err = g.Wait()
	if err != io.EOF || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}

}
