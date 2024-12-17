// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

package localstate

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/awinterman/anarchoredis/protocol"
	"github.com/dgraph-io/badger/v4"
	badgerpb "github.com/dgraph-io/badger/v4/pb"
)

type Store struct {
	DB      *badger.DB
	Log     *slog.Logger
	LockTTL time.Duration
}

var sentinel = []byte("OK")
var keyprefix = "anarcho:key:"

func (b Store) UnlockKeys(keys []string) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			err := txn.Delete([]byte(keyprefix + key))
			if err != nil {
				return err
			}
			b.Log.Debug("freeing", "key", key)
		}
		return nil
	})
}

func (b Store) AwaitUnlocked(ctx context.Context, cmd *protocol.Command) error {
	keys, err := cmd.Keys()
	if err != nil {
		return err
	}
	var waitFor []badgerpb.Match
	var waitForSet = map[string]struct{}{}
	err = b.DB.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			_, err := txn.Get([]byte(keyprefix + key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				b.Log.Debug("no lock", "key", key)
				continue
			} else if err != nil {
				return err
			}
			b.Log.Debug("locked", "key", key)

			waitFor = append(waitFor, badgerpb.Match{Prefix: []byte(key)})

			waitForSet[key] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil
	}

	if len(waitFor) == 0 {
		return nil
	}

	errDone := errors.New("done")

	err = b.DB.Subscribe(ctx, func(kv *badger.KVList) error {
		for _, k := range kv.GetKv() {
			key := strings.TrimPrefix(string(k.Key), keyprefix)

			if !bytes.Equal(k.GetValue(), sentinel) {
				b.Log.Debug("lock removed", "key", key)

				delete(waitForSet, key)
			}
		}
		if len(waitForSet) > 0 {
			return nil
		} else {
			return errDone
		}
	}, waitFor)

	if errors.Is(err, errDone) {
		return nil
	}

	return err
}

func (b Store) LockKeys(cmd *protocol.Command) error {
	keys, err := cmd.Keys()
	if err != nil {
		return err
	}

	return b.DB.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			b.Log.Debug("lock created", "key", key)
			err := txn.SetEntry(badger.NewEntry([]byte(key), sentinel).WithTTL(b.LockTTL))
			if err != nil {
				return err
			}
		}
		return nil
	})
}
