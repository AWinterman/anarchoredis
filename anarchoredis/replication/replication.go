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
}

// Subscribe subscribes to a replication stream
func (s Subscriber) Subscribe(ctx context.Context, msgFunc func(cmd *protocol.Message) error) error {
	conn, err := s.Dialer.DialContext(ctx, "tcp", s.LeaderAddr)
	mu := &sync.Mutex{}
	if err != nil {
		return err
	}

	host, port, err := net.SplitHostPort(s.LeaderAddr)
	if err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	rw := bufio.NewReadWriter(r, w)

	replids, offset, err := infoReplication(rw)
	if err != nil {
		return err
	}

	psync := protocol.NewArray(
		protocol.NewBulkString("PSYNC"),
		protocol.NewBulkString(replids[0]),
		protocol.NewBulkString(strconv.FormatInt(offset, 10)),
	)
	configure := protocol.NewOutgoingCommand("REPLCONF",
		"listening-port", port,
		"ip-address", host,
		"rdb-filter-only", "",
		"capa", "eof",
		"ack", strconv.FormatInt(offset, 10),
	)

	slog.Debug("sending", "cmd", psync)
	_, err = protocol.Write(rw, psync)
	if err != nil {
		return err
	}
	_, err = protocol.Write(rw, configure)
	if err != nil {
		return err
	}

	err = w.Flush()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Millisecond * 900):
				mu.Lock()
				err := replconfAck(rw, atomic.LoadInt64(&offset))
				mu.Unlock()
				if err != nil {
					slog.Error("replconfAck", "err", err)
				}
			}
		}
	}()

	for ctx.Err() == nil {
		mu.Lock()
		read, err := protocol.Read(rw)
		mu.Unlock()
		if err != nil {
			return fmt.Errorf("%w reading message", err)
		}
		atomic.AndInt64(&offset, read.OriginalSize)
		if read.Indicator == protocol.Array {
			if read.Array[0].Str != "PING" {
				err := msgFunc(read)
				if err != nil {
					return err
				}
			}
		}
		slog.Debug("replication msg", "msg", read)
	}

	return nil
}

func infoReplication(rw *bufio.ReadWriter) ([]string, int64, error) {
	replication := protocol.NewArray(
		protocol.NewBulkString("info"),
		protocol.NewBulkString("replication"),
	)

	_, err := protocol.Write(rw, replication)
	if err != nil {
		return nil, 0, err
	}

	err = rw.Writer.Flush()
	if err != nil {
		return nil, 0, err
	}

	read, err := protocol.Read(rw)
	if err != nil {
		return nil, 0, err
	}

	split := strings.Split(read.Str, "\r\n")
	var replids []string
	var offset int64
	for _, line := range split {
		kvs := strings.Split(line, ":")
		if strings.HasPrefix(kvs[0], "master_replid") {
			replids = append(replids, kvs[1])
		}
		if strings.HasPrefix(kvs[0], "master_repl_offset") {
			offset, err = strconv.ParseInt(kvs[1], 10, 64)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	return replids, offset, nil
}

// replconfAck sends an ack back to the server
func replconfAck(rw *bufio.ReadWriter, offset int64) error {
	req := protocol.NewArray(
		protocol.NewBulkString("replconf"),
		protocol.NewBulkString("ack"),
		protocol.NewBulkString(strconv.FormatInt(offset, 10)),
	)

	slog.Debug("sending", "msg", req)
	_, err := protocol.Write(rw, req)
	if err != nil {
		return err
	}
	err = rw.Writer.Flush()
	if err != nil {
		return err
	}

	read, err := protocol.Read(rw)
	if err != nil {
		return err
	}
	slog.Debug("received", "msg", read, "size", read.OriginalSize)
	return nil
}
