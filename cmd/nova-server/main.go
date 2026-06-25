// Package main is the entry point for the self-contained Nova workflow engine.
//
// Nova is an open-source workflow engine with built-in web UI.
// Use it as a standalone server, or embed the `nova` PDK into your own Go app.
//
// Usage:
//
//	nova-server --db /path/to/nova.db
//	nova-server --port 8080
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/liyan/nova/internal/app"
)

func main() {
	dbPath := flag.String("db", "nova.db", "path to SQLite database")
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	a, err := app.New(app.Config{
		DBPath: *dbPath,
	})
	if err != nil {
		log.Fatalf("app init: %v", err)
	}

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Nova workflow engine starting on http://localhost%s", addr)
	log.Printf("  PDK version: github.com/liyan/nova")
	log.Printf("  Database: %s", *dbPath)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		a.Close()
		os.Exit(0)
	}()

	if err := a.Serve(addr); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
