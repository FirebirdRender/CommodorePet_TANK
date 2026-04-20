package server

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	hub               *Hub
	handler           *WSHandler
	tokens            *TokenStore
	serverAddr        string
	enabled           bool
	botBinary         string
	maxConcurrentBots int

	mu     sync.Mutex
	active map[string]*BotAssignment
}

func NewBotManager(hub *Hub, handler *WSHandler, tokens *TokenStore, serverAddr string, maxBots int) *BotManager {
	enabled := os.Getenv("TANK_ENABLE_BOTS") == "1"
	binary, err := resolveBotBinary()
	if err != nil {
		log.Fatalf("[bot] failed to resolve binary: %v", err)
	}
	return &BotManager{
		hub:               hub,
		handler:           handler,
		tokens:            tokens,
		serverAddr:        serverAddr,
		enabled:           enabled,
		botBinary:         binary,
		maxConcurrentBots: maxBots,
		active:            make(map[string]*BotAssignment),
	}
}

func (bm *BotManager) IsEnabled() bool {
	return bm.enabled
}

// resolveBotBinary picks and validates the tank-bot executable.
// Returns an error for invalid paths (relative, traversal, outside allowed dirs).
func resolveBotBinary() (string, error) {
	if env := os.Getenv("TANK_BOT_BIN"); env != "" {
		if !filepath.IsAbs(env) {
			return "", fmt.Errorf("TANK_BOT_BIN must be absolute path, got: %s", env)
		}
		resolved, err := filepath.EvalSymlinks(env)
		if err != nil {
			return "", fmt.Errorf("TANK_BOT_BIN: %w", err)
		}
		if !isAllowedBotPath(resolved) {
			return "", fmt.Errorf("TANK_BOT_BIN must be in allowed directories: %s", resolved)
		}
		return resolved, nil
	}

	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "tank-bot")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		if _, err := os.Stat(candidate + ".exe"); err == nil {
			return candidate + ".exe", nil
		}
	}

	fallback := filepath.Join("bin", "tank-bot")
	return fallback, nil
}

func isAllowedBotPath(path string) bool {
	cwd, _ := os.Getwd()
	allowList := []string{
		filepath.Join(cwd, "bin"),
		filepath.Dir(os.Args[0]),
	}
	for _, allow := range allowList {
		rel, err := filepath.Rel(allow, path)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			return true
		}
	}
	return false
}

func (bm *BotManager) AssignBotToRoom(roomCode string, botClass string) error {
	return bm.AssignBotToRoomAt(roomCode, 1, botClass)
}

func (bm *BotManager) AssignBotToRoomAt(roomCode string, slot int, botClass string) error {
	if !bm.enabled {
		return fmt.Errorf("bot mode not available")
	}

	// B6: Concurrent bot subprocess cap
	if bm.maxConcurrentBots > 0 && bm.ConcurrentBotCount() >= bm.maxConcurrentBots {
		return fmt.Errorf("bot subprocess limit reached")
	}

	if slot < 0 || slot > 1 {
		return fmt.Errorf("invalid slot %d", slot)
	}

	room := bm.hub.GetRoom(roomCode)
	if room == nil {
		return fmt.Errorf("room not found: %s", roomCode)
	}

	if err := room.ReserveBotSeatAt(slot); err != nil {
		return fmt.Errorf("reserve bot seat: %w", err)
	}

	botID := fmt.Sprintf("bot-%s-%d-%d", botClass, slot, time.Now().UnixNano())
	botName := fmt.Sprintf("CPU%d", slot+1)
	if botClass != "" {
		botName = fmt.Sprintf("CPU%d-%s", slot+1, botClass)
	}

	skill := rand.IntN(10)
	playerID, err := room.AddBotPlayerAt(slot, botName, botID, botClass, skill)
	if err != nil {
		room.CancelBotReservationAt(slot)
		return fmt.Errorf("add bot player: %w", err)
	}

	token := bm.tokens.GenerateToken(roomCode, playerID, botName)

	if _, err := os.Stat(bm.botBinary); err != nil {
		room.CancelBotReservationAt(slot)
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
		room.CancelBotReservationAt(slot)
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

	key := botAssignmentKey(roomCode, slot)
	bm.mu.Lock()
	bm.active[key] = assignment
	bm.mu.Unlock()

	log.Printf("[bot] spawned %s pid=%d for room %s slot %d as player %d", bm.botBinary, cmd.Process.Pid, roomCode, slot, playerID)

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("[bot] subprocess for room %s slot %d exited: %v", roomCode, slot, err)
		} else {
			log.Printf("[bot] subprocess for room %s slot %d exited cleanly", roomCode, slot)
		}
		bm.releaseBotAt(roomCode, slot)
	}()

	return nil
}

func botAssignmentKey(roomCode string, slot int) string {
	return fmt.Sprintf("%s#%d", roomCode, slot)
}

func (bm *BotManager) ReleaseBot(roomCode string) {
	bm.releaseBotAt(roomCode, 1)
	bm.releaseBotAt(roomCode, 0)
}

func (bm *BotManager) releaseBotAt(roomCode string, slot int) {
	key := botAssignmentKey(roomCode, slot)
	bm.mu.Lock()
	assignment, ok := bm.active[key]
	if ok {
		delete(bm.active, key)
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

	log.Printf("[bot] released bot from room %s slot %d", roomCode, slot)
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

func (bm *BotManager) ConcurrentBotCount() int {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return len(bm.active)
}
