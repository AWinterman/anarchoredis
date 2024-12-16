package txn

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
	"golang.org/x/sync/errgroup"
)

func TestSubscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	conf := &Conf{}
	conf.LoadEnv()

	//old := slog.SetLogLoggerLevel(slog.LevelDebug)
	//defer slog.SetLogLoggerLevel(old)
	go func() {
		err := write(ctx, conf.RedisAddress)
		t.Error("error generating updates", err)
	}()

	err := subscribe(ctx, conf, func(cmd *protocol.Message) {
	})
	if err != nil {
		t.Fatal(err)
	}
}

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
		slog.Info("starting transactor")
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
			array := protocol.NewOutgoingCommand("SET", time.Now().String(), time.Now().String(), "PX", "1000")
			slog.Info("writing", "msg", array)
			_, err := connection.Write(array)
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
			t.Log(read)
			time.Sleep(time.Millisecond * 100)
			return nil
		}
		return ctx.Err()
	})

	err = g.Wait()
	if err != io.EOF {
		t.Fatal(err)
	}

}

func write(ctx context.Context, addr string) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	for ctx.Err() == nil {
		array := protocol.NewOutgoingCommand("SET", time.Now().String(), time.Now().String(), "PX", "1000")

		_, err = protocol.Write(rw, array)
		if err != nil {
			return err
		}
		err = rw.Flush()
		if err != nil {
			return err
		}

		_, err := protocol.Read(rw)
		if err != nil {
			return err
		}
	}

	return nil
}
