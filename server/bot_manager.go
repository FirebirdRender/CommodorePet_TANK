package server

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type BotAssignment struct {
	RoomCode string
	BotID    string
	BotClass string
	Cmd      *exec.Cmd
	Done     chan struct{}
}

type BotManager struct {
	hub        *Hub
	handler    *WSHandler
	tokens     *TokenStore
	serverAddr string
	enabled    bool
	botBinary  string

	mu     sync.Mutex
	active map[string]*BotAssignment
}

func NewBotManager(hub *Hub, handler *WSHandler, tokens *TokenStore, serverAddr string) *BotManager {
	enabled := os.Getenv("TANK_ENABLE_BOTS") == "1"
	return &BotManager{
		hub:        hub,
		handler:    handler,
		tokens:     tokens,
		serverAddr: serverAddr,
		enabled:    enabled,
		botBinary:  resolveBotBinary(),
		active:     make(map[string]*BotAssignment),
	}
}

func (bm *BotManager) IsEnabled() bool {
	return bm.enabled
}

// resolveBotBinary picks the tank-bot executable in the following order:
//  1. TANK_BOT_BIN env var (explicit override)
//  2. Same directory as the running server binary
//  3. ./bin/tank-bot relative to the working directory
//
// On Windows, ".exe" is appended automatically by exec.LookPath when needed.
func resolveBotBinary() string {
	if env := os.Getenv("TANK_BOT_BIN"); env != "" {
		return env
	}

	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "tank-bot")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if _, err := os.Stat(candidate + ".exe"); err == nil {
			return candidate + ".exe"
		}
	}

	return filepath.Join("bin", "tank-bot")
}

func (bm *BotManager) AssignBotToRoom(roomCode string, botClass string) error {
	if !bm.enabled {
		return fmt.Errorf("bot mode not available")
	}

	room := bm.hub.GetRoom(roomCode)
	if room == nil {
		return fmt.Errorf("room not found: %s", roomCode)
	}

	if err := room.ReserveBotSeat(); err != nil {
		return fmt.Errorf("reserve bot seat: %w", err)
	}

	botID := fmt.Sprintf("bot-%s-%d", botClass, time.Now().UnixNano())
	botName := "CPU"
	if botClass != "" {
		botName = fmt.Sprintf("CPU-%s", botClass)
	}

	playerID, err := room.AddBotPlayer(botName, botID, botClass)
	if err != nil {
		room.CancelBotReservation()
		return fmt.Errorf("add bot player: %w", err)
	}

	token := bm.tokens.GenerateToken(roomCode, playerID, botName)

	if _, err := os.Stat(bm.botBinary); err != nil {
		room.CancelBotReservation()
		return fmt.Errorf("bot binary not found at %s (set TANK_BOT_BIN or run 'make bot'): %w", bm.botBinary, err)
	}

	wsURL := fmt.Sprintf("ws://%s/ws", bm.serverAddr)
	cmd := exec.Command(bm.botBinary,
		"-server", wsURL,
		"-room", roomCode,
		"-player-id", fmt.Sprintf("%d", playerID),
		"-token", token,
		"-name", botName,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		room.CancelBotReservation()
		return fmt.Errorf("start bot subprocess: %w", err)
	}

	done := make(chan struct{})
	assignment := &BotAssignment{
		RoomCode: roomCode,
		BotID:    botID,
		BotClass: botClass,
		Cmd:      cmd,
		Done:     done,
	}

	bm.mu.Lock()
	bm.active[roomCode] = assignment
	bm.mu.Unlock()

	log.Printf("[bot] spawned %s pid=%d for room %s as player %d", bm.botBinary, cmd.Process.Pid, roomCode, playerID)

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("[bot] subprocess for room %s exited: %v", roomCode, err)
		} else {
			log.Printf("[bot] subprocess for room %s exited cleanly", roomCode)
		}
		bm.ReleaseBot(roomCode)
	}()

	return nil
}

func (bm *BotManager) ReleaseBot(roomCode string) {
	bm.mu.Lock()
	assignment, ok := bm.active[roomCode]
	if ok {
		delete(bm.active, roomCode)
	}
	bm.mu.Unlock()

	if !ok {
		return
	}

	select {
	case <-assignment.Done:
	default:
		close(assignment.Done)
	}

	if assignment.Cmd != nil && assignment.Cmd.Process != nil {
		if assignment.Cmd.ProcessState == nil || !assignment.Cmd.ProcessState.Exited() {
			_ = assignment.Cmd.Process.Kill()
		}
	}

	log.Printf("[bot] released bot from room %s", roomCode)
}

func (bm *BotManager) HealthSnapshot() map[string]string {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	return map[string]string{
		"enabled":     fmt.Sprintf("%v", bm.enabled),
		"active_bots": fmt.Sprintf("%d", len(bm.active)),
		"bot_binary":  bm.botBinary,
	}
}
