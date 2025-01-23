// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

package replication

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
)

type Subscriber struct {
	Dialer             net.Dialer
	LeaderAddr, MyAddr string
	Logger             *slog.Logger

	Offset        atomic.Int64
	ReplicationID atomic.Pointer[string]

	signal
}

// signal is a struct used to manage a one-time signaling mechanism with thread-safety.
// It offers a single broadcast operation and provides synchronization using a channel.
// Contains a channel, synchronization primitives, and an atomic flag for signaling status.
type signal struct {
	ch        chan struct{}
	once      sync.Once
	didSignal atomic.Bool
	mu        sync.Mutex
}

// Broadcast signals all goroutines waiting on the signal by closing the channel, ensuring it happens only once.
func (s *signal) Broadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch == nil {
		s.ch = make(chan struct{})
	}
	s.once.Do(func() {
		close(s.ch)
		s.didSignal.Store(true)
	})
}

// ReplicationStartedCh returns a channel that signals when replication starts. The channel is lazily initialized if nil.
func (s *Subscriber) ReplicationStartedCh() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch == nil {
		s.ch = make(chan struct{})
	}
	return s.ch
}

// pointer returns a pointer to the given value of type T.
func pointer[T any](t T) *T {
	return &t
}

// StreamUpdates subscribes to a replication stream. It blocks
func (s *Subscriber) StreamUpdates(
	ctx context.Context,
	msgFunc func(cmd *protocol.Message) error,
) error {
	s.ReplicationID.CompareAndSwap(nil, pointer(""))
	if s.signal.didSignal.Load() {
		return fmt.Errorf("attempting to reuse a subscriber, which is not allowed")
	}

	p, err2 := s.startReplication(ctx, *s.ReplicationID.Load(), s.Offset.Load(), false)
	if err2 != nil {
		return err2
	}
	// when we exit; try to send the last stored replication offset.
	defer s.replconfAck(p, s.Offset.Load())

	// a goroutine that regularly sends the offset to the server.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond * 900):
				err := s.replconfAck(p, s.Offset.Load())
				if err != nil {
					slog.Error("replconfAck", "err", err)
				}
			}
		}
	}()

	for ctx.Err() == nil {
		read, err := p.Read()

		if err != nil {
			return fmt.Errorf("%w reading message", err)
		}
		slog.Debug("replication", "msg", read)

		switch {
		case read.Kind == protocol.SimpleString:
			split := strings.Split(read.SimpleString, " ")
			switch split[0] {
			case "FULLRESYNC":
				// from redis docs:
				// /* Send a FULLRESYNC reply in the specific case of a full resynchronization,
				// * as a side effect setup the slave for a full sync in different ways:
				// *
				// * 1) Remember, into the slave client structure, the replication offset
				// *    we sent here, so that if new slaves will later attach to the same
				// *    background RDB saving process (by duplicating this client output
				// *    buffer), we can get the right offset from this slave.
				// * 2) Set the replication state of the slave to WAIT_BGSAVE_END so that
				// *    we start accumulating differences from this point.
				// * 3) Force the replication stream to re-emit a SELECT statement so
				// *    the new slave incremental differences will start selecting the
				// *    right database number.
				// *
				// * Normally this function should be called immediately after a successful
				// * BGSAVE for replication was started, or when there is one already in
				// * progress that we attached our slave to. */

				// store the provided replication id
				s.ReplicationID.Store(&split[1])
				// storing the offset
				offset, err := strconv.ParseInt(split[2], 10, 64)
				if err != nil {
					return err
				}
				s.Offset.Store(offset)
				slog.Info(read.String())
				s.signal.Broadcast()
			default:
				slog.Info("replication metadata", "msg", read)
			}
		case read.Kind == protocol.Array:
			switch {
			case read.Array[0].Str != "PING":
				err := msgFunc(read)
				if err != nil {
					return err
				}
			case read.Array[0].Str != "REPLCONF":
				s.Logger.Info("received REPLCONF", "msg", read)
			}
		case read.Indicator == protocol.Error:
			return fmt.Errorf("%s", read)
		case read.Indicator == protocol.BulkString && strings.HasPrefix(read.Str, "REDIS0011"):
			s.Logger.Info("StreamUpdates received RDB snapshot; skipping")
		}

		s.Offset.Add(read.Size)
	}

	return nil
}

