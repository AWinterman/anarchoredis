// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description:

// Package message:
package protocol

import (
	"github.com/awinterman/anarchoredis/protocol/kind"
	"github.com/awinterman/anarchoredis/protocol/message"
)

type Indicator = kind.Kind

const (
	End = "\r\n"

	SimpleString   = kind.SimpleString
	Error          = kind.Error
	Int            = kind.Int
	BulkString     = kind.BulkString
	Array          = kind.Array
	Null           = kind.Null
	Bool           = kind.Bool
	Double         = kind.Double
	BigNumber      = kind.BigNumber
	BulkError      = kind.BulkError
	VerbatimString = kind.VerbatimString
	Map            = kind.Map
	Attribute      = kind.Attribute
	Sets           = kind.Set
	Push           = kind.Push
)

// Message is a composite type that represents a message in the protocol
// the Indicator says which fields should be respected.
//
// where sensible, metadata or original data will be included in the
// struct, for example, the encoding of a VerbatimString
type Message = message.Message
