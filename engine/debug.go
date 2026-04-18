package engine

import (
	"fmt"
	"os"
	"sync"
)

var (
	debugFireMu      sync.Mutex
	debugFireEnabled = os.Getenv("TANK_FIRE_DEBUG") == "1"
)

func debugFire(playerID int, tank *Tank, outcome string) {
	if !debugFireEnabled {
		return
	}
	debugFireMu.Lock()
	defer debugFireMu.Unlock()
	fmt.Fprintf(os.Stderr, "FIRE p=%d pos=(%d,%d) dir=%d shotsLeft=%d outcome=%s\n",
		playerID, tank.X, tank.Y, tank.Dir, tank.ShotsLeft, outcome)
}
