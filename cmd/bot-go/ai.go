package main

import (
	"fmt"
	"math/rand"

	botsdk "github.com/FirebirdRender/CommodorePet_TANK/bot-sdk-go"
)

type InputAction struct {
	Key    string
	Action string
}

const (
	cellEmpty   = 1
	cellBarrel1 = 5
	cellBarrel2 = 6
)

type pendingKind int

const (
	pendingNone pendingKind = iota
	pendingMove
	pendingFire
	pendingMine
)

func (pk pendingKind) String() string {
	switch pk {
	case pendingMove:
		return "move"
	case pendingFire:
		return "fire"
	case pendingMine:
		return "mine"
	default:
		return "idle"
	}
}

type pending struct {
	kind        pendingKind
	key         string
	emittedTick uint64
	beforeX     int
	beforeY     int
	beforeDir   int
	beforeShots int
	beforeMines int
}

type BotState struct {
	MyID       int
	Difficulty int
	GridW      int
	GridH      int

	grid [][]int

	pending pending

	heldMove string
	heldFire bool
	heldMine bool

	// Hysteresis for movement: require the same desired direction to be picked
	// for hysteresisTicks consecutive Decide() calls before committing to it.
	// Eliminates the per-tick flip-flop that arises when dx and dy are nearly
	// equal and one becomes blocked, causing primary/perpendicular swap.
	lastDesiredDir     string
	desiredStableTicks int

	// Thinking-delay throttle. nextActionTick is the earliest tick at which
	// startNewAction is allowed to emit a NEW intent (commitMove / fire-down /
	// mine-down). Pending-resolution traffic (release-up emitted by checkAck
	// or by startNewAction's heldFire/heldMine drain) is NOT gated — those
	// finish the prior action and must flow at engine cadence to keep the
	// closed-loop SM honest. Difficulty 10 → delayTicks=0 (no gate); difficulty
	// 1 → delayTicks=60 (1.0s between every new action including move
	// re-commits, per user request "Delay both before action AND between move
	// re-commits").
	nextActionTick uint64

	lastLogLine string
	decideCount int
}

func NewBotState(playerID, difficulty, gridW, gridH int) *BotState {
	bs := &BotState{
		MyID:       playerID,
		Difficulty: difficulty,
		GridW:      gridW,
		GridH:      gridH,
	}
	bs.grid = make([][]int, gridH)
	for i := range bs.grid {
		bs.grid[i] = make([]int, gridW)
	}
	return bs
}

func (bs *BotState) LoadKeyframe(grid [][]int) {
	if len(grid) == 0 {
		return
	}
	bs.grid = make([][]int, len(grid))
	for y := range grid {
		bs.grid[y] = make([]int, len(grid[y]))
		copy(bs.grid[y], grid[y])
	}
	bs.GridH = len(grid)
	if bs.GridH > 0 {
		bs.GridW = len(grid[0])
	}
}

func (bs *BotState) ApplyDelta(changes []botsdk.CellChange) {
	for _, c := range changes {
		if c.Y >= 0 && c.Y < len(bs.grid) && c.X >= 0 && c.X < len(bs.grid[c.Y]) {
			bs.grid[c.Y][c.X] = c.Cell
		}
	}
}

const (
	moveAckTimeoutTicks    = 30
	fireAckTimeoutTicks    = 10
	mineAckTimeoutTicks    = 10
	hysteresisTicks        = 2
	thinkingDelayStepTicks = 6
	thinkingDelayMaxLevel  = 10
)

func thinkingDelayTicks(difficulty int) uint64 {
	if difficulty >= thinkingDelayMaxLevel {
		return 0
	}
	if difficulty < 1 {
		difficulty = 1
	}
	return uint64((thinkingDelayMaxLevel - difficulty) * thinkingDelayStepTicks)
}

// Direction constants mirror engine/constants.go; the SDK's TankInfo.Dir
// carries the engine's int value. Local copy avoids importing the engine
// package into the bot binary.
const (
	dirUp    = 1
	dirDown  = 2
	dirLeft  = 3
	dirRight = 4
)

func dirToward(myX, myY, exX, exY int) int {
	dx := exX - myX
	dy := exY - myY
	if dx == 0 && dy == 0 {
		return 0
	}
	if abs(dx) >= abs(dy) {
		if dx > 0 {
			return dirRight
		}
		if dx < 0 {
			return dirLeft
		}
	}
	if dy > 0 {
		return dirDown
	}
	if dy < 0 {
		return dirUp
	}
	return 0
}

