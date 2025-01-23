// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package raftbadger:
package raftbadger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
	anarchoredis "github.com/awinterman/anarchoredis/txn"
	"github.com/awinterman/anarchoredis/txn/replication"
	"github.com/hashicorp/raft"
	"github.com/jackc/puddle/v2"
)

type Appender struct {
	path                string
	RaftBind            string
	RaftDir             string
	RetainSnapshotCount int
	pool                puddle.Pool[*protocol.Conn]
	raft                *raft.Raft
	dialer              net.Dialer
	LeaderAddr, MyAddr  string
	Logger              *slog.Logger
}

type MsgOrError struct {
	Msg *protocol.Message
	Err error
}

func (s *Appender) Apply(log *raft.Log) interface{} {
	ctx := context.Background()
	pc, err := s.pool.Acquire(ctx)
	if err != nil {
		return &MsgOrError{
			Err: err,
		}
	}
	defer pc.Release()

	value := pc.Value()
	msg, err := value.RawRoundtrip(log.Data)

	return &MsgOrError{Err: err, Msg: msg}
}

type RedisFSM struct {
	pool puddle.Pool[*protocol.Conn]

	ctx  context.Context
	conf *anarchoredis.Conf
}

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.
func (r *RedisFSM) Apply(log *raft.Log) interface{} {
	poolItem, err := r.pool.Acquire(r.ctx)
	if err != nil {
		return MsgOrError{Err: err}
	}

	defer poolItem.Release()
	conn := poolItem.Value()

	resp, err := conn.RawRoundtrip(log.Data)
	if err != nil {
		poolItem.Destroy()
		return MsgOrError{Err: err}
	}

	return MsgOrError{
		Msg: resp,
		Err: err,
	}
}

// Snapshot is used to support log compaction. This call should
// return an FSMSnapshot which can be used to save a point-in-time
// snapshot of the FSM. Apply and Snapshot are not called in multiple
// threads, but Apply will be called concurrently with Persist. This means
// the FSM should be implemented in a fashion that allows for concurrent
// updates while a snapshot is happening.
func (r *RedisFSM) Snapshot() (raft.FSMSnapshot, error) {
	poolItem, err := r.pool.Acquire(r.ctx)
	if err != nil {
		return MsgOrError{Err: err}
	}

	defer poolItem.Release()
	conn := poolItem.Value()

	conn.Write(&)
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
func (r RedisFSM) Restore(closer io.ReadCloser) error {
	//TODO implement me
	panic("implement me")
}

// Open opens the store. If enableSingle is set, and there are no existing peers,
// then this node becomes the first node, and therefore leader, of the cluster.
// localID should be the server identifier for this node.
func (s *Appender) Open(enableSingle bool, localID string) error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.RaftBind)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(s.RaftBind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(s.RaftDir, s.RetainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	var logStore raft.LogStore
	var stableStore raft.StableStore

	store, err := NewBadgerStore(s.path)
	if err != nil {
		return err
	}

	logStore = store
	stableStore = store

	// Instantiate the Raft systems.

	fsm := &RedisFSM{}
	ra, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra

	if enableSingle {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		ra.BootstrapCluster(configuration)
	}

	return nil
}

type fsmSnapshot struct {
	persist func(sink raft.SnapshotSink) error
	release func()
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	return f.persist(sink)
}

func (f *fsmSnapshot) Release() {
	f.release()
}
