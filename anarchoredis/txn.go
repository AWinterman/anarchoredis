// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package anarchoredis:
package anarchoredis

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
	"github.com/awinterman/anarchoredis/txn/localstate"
	"github.com/awinterman/anarchoredis/txn/replication"
	"golang.org/x/sync/errgroup"

	"github.com/dgraph-io/badger/v4"
)

type Conf struct {
	ListenAddress string
	RedisAddress  string

	KafkaAddress []string
	Topic        string

	ClientID string
	GroupID  string

	LocalStateDir string
	LockTTL       time.Duration

	net.Dialer
}

func (conf *Conf) LoadEnv() {
	conf.ListenAddress = os.Getenv("LISTEN_ADDRESS")
	conf.RedisAddress = os.Getenv("REDIS_ADDRESS")
	conf.KafkaAddress = strings.Split(os.Getenv("KAFKA_ADDRESS"), ",")
	conf.ClientID = os.Getenv("CLIENT_ID")
	conf.GroupID = os.Getenv("GROUP_ID")
	conf.Topic = os.Getenv("TXN_TOPIC")
	slog.Info("env loaded", "conf", conf)
}

type Transactor struct {
	conf                       *Conf
	keys                       *localstate.Store
	redisReplicationSubscriber *replication.Subscriber
	txnlog                     *TxnLog
	database                   *atomic.Pointer[string]
}

func NewTransactor(ctx context.Context, conf *Conf) (*Transactor, error) {
	db, err := badger.Open(badger.DefaultOptions(conf.LocalStateDir).WithInMemory(conf.LocalStateDir == ""))
	if err != nil {
		return nil, fmt.Errorf("badgerdb.Open(): %w", err)
	}

	log, err := NewTxnLog(conf.ClientID, conf.KafkaAddress, conf.Topic)
	transactor := Transactor{
		conf,
		&localstate.Store{DB: db, Log: slog.With("comp", "key-lock")},
		&replication.Subscriber{
			Dialer:     conf.Dialer,
			LeaderAddr: conf.RedisAddress,
			MyAddr:     conf.ListenAddress,
			Logger:     slog.With("comp", "replication"),
		},
		log,
		&atomic.Pointer[string]{},
	}
	database := "0"
	transactor.database.Store(&database)

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

	g.Go(func() error {
		return t.redisReplicationSubscriber.Subscribe(ctx,
			func(msg *protocol.Message) error {
				ctx, cancel := context.WithCancelCause(ctx)
				defer cancel(nil)
				if ctx.Err() != nil {
					return ctx.Err()
				}

				cmd, err := msg.Cmd()
				if err != nil {
					return err
				}

				keys, err := cmd.Keys()
				if err != nil {
					return err
				}

				err = t.txnlog.Append(ctx, msg, func(mgs *protocol.Message, err error) {
					if err == nil {
						err := t.keys.UnlockKeys(keys)
						if err != nil {
							cancel(err)
						}
						return
					}
					cancel(err)
				})

				if err != nil {
					return err
				}

				return context.Cause(ctx)
			})
	})
	g.Go(func() error {
		for ctx.Err() == nil {
			err2 := t.proxy(ctx, connection, upstream)
			if err2 != nil {
				return err2
			}
		}
		return context.Cause(ctx)
	})

	return g.Wait()
}

func (t *Transactor) proxy(ctx context.Context, connection *protocol.Conn, upstream *protocol.Conn) error {
	log := slog.With("comp", "proxy")
	req, err := connection.Read()
	if err != nil {
		return err
	}

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
		msg := protocol.NewError(err)
		_, err := connection.Write(msg)
		if err != nil {
			return err
		}
	}
	if cmd.Name == "SELECT" {
		t.database.Store(&cmd.Args[0])
	}

	if cmd.IsWrite() {
		err := t.keys.LockKeys(cmd)
		if err != nil {
			return err
		}
	}

	log.Debug("awaiting release of lock", "msg", cmd.Message)
	err = t.keys.AwaitUnlocked(ctx, cmd)
	if err != nil {
		return err
	}

	log.Info("command", "req", req, "resp", resp)

	_, err = connection.Write(resp)
	if err != nil {
		return err
	}

	err = connection.Flush()
	if err != nil {
		return err
	}
	return nil
}
