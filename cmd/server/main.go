package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FirebirdRender/CommodorePet_TANK/server"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address (use :0 for OS-assigned port; resolved port logged as 'listening on :PORT')")
	dir := flag.String("dir", "web", "static files directory")
	cors := flag.String("cors", "*", "CORS allowed origin")
	maxRoomAge := flag.Duration("max-room-age", 30*time.Minute, "stale room cleanup interval")
	pprofAddr := flag.String("pprof-addr", "", "if non-empty, expose net/http/pprof on this address (e.g. :6060) for goroutine-leak detection")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}
	resolvedAddr := ln.Addr().String()
	_, resolvedPort, splitErr := net.SplitHostPort(resolvedAddr)
	if splitErr != nil {
		resolvedPort = resolvedAddr
	}

	hub := server.NewHub()
	tokens := server.NewTokenStore()
	handler := server.NewWSHandler(hub, tokens)
	roomAPI := server.NewRoomAPI(hub, handler.Registry(), tokens)

	serverAddr := "localhost:" + resolvedPort
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
		Handler:      wrappedMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if *pprofAddr != "" {
		go func() {
			log.Printf("[pprof] listening on %s (handlers auto-registered on http.DefaultServeMux)", *pprofAddr)
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				log.Printf("[pprof] server error: %v", err)
			}
		}()
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

		hub.Shutdown(30*time.Second, handler.Registry())

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP shutdown error: %v", err)
		}
	}()

	log.Printf("TANK! server (v%s) listening on :%s", server.AppVersion, resolvedPort)
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("server stopped")
}