func dirToKey(d int) string {
	switch d {
	case dirUp:
		return "up"
	case dirDown:
		return "down"
	case dirLeft:
		return "left"
	case dirRight:
		return "right"
	default:
		return ""
	}
}

// Decide implements the closed-loop state machine:
//   - If an action is pending, check for ack (expected state delta) and either
//     clear pending + emit a release for any still-held key, or on timeout
//     release + reset. Until one of those fires, emit nothing.
//   - If idle, choose one intent (fire > mine > move) and emit exactly one
//     key-down event, recording what we expect to change.
//
// This enforces the original PET "one action at a time, wait for confirmation"
// contract and keeps outbound rate well under the server's 120/sec input-flood
// guard by never emitting more than one event per tick.
func (bs *BotState) Decide(tick uint64, tanks [2]botsdk.TankInfo) []InputAction {
	bs.decideCount++
	my, enemy := bs.findTanks(tanks)

	if my == nil || !my.Active {
		if bs.pending.kind != pendingNone || bs.heldMove != "" || bs.heldFire || bs.heldMine {
			return bs.releaseAllHeld("tank_inactive")
		}
		return nil
	}

	if bs.pending.kind != pendingNone {
		if acted, release := bs.checkAck(my, tick); acted {
			return release
		}
	}

	// Fire-interrupt: if a move is pending but we now have a clean shot
	// already pointing the right way, abort the move (release held key) so
	// next tick startNewAction can fire. Without this preemption the bot
	// stays locked in move-pending across the entire alignment window —
	// MoveDelay at low difficulty (~33 ticks) far exceeds the window in
	// which both tanks share a row/column, so fire opportunities are
	// systematically missed (root cause of 0.9.4 "bot never fires" bug).
	if bs.pending.kind == pendingMove && enemy != nil && enemy.Active &&
		my.ShotsLeft > 0 && bs.canFireWithLOS(my, enemy) {
		desiredFireDir := dirToward(my.X, my.Y, enemy.X, enemy.Y)
		if desiredFireDir != 0 && my.Dir == desiredFireDir {
			bs.clearPending("fire_interrupt")
			if bs.heldMove != "" {
				key := bs.heldMove
				bs.heldMove = ""
				return []InputAction{{Key: key, Action: "up"}}
			}
		}
	}

	if bs.pending.kind != pendingNone {
		return nil
	}

	return bs.startNewAction(tick, my, enemy)
}

func (bs *BotState) checkAck(my *botsdk.TankInfo, tick uint64) (acted bool, release []InputAction) {
	p := bs.pending
	age := tick - p.emittedTick

	switch p.kind {
	case pendingMove:
		moved := my.X != p.beforeX || my.Y != p.beforeY
		rotated := my.Dir != p.beforeDir
		if moved || rotated {
			bs.clearPending("move_ack")
			if bs.heldMove != "" {
				key := bs.heldMove
				bs.heldMove = ""
				return true, []InputAction{{Key: key, Action: "up"}}
			}
			return true, nil
		}
		if age >= moveAckTimeoutTicks {
			bs.clearPending("move_timeout")
			return true, bs.releaseAllHeld("move_timeout")
		}
	case pendingFire:
		fired := my.ShotsLeft < p.beforeShots
		if fired {
			bs.clearPending("fire_ack")
			if bs.heldFire {
				bs.heldFire = false
				return true, []InputAction{{Key: "fire", Action: "up"}}
			}
			return true, nil
		}
		if age >= fireAckTimeoutTicks {
			bs.clearPending("fire_timeout")
			return true, bs.releaseAllHeld("fire_timeout")
		}
	case pendingMine:
		dropped := my.MinesLeft < p.beforeMines
		if dropped {
			bs.clearPending("mine_ack")
			if bs.heldMine {
				bs.heldMine = false
				return true, []InputAction{{Key: "mine", Action: "up"}}
			}
			return true, nil
		}
		if age >= mineAckTimeoutTicks {
			bs.clearPending("mine_timeout")
			return true, bs.releaseAllHeld("mine_timeout")
		}
	}
	return false, nil
}

