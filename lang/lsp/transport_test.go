package lsp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Ensono/eirctl/lang/lsp"
	"github.com/rs/zerolog"
)

func Test_Serve_Process(t *testing.T) {

	t.Run("serve process", func(t *testing.T) {

		in, out := &bytes.Buffer{}, &bytes.Buffer{}
		log := zerolog.New(out).With().Timestamp().Logger().Level(zerolog.InfoLevel)

		writeFramed(t, in, `{"jsonrpc":"2.0","method":"exit"}`)
		err := lsp.Init(log, lsp.TransportConfig{Stdio: in, Stdout: out})
		if err != nil {
			t.Fatalf("Failed to initialize LSP transport: %v", err)
		}
	})

	t.Run("serve TCP", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		lo := &bytes.Buffer{}
		log := zerolog.New(lo)
		addrCh := make(chan net.Addr, 1)
		done := make(chan error, 1)

		go func() {
			done <- lsp.Init(log, lsp.TransportConfig{
				UseTCP:   true,
				Host:     "127.0.0.1",
				Port:     0, // ephemeral
				Ctx:      ctx,
				OnListen: func(addr net.Addr) { addrCh <- addr },
			})
		}()

		var addr net.Addr
		select {
		case addr = <-addrCh:
		case <-time.After(2 * time.Second):
			t.Fatal("listener did not start")
		}

		conn, err := net.Dial("tcp", addr.String())
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		defer conn.Close()

		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatalf("set deadline failed: %v", err)
		}

		writeFramed(t, conn, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)

		reader := bufio.NewReader(conn)
		payload, err := lsp.ReadMessage(reader)
		if err != nil {
			t.Fatalf("read message failed: %v", err)
		}

		var response struct {
			Result struct {
				Capabilities map[string]any `json:"capabilities"`
			} `json:"result"`
		}
		if err := json.Unmarshal(payload, &response); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if len(response.Result.Capabilities) == 0 {
			t.Fatalf("expected capabilities, got: %s", payload)
		}

		writeFramed(t, conn, `{"jsonrpc":"2.0","method":"exit"}`)

		// The server closes the connection once its Serve loop returns.
		if _, err := reader.ReadByte(); err != io.EOF {
			t.Fatalf("expected EOF after exit, got: %v", err)
		}

		// The accept loop only stops when the context is cancelled.
		cancel()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Init returned error: %v", err)
			}
			if lo.String() != "" && strings.Contains(lo.String(), "error") {
				t.Errorf("Server log output contains an error:\n%s", lo.String())
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for transport shutdown")
		}
	})
}

func writeFramed(t *testing.T, writer io.Writer, payload string) {
	t.Helper()
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n%s", len(payload), payload); err != nil {
		t.Errorf("Failed to write framed payload: %v", err)
	}
}
