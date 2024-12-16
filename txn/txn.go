// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package txn:
package txn

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/awinterman/anarchoredis/protocol"
	"golang.org/x/sync/errgroup"
)

type Conf struct {
	ListenAddress  string
	RedisAddress   string
	KafkaAddress   []string
	ClientID       string
	GroupID        string
	TCPReadTimeout int64

	net.Dialer
}

func (conf *Conf) LoadEnv() {
	conf.ListenAddress = os.Getenv("LISTEN_ADDRESS")
	conf.RedisAddress = os.Getenv("REDIS_ADDRESS")
	conf.KafkaAddress = strings.Split(os.Getenv("KAFKA_ADDRESS"), ",")
	conf.ClientID = os.Getenv("CLIENT_ID")
	conf.GroupID = os.Getenv("GROUP_ID")
	conf.TCPReadTimeout, _ = strconv.ParseInt(os.Getenv("REPLICATION_TCP_READ_TIMEOUT_MILLIS"), 10, 64)
	slog.Info("env loaded", "conf", conf)
}

type Transactor struct {
	conf *Conf
}

func NewTransactor(ctx context.Context, conf *Conf) (*Transactor, error) {
	transactor := Transactor{conf}

	return &transactor, nil
}

func (t *Transactor) Transact(ctx context.Context, conn net.Conn) error {
	connection := protocol.NewConnection(conn)

	d, err := t.conf.Dialer.DialContext(ctx, "tcp", t.conf.RedisAddress)

	if err != nil {
		return fmt.Errorf("could not dial upstream address %q: %w", t.conf.RedisAddress, err)
	}
	slog.Info("established upstream connection", "addr", d.LocalAddr(), "error", err)
	upstream := protocol.NewConnection(d)

	g := errgroup.Group{}

	replicatedMessages := make(chan *protocol.Message, 10)

	g.Go(func() error {
		return subscribe(ctx, t.conf, func(cmd *protocol.Message) {
			select {
			case replicatedMessages <- cmd:
			case <-ctx.Done():
				close(replicatedMessages)
			}
		})
	})
	g.Go(func() error {
		for ctx.Err() == nil {
			req, err := connection.Read()
			if err != nil {
				return err
			}
			slog.Info("recv", "cmd", req)
			_, err = upstream.Write(req)
			if err != nil {
				return err
			}
			err = upstream.Flush()
			if err != nil {
				return err
			}

			resp, err := upstream.Read()
			if err != nil {
				return err
			}

			cmd, err := req.Cmd()
			if err != nil {
				_, err := connection.Write(protocol.NewError(err))
				if err != nil {
					return err
				}
			}

			if cmd.IsWrite() {
				repl := <-replicatedMessages
				slog.Info("replicated; can return", "repl", repl.String())
			}

			_, err = connection.Write(resp)
			if err != nil {
				return err
			}

			err = connection.Flush()
			if err != nil {
				return err
			}
		}
		return ctx.Err()
	})

	return g.Wait()
}