func (bs *BotState) startNewAction(tick uint64, my, enemy *botsdk.TankInfo) []InputAction {
	if bs.heldFire {
		bs.heldFire = false
		return []InputAction{{Key: "fire", Action: "up"}}
	}
	if bs.heldMine {
		bs.heldMine = false
		return []InputAction{{Key: "mine", Action: "up"}}
	}

	if tick < bs.nextActionTick {
		return nil
	}

	if enemy != nil && enemy.Active && bs.canFireWithLOS(my, enemy) {
		desiredFireDir := dirToward(my.X, my.Y, enemy.X, enemy.Y)
		if desiredFireDir != 0 && my.Dir != desiredFireDir {
			key := dirToKey(desiredFireDir)
			if key != "" {
				return bs.commitMove(tick, my, key)
			}
		}
		bs.pending = pending{
			kind:        pendingFire,
			key:         "fire",
			emittedTick: tick,
			beforeX:     my.X,
			beforeY:     my.Y,
			beforeDir:   my.Dir,
			beforeShots: my.ShotsLeft,
			beforeMines: my.MinesLeft,
		}
		bs.heldFire = true
		bs.nextActionTick = tick + thinkingDelayTicks(bs.Difficulty)
		return []InputAction{{Key: "fire", Action: "down"}}
	}

	if enemy != nil && enemy.Active && my.MinesLeft > 0 {
		dist := abs(my.X-enemy.X) + abs(my.Y-enemy.Y)
		if dist < 3 && rand.Float64() < 0.05 {
			bs.pending = pending{
				kind:        pendingMine,
				key:         "mine",
				emittedTick: tick,
				beforeX:     my.X,
				beforeY:     my.Y,
				beforeDir:   my.Dir,
				beforeShots: my.ShotsLeft,
				beforeMines: my.MinesLeft,
			}
			bs.heldMine = true
			bs.nextActionTick = tick + thinkingDelayTicks(bs.Difficulty)
			return []InputAction{{Key: "mine", Action: "down"}}
		}
	}

	var desired string
	if enemy != nil && enemy.Active {
		desired = bs.moveTowardEnemy(my, enemy)
	} else {
		desired = bs.fallbackMove(my.X, my.Y)
	}
	if desired == "" {
		bs.lastDesiredDir = ""
		bs.desiredStableTicks = 0
		return nil
	}

	if desired == bs.lastDesiredDir {
		bs.desiredStableTicks++
	} else {
		bs.lastDesiredDir = desired
		bs.desiredStableTicks = 1
	}
	if bs.desiredStableTicks < hysteresisTicks && bs.heldMove != desired {
		return nil
	}

	return bs.commitMove(tick, my, desired)
}

// commitMove emits the key-down for `key`, releasing any other held move key
// first. Records pending so checkAck() can confirm via either position OR
// direction change (rotation-only inputs do not move the tank).
//
// If `key` is already held, returns nil with no state change — the prior
// pending entry remains the source of truth and will resolve via ack/timeout.
// Re-emitting a held key would (a) duplicate input and (b) reset the ack
// window every tick, masking timeouts forever.
func (bs *BotState) commitMove(tick uint64, my *botsdk.TankInfo, key string) []InputAction {
	if bs.heldMove == key {
		return nil
	}
	if bs.heldMove != "" {
		old := bs.heldMove
		bs.heldMove = ""
		return []InputAction{{Key: old, Action: "up"}}
	}
	if tick < bs.nextActionTick {
		return nil
	}
	bs.pending = pending{
		kind:        pendingMove,
		key:         key,
		emittedTick: tick,
		beforeX:     my.X,
		beforeY:     my.Y,
		beforeDir:   my.Dir,
		beforeShots: my.ShotsLeft,
		beforeMines: my.MinesLeft,
	}
	bs.heldMove = key
	bs.nextActionTick = tick + thinkingDelayTicks(bs.Difficulty)
	return []InputAction{{Key: key, Action: "down"}}
}

func (bs *BotState) clearPending(reason string) {
	bs.lastLogLine = fmt.Sprintf("pending_cleared=%s kind=%s", reason, bs.pending.kind)
	bs.pending = pending{}
}

func (bs *BotState) releaseAllHeld(reason string) []InputAction {
	out := make([]InputAction, 0, 3)
	if bs.heldFire {
		out = append(out, InputAction{Key: "fire", Action: "up"})
		bs.heldFire = false
	}
	if bs.heldMine {
		out = append(out, InputAction{Key: "mine", Action: "up"})
		bs.heldMine = false
	}
	if bs.heldMove != "" {
		out = append(out, InputAction{Key: bs.heldMove, Action: "up"})
		bs.heldMove = ""
	}
	if len(out) > 0 {
		bs.lastLogLine = fmt.Sprintf("release_all=%s count=%d", reason, len(out))
	}
	return out
}

