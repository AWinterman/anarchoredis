// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package protocol:
package protocol

// NewArray creates a new Message instance with the Indicator set to Array and its Array field populated with the given messages.
func NewArray(messages ...*Message) *Message {
	return &Message{Indicator: Array, Array: messages}
}

func NewBulkString(s string) *Message {
	return &Message{Str: s, Indicator: BulkString}
}

func NewSimpleString(s string) *Message {
	return &Message{
		Indicator: String,
		Str:       s,
	}
}

func NewOutgoingCommand(args ...string) *Message {
	var ms []*Message
	for i := range args {
		ms = append(ms, NewBulkString(args[i]))
	}
	a := NewArray(ms...)

	return a
}
