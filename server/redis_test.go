package server_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	anarchoredis "github.com/awinterman/anarchoredis/server"
)

func split(s string) []string {
	return strings.Split(s, " ")
}

func TestRun(t *testing.T) {
	// These are the args you would pass in on the command line
	os.Args = split("./anarchoredis")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := anarchoredis.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
}
