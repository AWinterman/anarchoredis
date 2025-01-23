// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description:

// Package message:
package message

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"math/big"
	"strings"

	"github.com/awinterman/anarchoredis/protocol/kind"
)

// Message is a composite type that represents a message in the protocol
// the Kind says which fields should be respected.
//
// where sensible, metadata or original data will be included in the
// struct, for example, the encoding of a VerbatimString
type Message struct {
	// Kind is what kind of message it is
	Kind kind.Kind

	// Simple types
	SimpleString string
	Error        error
	Int          int64
	Bool         bool
	Double       float64
	BigNumber    *big.Int

	// including indicator bytes, line feeds, encoded run length, etc.
	TotalMessageSize int64

	// Aggregate types
	RunLength int64
	// Reader is for large strings, e.g. BulkError, String, VerbatimString
	Reader io.Reader

	Encoding [3]byte

	// Collection Types
	Seq   iter.Seq2[Message, error]    // Sequence
	Assoc iter.Seq2[[2]Message, error] // Association of pairs
}

func (m Message) ReadAll() (string, error) {
	b := make([]byte, m.RunLength)
	n, err := io.ReadFull(m.Reader, b)
	if err != nil {
		return "", nil
	}
	return string(b[:n]), nil
}

func SimpleString(s string) Message {
	return Message{Kind: kind.SimpleString, SimpleString: s}
}

func Error(s string) Message {
	return Message{Kind: kind.Error, Error: errors.New(s)}
}

func Null() Message {
	return Message{Kind: kind.Null}
}

func Int(i int64) Message {
	return Message{Kind: kind.Int, Int: i}
}

func Bool(b bool) Message {
	return Message{Kind: kind.Bool, Bool: b}
}

func Double(f float64) Message {
	return Message{Kind: kind.Double, Double: f}
}

func BigNumber(b *big.Int) Message {
	return Message{Kind: kind.BigNumber, BigNumber: b}
}

func BulkString(s io.Reader, runLength int64) Message {
	return Message{Kind: kind.BulkString, Reader: s, RunLength: runLength}
}

func BulkError(s io.Reader, runlength int64) Message {
	return Message{Kind: kind.BulkError, Reader: s, RunLength: runlength}
}

func VerbatimString(s io.Reader, runlength int64, encoding [3]byte) Message {
	return Message{Kind: kind.VerbatimString, Reader: s, RunLength: runlength, Encoding: encoding}
}

func Array(arrs ...Message) Message {
	i := iter.Seq2[Message, error](func(yield func(Message, error) bool) {
		for i := 0; i < len(arrs); i += 1 {
			ok := yield(arrs[i], nil)
			if !ok {
				return
			}
		}
	})
	return Message{Kind: kind.Array, Seq: i, RunLength: int64(len(arrs))}
}

func Set(arrs ...Message) Message {
	i := iter.Seq2[Message, error](func(yield func(Message, error) bool) {
		for i := 0; i < len(arrs); i += 1 {
			ok := yield(arrs[i], nil)
			if !ok {
				return
			}
		}
	})
	return Message{Kind: kind.Set, Seq: i, RunLength: int64(len(arrs))}
}

func Map(kvs ...Message) Message {
	if len(kvs)%2 != 0 {
		panic("must have even number of key/value pairs")
	}

	i := iter.Seq2[[2]Message, error](func(yield func([2]Message, error) bool) {
		for i := 0; i < len(kvs); i += 2 {
			k := kvs[i]
			v := kvs[i+1]
			ok := yield([2]Message{k, v}, nil)
			if !ok {
				return
			}
		}
	})

	return Message{Kind: kind.Map, Assoc: i, RunLength: int64(len(kvs) / 2)}
}

func (m Message) String() string {
	return fmt.Sprintf("%s%s", string(m.Kind), m.string())
}

func (m Message) string() string {
	switch m.Kind {
	case kind.SimpleString:
		return m.SimpleString
	case kind.Error:
		return m.Error.Error()
	case kind.Int:
		return fmt.Sprintf("%d", m.Int)
	case kind.Array:
		var s []string
		for msg, err := range m.Seq {
			if err == nil {
				s = append(s, msg.String())
			} else {
				s = append(s, "!ERR:"+err.Error())
			}
		}
		return fmt.Sprintf("%s", strings.Join(s, " "))
	case kind.Map:
		var s []string

		for pair, err := range m.Assoc {
			if err == nil {
				s = append(s, pair[0].String()+": "+pair[1].String())
			} else {
				s = append(s, "!ERR:"+err.Error())
			}
		}
		return fmt.Sprintf("{%s}", strings.Join(s, " "))
	case kind.Bool:
		return fmt.Sprintf("%v", m.Bool)
	case kind.Double:
		return fmt.Sprintf("%v", m.Double)
	case kind.BigNumber:
		return fmt.Sprintf("%v", m.BigNumber)
	case kind.VerbatimString:
		return m.SimpleString
	default:
		return fmt.Sprintf("Unknown %s", m.Kind)
	}
}
