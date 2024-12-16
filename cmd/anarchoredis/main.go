package main

import (
	"context"
	"log/slog"
	"os"

	anarchoredis "anarchoredis/server"
)

func main() {
	ctx := context.Background()
	err := anarchoredis.Run(ctx)
	if err != nil {
		slog.Error("exiting;", "error", err)
		os.Exit(1)
	}
}
