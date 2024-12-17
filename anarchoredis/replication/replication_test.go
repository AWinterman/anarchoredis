// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package replication:
package replication

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
)

func TestSubscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	ListenAddress := os.Getenv("LISTEN_ADDRESS")
	RedisAddress := os.Getenv("REDIS_ADDRESS")

	//old := slog.SetLogLoggerLevel(slog.LevelDebug)
	//defer slog.SetLogLoggerLevel(old)
	go func() {
		err := write(ctx, RedisAddress)
		t.Error("error generating updates", err)
	}()

	s := Subscriber{
		Dialer:     net.Dialer{},
		LeaderAddr: RedisAddress,
		MyAddr:     ListenAddress,
		Logger:     slog.Default(),
	}

	err := s.Subscribe(ctx, func(cmd *protocol.Message) error {
		return nil
	})
	if err != nil {
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
