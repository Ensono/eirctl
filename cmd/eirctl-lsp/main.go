package main

import (
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/Ensono/eirctl/lang/lsp"
)

func main() {
	useTcp := flag.Bool("use-tcp", false, "enable TCP JSON-RPC mode")
	host := flag.String("host", "127.0.0.1", "host interface for TCP JSON-RPC mode")
	port := flag.Int("port", 11103, "TCP port for JSON-RPC mode")
	flag.Parse()

	if *useTcp {
		if err := serveTCP(*host, *port); err != nil {
			log.Fatal(err)
		}
		return
	}

	server, err := lsp.NewServer(os.Stdin, os.Stdout)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Starting eirctl language server...")
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}

func serveTCP(host string, port int) error {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("Starting eirctl language server on tcp://%s", address)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	server, err := lsp.NewServer(conn, conn)
	if err != nil {
		log.Printf("eirctl-lsp connection setup failed: %v", err)
		return
	}

	if err := server.Serve(); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("eirctl-lsp connection terminated: %v", err)
	}
}
