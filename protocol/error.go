// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package protocol:
package protocol

// NewError creates a new Message with the Indicator set to Error and the provided error assigned to the Error field.
func NewError(err error) *Message {
	return &Message{
		Kind:  Error,
		Error: err,
	}
}