func (s *Subscriber) Snapshot(ctx context.Context) (*bufio.Reader, error) {
	replication, err := s.startReplication(ctx, "?", 0, true)
	if err != nil {
		return nil, err
	}
	for ctx.Err() == nil {
		read, err := replication.Read()
		if err != nil {
			return nil, err
		}
		if read.Indicator == protocol.BulkString {
		}
	}
}

func (s *Subscriber) startReplication(ctx context.Context, replicationID string, offset int64,
	snapshotOnly bool) (*protocol.Conn,
	error) {
	conn, err := s.Dialer.DialContext(ctx, "tcp", s.LeaderAddr)
	if err != nil {
		return nil, err
	}

	slog.Info("start replication", "leader", s.LeaderAddr, "myaddress", s.MyAddr)

	myHost, myPort, err := net.SplitHostPort(s.MyAddr)
	if err != nil {
		return nil, err
	}

	p := protocol.NewConnection(conn)
	p.Logger = s.Logger

	ping := protocol.NewOutgoingCommand("PING")
	psync := protocol.NewOutgoingCommand(
		"PSYNC",
		replicationID,
		strconv.FormatInt(offset, 10),
	)
	replconf := []string{
		"REPLCONF",
		"listening-port", myPort,
		"ip-address", myHost,
		"capa", "psync2",
	}
	if snapshotOnly {
		replconf = append(replconf, "snapshot-only", "1")
	}

	capa := protocol.NewOutgoingCommand(replconf...)

	_, err = p.RoundTrip(ping)
	if err != nil {
		return nil, err
	}
	_, err = p.RoundTrip(capa)
	if err != nil {
		return nil, err
	}
	_, err = p.RoundTrip(psync)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (s *Subscriber) infoReplication(p *protocol.Conn) ([]string, []int64, error) {
	replication := protocol.NewArray(
		protocol.NewBulkString("info"),
		protocol.NewBulkString("replication"),
	)
	read, err := p.RoundTrip(replication)
	if err != nil {
		return nil, nil, err
	}

	var replids []string
	var offsets []int64
	var attrs []slog.Attr

	split := strings.Split(read.Str, "\r\n")
	for _, line := range split {
		kvs := strings.Split(line, ":")
		if len(kvs) != 2 {

			continue
		}
		if strings.HasPrefix(kvs[0], "master_replid") {
			replids = append(replids, kvs[1])
		}
		if strings.HasPrefix(kvs[0], "master_repl_offset") {
			offset, err := strconv.ParseInt(kvs[1], 10, 64)
			if err != nil {
				return nil, nil, err
			}
			offsets = append(offsets, offset)
		}
		attrs = append(attrs, slog.String(kvs[0], kvs[1]))
	}
	s.Logger.LogAttrs(context.Background(), slog.LevelDebug, "replication info", attrs...)
	return replids, offsets, nil
}

// replconfAck sends an ack back to the server
func (s *Subscriber) replconfAck(p *protocol.Conn, offset int64) error {
	req := protocol.NewOutgoingCommand(
		"replconf",
		"ack",
		strconv.FormatInt(offset, 10),
	)

	msg, err := p.RoundTrip(req)
	if err != nil {
		return err
	}

	if strings.ToUpper(msg.Str) == "PING" {
		s.Logger.Info("got PING response; sending PONG")
		_, err := p.Write(protocol.NewOutgoingCommand("PONG"))
		if err != nil {
			return err
		}
	}

	return nil
}
