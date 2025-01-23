// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description:

// Package valkey:
package valkey

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
)

type Valkey struct {
	RedisAddress string

	cmd atomic.Pointer[exec.Cmd]
}

func (v *Valkey) GetRedisAddress() string {
	return v.RedisAddress
}

func (v *Valkey) Start(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		"valkey-server",
		`--save=""`,
		fmt.Sprintf(`--port=%s`, v.RedisAddress),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	v.cmd.Store(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

// Stop valkey
func (v *Valkey) Stop() error {
	cmd := v.cmd.Load()
	err := cmd.Cancel()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}
	v.cmd.Store(nil)
	return nil
}
