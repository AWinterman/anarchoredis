// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package replication:
package replication

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
	"github.com/stretchr/testify/assert"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: nil,
	})))
}

func TestSubscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	ListenAddress := os.Getenv("LISTEN_ADDRESS")
	RedisAddress := os.Getenv("REDIS_ADDRESS")

	s := &Subscriber{
		Dialer:     net.Dialer{},
		LeaderAddr: RedisAddress,
		MyAddr:     ListenAddress,
		Logger:     slog.Default(),
	}
	old := slog.SetLogLoggerLevel(slog.LevelDebug)
	//old := slog.SetLogLoggerLevel(protocol.LogLevelTrace)
	defer slog.SetLogLoggerLevel(old)
	go func() {
		err := write(t, ctx, RedisAddress, s)
		if err != nil {
			t.Error("error generating updates", err)
		}
	}()

	err := s.StreamUpdates(ctx, func(cmd *protocol.Message) error {
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestGetSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	ListenAddress := os.Getenv("LISTEN_ADDRESS")
	RedisAddress := os.Getenv("REDIS_ADDRESS")

	s := &Subscriber{
		Dialer:     net.Dialer{},
		LeaderAddr: RedisAddress,
		MyAddr:     ListenAddress,
		Logger:     slog.Default(),
	}
	old := slog.SetLogLoggerLevel(slog.LevelDebug)
	//old := slog.SetLogLoggerLevel(protocol.LogLevelTrace)
	defer slog.SetLogLoggerLevel(old)
	go func() {
		err := write(t, ctx, RedisAddress, s)
		if err != nil {
			t.Error("error generating updates", err)
		}
	}()

	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}

	all, err := io.ReadAll(snapshot)
	if err != nil {
		return
	}

	t.Log(string(all))

}

func write(t *testing.T, ctx context.Context, addr string, s *Subscriber) error {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	p := protocol.NewConnection(conn)

	select {
	case <-s.ReplicationStartedCh():
		t.Log("replication started")
	case <-ctx.Done():
	}

	var i = 0
	for ctx.Err() == nil {
		i++
		array := protocol.NewOutgoingCommand(
			"SET",
			fmt.Sprintf("test:%d", i),
			time.Now().String(),
			"PX",
			"1000",
		)

		_, err := p.RoundTrip(array)
		if err != nil {
			return err
		}
		_, offsets, err := s.infoReplication(p)
		if err != nil {
			return err
		}

		ok := assert.Contains(t, offsets, s.Offset.Load())
		if !ok {
			return nil
		}

		time.Sleep(time.Millisecond * 100)
	}

	return nil
}
