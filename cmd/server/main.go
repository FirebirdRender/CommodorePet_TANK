package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/FirebirdRender/CommodorePet_TANK/server"
	"golang.org/x/time/rate"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address (use :0 for OS-assigned port; resolved port logged as 'listening on :PORT')")
	dir := flag.String("dir", "web", "static files directory")
	cors := flag.String("cors", "https://localhost:8080", "CORS allowed origin")
	maxRoomAge := flag.Duration("max-room-age", 30*time.Minute, "stale room cleanup interval")
	allowedOrigins := flag.String("allowed-origins", "", "comma-separated WebSocket origin patterns (host-only, e.g. 'localhost:8080,*.example.com'); empty = allow all (dev only)")
	maxConns := flag.Int64("max-conns", 1000, "maximum concurrent WebSocket connections (0 = unlimited)")
	maxRooms := flag.Int("max-rooms", 500, "maximum concurrent rooms (0 = unlimited)")
	maxBots := flag.Int("max-bots", 20, "maximum concurrent bot subprocesses (0 = unlimited)")
	rateLimit := flag.Float64("rate-requests", 5, "API rate limit: requests per second per IP")
	rateBurst := flag.Int("rate-burst", 10, "API rate limit: burst size per IP")
	debugAddr := flag.String("debug-addr", "", "debug pprof address (dev only, no auth)")
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

	var originPatterns []string
	if *allowedOrigins != "" {
		originPatterns = strings.Split(*allowedOrigins, ",")
		for i := range originPatterns {
			originPatterns[i] = strings.TrimSpace(originPatterns[i])
		}
	}

	hub := server.NewHub(*maxRooms)
	tokens := server.NewTokenStore()
	handler := server.NewWSHandler(hub, tokens, originPatterns, *maxConns)
	roomAPI := server.NewRoomAPI(hub, handler.Registry(), tokens, *cors)
	roomAPI.SetStaticDir(*dir)

	serverAddr := "localhost:" + resolvedPort
	botManager := server.NewBotManager(hub, handler, tokens, serverAddr, *maxBots)
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

	rateLimiter := server.NewRateLimiter(rate.Limit(*rateLimit), *rateBurst)

	// Order: RateLimit → CORS → SecurityHeaders → mux
	// N4: Server-level WriteTimeout (300s) caps API responses; SSE/WS have own timeouts
	wrappedMux := rateLimiter.Middleware(
		server.SecurityHeadersMiddleware(
			server.CORSMiddleware(*cors, mux),
		),
	)

	srv := &http.Server{
		Handler:      wrappedMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if *debugAddr != "" {
		go func() {
			log.Printf("[debug] pprof on %s (no auth — dev only!)", *debugAddr)
			if err := http.ListenAndServe(*debugAddr, nil); err != nil {
				log.Printf("[debug] pprof error: %v", err)
			}
		}()
	}

	// Cleanup goroutine for stale rooms and tokens
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			hub.CleanupStaleRooms(*maxRoomAge)
			tokens.CleanupStaleTokens(*maxRoomAge)
		}
	}()

	// Shutdown goroutine
	shutdownCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		select {
		case <-shutdownCh:
			return
		default:
			close(shutdownCh)
		}

		hub.Shutdown(30*time.Second, handler.Registry())

		// Stop accepting new connections and unblock srv.Serve so the process exits.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("http server shutdown: %v", err)
		}
	}()

	// Bot auto-room goroutine (stops when shutdownCh is closed)
	if botManager.IsEnabled() {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-shutdownCh:
					return
				case <-ticker.C:
					if hub.GetBotMatchCount() > 0 {
						continue
					}
					room := hub.CreateRoomWithBotPolicy(5, true, false, 0)
					if room == nil {
						continue
					}
					if err := room.ReserveBotSeatAt(0); err != nil {
						hub.RemoveRoom(room.Code)
						continue
					}
					if err := room.ReserveBotSeatAt(1); err != nil {
						hub.RemoveRoom(room.Code)
						continue
					}
					if err := botManager.AssignBotToRoomAt(room.Code, 0, "mvp"); err != nil {
						log.Printf("[bot] failed to assign bot-0 to room %s: %v", room.Code, err)
						hub.RemoveRoom(room.Code)
						continue
					}
					if err := botManager.AssignBotToRoomAt(room.Code, 1, "mvp"); err != nil {
						log.Printf("[bot] failed to assign bot-1 to room %s: %v", room.Code, err)
						hub.RemoveRoom(room.Code)
						continue
					}
					log.Printf("[bot] auto-created bot-vs-bot room %s (difficulty 5)", room.Code)
				}
			}
		}()
	}

	log.Printf("TANK! server (v%s) listening on :%s", server.AppVersion, resolvedPort)
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("server stopped")
}
