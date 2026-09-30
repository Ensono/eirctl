package lsp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
)

type TransportConfig struct {
	UseTCP bool
	Host   string
	Port   int
	Stdio  io.Reader
	Stdout io.Writer
	// OnListen is invoked with the bound address once the TCP listener is
	// ready. Primarily used by tests binding to an ephemeral port (Port: 0).
	OnListen func(addr net.Addr)
}

const name string = "eirctl-lsp"

var (
	ErrFailedToStartTCP   = errors.New(name + ": Failed to start TCP server")
	ErrFailedToStartStdio = errors.New(name + ": Failed to start stdio server")
)

func Init(ctx context.Context, log zerolog.Logger, config TransportConfig) error {
	if config.UseTCP {
		if err := serveTCP(ctx, config, log); err != nil {
			return fmt.Errorf("%w: %v", ErrFailedToStartTCP, err)
		}
		return nil
	}

	// create stdio/stdout LSP server - in process invocation
	server, err := NewServer(config.Stdio, config.Stdout, WithLogger(log), WithTransportConfig(config))
	if err != nil {
		return fmt.Errorf("%w", ErrFailedToStartStdio)
	}

	log.Info().Msg(name + ": Starting stdio language server...")

	if err := server.Serve(); err != nil {
		return fmt.Errorf(name + ": LSP server terminated")
	}
	return nil
}

// serveTCP creates a listener and serves an LSP server per client connection
// until ctx is cancelled.
func serveTCP(ctx context.Context, config TransportConfig, log zerolog.Logger) error {
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	if config.OnListen != nil {
		config.OnListen(listener.Addr())
	}

	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	log.Info().Msgf(name+": Starting TCP language server on %s", listener.Addr().String())

	for {
		conn, err := listener.Accept()
		if err != nil {
			wg.Wait()
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleConnection(conn, log)
		}()
	}
}
func handleConnection(conn net.Conn, log zerolog.Logger) {
	defer conn.Close()

	server, err := NewServer(conn, conn, WithLogger(log))
	if err != nil {
		log.Error().Err(err).Msg(name + ": connection setup failed")
		return
	}

	if err := server.Serve(); err != nil && !errors.Is(err, io.EOF) {
		log.Error().Err(err).Msg(name + ": connection terminated")
	}
}
