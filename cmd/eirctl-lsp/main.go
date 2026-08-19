package main

import (
	"log"
	"os"

	"github.com/Ensono/eirctl/lang/lsp"
)

func main() {
	server, err := lsp.NewServer(os.Stdin, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
