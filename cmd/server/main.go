package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FirebirdRender/CommodorePet_TANK/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dir := flag.String("dir", "web", "static files directory")
	cors := flag.String("cors", "*", "CORS allowed origin")
	maxRoomAge := flag.Duration("max-room-age", 30*time.Minute, "stale room cleanup interval")
	flag.Parse()

	hub := server.NewHub()
	handler := server.NewWSHandler(hub)

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	fileHandler := server.WASMNoCacheMiddleware(http.FileServer(http.Dir(*dir)))
	mux.Handle("/", fileHandler)

	wrappedMux := server.CORSMiddleware(*cors, mux)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      wrappedMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			hub.CleanupStaleRooms(*maxRoomAge)
		}
	}()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")

		// Notify players and drain matches (30s)
		hub.Shutdown(30*time.Second, handler.Registry())

		// Stop HTTP server (15s)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP shutdown error: %v", err)
		}
	}()

	log.Printf("TANK! server listening on %s", *addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("server stopped")
}
