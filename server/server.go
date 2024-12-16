package server

import (
	"context"
	"log/slog"
	"net"
)

type ConnFunc func(context.Context, net.Conn) error

// Server creates a new server
type Server struct {
	config *Config

	l net.Listener

	connFunc ConnFunc

	log *slog.Logger
}

// New creates a new server
func New(ctx context.Context, config *Config, f ConnFunc) (*Server, error) {
	var lc = net.ListenConfig{}

	listener, err := lc.Listen(ctx, "tcp", config.Address)
	if err != nil {
		return nil, err
	}

	return &Server{config, listener, f, slog.Default()}, nil
}

// Serve serves at the configured value
func (r *Server) Serve(ctx context.Context) error {
	ctx, cancle := context.WithCancelCause(ctx)

	r.log.Info("listening", "addr", r.l.Addr().String(), "network", r.l.Addr().Network())
	go func() {
		<-ctx.Done()
		r.l.Close()
	}()

	for ctx.Err() == nil {
		conn, err := r.l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return context.Cause(ctx)
			}
			return err
		}
		r.log.Info("got conn", "local", conn.LocalAddr().String(), "remote", conn.RemoteAddr().String(), "network", conn.RemoteAddr().Network())

		go func() {
			err = r.connFunc(ctx, conn)
			if err != nil {
				r.log.Error("cancelling", "error", err)
				cancle(err)
			}
		}()

	}
	r.log.Info("listen loop  exited")

	return context.Cause(ctx)
}
