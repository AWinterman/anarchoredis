package anarchoredis

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
	"golang.org/x/sync/errgroup"
)

func TestTransactor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	conf := &Conf{}
	conf.LoadEnv()
	old := slog.SetLogLoggerLevel(slog.LevelDebug)
	defer slog.SetLogLoggerLevel(old)
	transactor, err := NewTransactor(ctx, conf)
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
			read, err := connection.Read()
			if err != nil {
				return err
			}
			t.Log("set response", read)

			read, err = connection.Read()
			if err != nil {
				return err
			}
			t.Log("get response", read)
			time.Sleep(time.Millisecond * 100)
		}
		return ctx.Err()
	})

	err = g.Wait()
	if err != io.EOF {
		t.Fatal(err)
	}

}
