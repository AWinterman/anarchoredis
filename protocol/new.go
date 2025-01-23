// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package protocol:
package protocol

import (
	"bytes"

	"github.com/awinterman/anarchoredis/protocol/message"
)

// NewArray creates a new Message instance with the Indicator set to Array and its Array field populated with the given messages.
func NewArray(messages ...*Message) *Message {
	var ms []Message
	for _, msg := range messages {
		ms = append(ms, *msg)
	}
	array := message.Array(ms...)
	return &array
}

func NewBulkString(s string) *Message {
	bulkString := message.BulkString(bytes.NewBufferString(s), int64(len(s)))
	return &bulkString
}

func NewOutgoingCommand(args ...string) *Message {
	var ms []*Message
	for i := range args {
		ms = append(ms, NewBulkString(args[i]))
	}
	a := NewArray(ms...)

	return a
}
