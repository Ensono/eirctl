package main

import (
	"context"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ensono/eirctl/lang/lsp"
	"github.com/rs/zerolog"
)

func runMain(in io.Reader, out, errOut io.Writer) int {
	useTcp := flag.Bool("use-tcp", false, "enable TCP JSON-RPC mode")
	host := flag.String("host", "127.0.0.1", "host interface for TCP JSON-RPC mode")
	port := flag.Int("port", 11103, "TCP port for JSON-RPC mode")
	flag.Parse()

	log := zerolog.New(errOut).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	if _, ok := os.LookupEnv("EIRCTL_LSP_DEBUG"); ok {
		log = log.Level(zerolog.DebugLevel)
	}

	ctx, stop := signal.NotifyContext(context.Background(), []os.Signal{os.Interrupt, syscall.SIGTERM, os.Kill}...)
	defer stop()

	transportConfig := lsp.TransportConfig{
		UseTCP: *useTcp,
		Host:   *host,
		Port:   *port,
		Stdio:  in,
		Stdout: out,
	}

	if err := lsp.Init(ctx, log, transportConfig); err != nil {
		log.Error().Err(err).Msg("Failed to initialize LSP transport")
		return 1
	}
	return 0
}

func main() {
	os.Exit(runMain(os.Stdin, os.Stdout, os.Stderr))
}
