// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description:

// Package protocol:
package protocol

import (
	"bufio"
	"io"
	"log/slog"
	"sync"

	"github.com/awinterman/anarchoredis/protocol/message"
)

type ConnOptions struct {
	NewWriter func(writer2 io.Writer) bufio.Writer
	NewReader func(io.Reader) bufio.Reader
	Logger    *slog.Logger
}

func NewConnection(conn io.ReadWriter) *Conn {
	c := Conn{
		RW:     bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		Logger: slog.With("comp", "conn"),
	}
	return &c
}

// Conn represents a thread-safe connection that provides read, write, and logging capabilities.
type Conn struct {
	sync.Mutex
	RW      *bufio.ReadWriter
	Logger  *slog.Logger
	Encoder message.Encoder
}

// Read locks the connection, reads a message using the connection's SubStream, and returns the parsed message or an error.
func (conn *Conn) Read() (Message, error) {
	conn.Lock()
	defer conn.Unlock()
	return conn.Encoder.Decode(conn.RW.Reader)
}

// Write writes the provided Message to the connection's SubStream and returns the number of bytes written or an error.
func (conn *Conn) Write(m Message) (int, error) {
	conn.Lock()
	defer conn.Unlock()
	return conn.Encoder.Encode(m, conn.RW.Writer)
}

// Flush writes any buffered data to the underlying writer from the connection's read-write buffer.
func (conn *Conn) Flush() error {
	conn.Lock()
	defer conn.Unlock()
	return conn.RW.Flush()
}

// RawRoundtrip sends raw byte data through the connection, flushes it, and reads the response as a Message.
func (conn *Conn) RawRoundtrip(data []byte) (Message, error) {
	conn.Lock()
	defer conn.Unlock()
	_, err := conn.RW.Write(data)
	if err != nil {
		return Message{}, err
	}
	err = conn.RW.Flush()
	if err != nil {
		return Message{}, err
	}

	msg, err := conn.Read()
	if err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (conn *Conn) RoundTrip(msg Message) (Message, error) {
	_, err := conn.Write(msg)
	if err != nil {
		return Message{}, err
	}
	err = conn.Flush()
	resp, err := conn.Read()

	conn.Logger.Debug("command", "cmd", msg, "resp", resp, "err", err)
	return resp, err
}
