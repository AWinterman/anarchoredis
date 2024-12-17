// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package anarchoredis:
package anarchoredis

import (
	"bytes"
	"context"

	"github.com/awinterman/anarchoredis/protocol"
	"github.com/twmb/franz-go/pkg/kgo"
)

type TxnLog struct {
	kafka      *kgo.Client
	clientId   string
	buffer     *bytes.Buffer // underlies connection
	connection *protocol.Conn
}

func NewTxnLog(clientID string, kafkaBrokers []string, topic string) (*TxnLog, error) {
	buffer := bytes.NewBuffer(make([]byte, 1024))
	conn := protocol.NewConnection(buffer)
	kafka, err := kgo.NewClient(kgo.ClientID(clientID), kgo.SeedBrokers(kafkaBrokers...),
		kgo.AllowAutoTopicCreation(), kgo.DefaultProduceTopic(topic))
	if err != nil {
		return nil, err
	}

	return &TxnLog{kafka, clientID, buffer, conn}, nil
}

// Append a record to the log
func (l TxnLog) Append(ctx context.Context, message *protocol.Message, cb func(mgs *protocol.Message, err error)) error {

	l.connection.Lock()
	_, err := l.connection.Write(message)
	if err != nil {
		l.connection.Unlock()
		return err
	}

	c := make([]byte, l.buffer.Len())
	copy(c, l.buffer.Bytes())
	l.buffer.Reset()
	l.connection.Unlock()

	l.kafka.Produce(ctx, &kgo.Record{
		Value: c,
	}, func(record *kgo.Record, err error) {
		cb(message, err)
	})

	return nil
}

// Heartbeat tells replicas the leader is still alive.
func (l TxnLog) Heartbeat(ctx context.Context) error {
	msg := protocol.NewOutgoingCommand("CONTROL", "HEARTBEAT", l.clientId)

	e := make(chan error)
	defer close(e)
	err := l.Append(ctx, msg, func(mgs *protocol.Message, err error) {
		if err != nil {
			e <- err
		}
	})
	if err != nil {
		return err
	}
	return <-e
}
