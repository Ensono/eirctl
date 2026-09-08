package lsp

import (
	"errors"
	"io"
	"net"
	"os"
	"strconv"

	"github.com/rs/zerolog"
)

type TransportConfig struct {
	UseTCP bool
	Host   string
	Port   int
}

const name string = "eirctl-lsp"

func Init(log zerolog.Logger, config TransportConfig) error {

	if config.UseTCP {
		if err := serveTCP(config.Host, config.Port, log); err != nil {
			log.Fatal().Err(err).Msg(name + ": Failed to start TCP server")
		}
		return nil
	}

	// create stdio/stdout LSP server - in process invocation
	server, err := NewServer(os.Stdin, os.Stdout, WithLogger(log), WithTransportConfig(config))
	if err != nil {
		log.Fatal().Err(err).Msg(name + ": Failed to create LSP server")
	}

	log.Info().Msg(name + ": Starting stdio language server...")

	if err := server.Serve(); err != nil {
		log.Fatal().Err(err).Msg(name + ": LSP server terminated unexpectedly")
	}
	return nil
}

// create TCP Listener per client connection and serve the LSP server over that connection
func serveTCP(host string, port int, log zerolog.Logger) error {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Info().Msgf(name+": Starting language server on tcp://%s", address)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go handleConnection(conn, log)
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
		log.Fatal().Err(err).Msg(name + ": connection terminated")
	}
}
