package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	tokens := server.NewTokenStore()
	handler := server.NewWSHandler(hub, tokens)
	roomAPI := server.NewRoomAPI(hub, handler.Registry(), tokens)

	serverAddr := *addr
	if !strings.Contains(serverAddr, ":") {
		serverAddr = "localhost:" + strings.TrimPrefix(serverAddr, ":")
	}
	botManager := server.NewBotManager(hub, handler, tokens, serverAddr)
	roomAPI.SetBotManager(botManager)

	if botManager.IsEnabled() {
		log.Println("[bot] bot mode enabled via TANK_ENABLE_BOTS=1")
	} else {
		log.Println("[bot] bot mode disabled (set TANK_ENABLE_BOTS=1 to enable)")
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	roomAPI.RegisterRoutes(mux)

	fileHandler := server.WASMNoCacheMiddleware(http.FileServer(http.Dir(*dir)))
	mux.Handle("/", fileHandler)

	wrappedMux := server.CORSMiddleware(*cors, mux)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      wrappedMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			hub.CleanupStaleRooms(*maxRoomAge)
			tokens.CleanupStaleTokens(*maxRoomAge)
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