func (bs *BotState) findTanks(tanks [2]botsdk.TankInfo) (my, enemy *botsdk.TankInfo) {
	for i := range tanks {
		t := tanks[i]
		if t.PlayerID == bs.MyID {
			m := t
			my = &m
		} else {
			e := t
			enemy = &e
		}
	}
	return
}

// canFireWithLOS returns true iff shots remain, we're strictly same-row or
// same-column with the enemy, and every cell strictly between us is empty
// floor. Walls, barrels, wreckage, and mines all block the shot.
func (bs *BotState) canFireWithLOS(my, enemy *botsdk.TankInfo) bool {
	if my.ShotsLeft <= 0 {
		return false
	}
	if my.Y == enemy.Y {
		y := my.Y
		x1, x2 := my.X, enemy.X
		if x1 > x2 {
			x1, x2 = x2, x1
		}
		for x := x1 + 1; x < x2; x++ {
			if !bs.isOpen(x, y) {
				return false
			}
		}
		return true
	}
	if my.X == enemy.X {
		x := my.X
		y1, y2 := my.Y, enemy.Y
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		for y := y1 + 1; y < y2; y++ {
			if !bs.isOpen(x, y) {
				return false
			}
		}
		return true
	}
	return false
}

func (bs *BotState) isOpen(x, y int) bool {
	if y < 0 || y >= len(bs.grid) {
		return false
	}
	if x < 0 || x >= len(bs.grid[y]) {
		return false
	}
	return bs.grid[y][x] == cellEmpty
}

func (bs *BotState) moveTowardEnemy(my, enemy *botsdk.TankInfo) string {
	dx := enemy.X - my.X
	dy := enemy.Y - my.Y

	var primary string
	if abs(dx) > abs(dy) {
		if dx > 0 {
			primary = "right"
		} else {
			primary = "left"
		}
	} else if dy != 0 {
		if dy > 0 {
			primary = "down"
		} else {
			primary = "up"
		}
	} else if dx != 0 {
		if dx > 0 {
			primary = "right"
		} else {
			primary = "left"
		}
	} else {
		return bs.fallbackMove(my.X, my.Y)
	}

	if !bs.isWall(my.X, my.Y, primary) {
		return primary
	}

	var perp string
	if primary == "up" || primary == "down" {
		if dx >= 0 {
			perp = "right"
		} else {
			perp = "left"
		}
	} else {
		if dy >= 0 {
			perp = "down"
		} else {
			perp = "up"
		}
	}
	if !bs.isWall(my.X, my.Y, perp) {
		return perp
	}
	return bs.fallbackMove(my.X, my.Y)
}

func (bs *BotState) isWall(x, y int, direction string) bool {
	nx, ny := x, y
	switch direction {
	case "up":
		ny--
	case "down":
		ny++
	case "left":
		nx--
	case "right":
		nx++
	case "up_left":
		nx--
		ny--
	case "up_right":
		nx++
		ny--
	case "down_left":
		nx--
		ny++
	case "down_right":
		nx++
		ny++
	default:
		return true
	}
	if nx < 0 || nx >= bs.GridW || ny < 0 || ny >= bs.GridH {
		return true
	}
	if ny >= len(bs.grid) || nx >= len(bs.grid[ny]) {
		return true
	}
	c := bs.grid[ny][nx]
	if c == cellEmpty {
		return false
	}
	// Own barrel never blocks own tank movement (engine game rule). The first
	// step of any directional input rotates the tank, vacating the old barrel
	// cell before the next step occurs, so treating own barrel as walkable
	// prevents the bot from being trapped against its own barrel cell.
	myBarrel := cellBarrel1
	if bs.MyID == 2 {
		myBarrel = cellBarrel2
	}
	if c == myBarrel {
		return false
	}
	return true
}

func (bs *BotState) fallbackMove(x, y int) string {
	directions := []string{"up", "down", "left", "right"}
	start := rand.Intn(len(directions))
	for i := range directions {
		d := directions[(start+i)%len(directions)]
		if !bs.isWall(x, y, d) {
			return d
		}
	}
	return ""
}

func (bs *BotState) PendingDesc() string {
	p := bs.pending
	if p.kind == pendingNone {
		return "idle"
	}
	return fmt.Sprintf("%s/%s@%d", p.kind, p.key, p.emittedTick)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
