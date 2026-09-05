package main

import (
	"flag"
	"os"

	"github.com/Ensono/eirctl/lang/lsp"
	"github.com/rs/zerolog"
)

func main() {
	useTcp := flag.Bool("use-tcp", false, "enable TCP JSON-RPC mode")
	host := flag.String("host", "127.0.0.1", "host interface for TCP JSON-RPC mode")
	port := flag.Int("port", 11103, "TCP port for JSON-RPC mode")
	flag.Parse()

	log := zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	if _, ok := os.LookupEnv("EIRCTL_LSP_DEBUG"); ok {
		log = log.Level(zerolog.DebugLevel)
	}
	transportConfig := lsp.TransportConfig{
		UseTCP: *useTcp,
		Host:   *host,
		Port:   *port,
	}

	if err := lsp.Init(log, transportConfig); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize LSP transport")
	}
}
