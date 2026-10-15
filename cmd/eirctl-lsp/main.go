package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ensono/eirctl/lang/lsp"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
)

func runMain(in io.Reader, out, errOut io.Writer) int {
	f := pflag.NewFlagSet("eirctl-lsp", pflag.ContinueOnError)
	useTcp := f.BoolP("use-tcp", "", false, "enable TCP JSON-RPC mode")
	enableDebug := f.BoolP("debug", "", false, "enable debug logging")
	host := f.StringP("host", "h", "127.0.0.1", "host interface for TCP JSON-RPC mode")
	port := f.IntP("port", "p", 11103, "TCP port for JSON-RPC mode")
	skipDir := f.StringArrayP("skip-dir", "", []string{lsp.SkipHidden}, "skip directories matching the given patterns, defaults to all hidden directories")

	if err := f.Parse(os.Args[1:]); err != nil {
		return 2
	}

	log := zerolog.New(errOut).With().Timestamp().Logger().Level(zerolog.InfoLevel)

	if _, ok := os.LookupEnv("EIRCTL_LSP_DEBUG"); ok || *enableDebug {
		log = log.Level(zerolog.DebugLevel)
	}

	ctx, stop := signal.NotifyContext(context.Background(), []os.Signal{os.Interrupt, syscall.SIGTERM, os.Kill}...)
	defer stop()

	transportConfig := lsp.Config{
		UseTCP:   *useTcp,
		Host:     *host,
		Port:     *port,
		SkipDirs: *skipDir,
		Stdio:    in,
		Stdout:   out,
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
