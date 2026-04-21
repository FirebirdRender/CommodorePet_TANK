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
	cellWall    = 2
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

	// One-shot flag: P2 gets a +1 tick offset on its very first commitMove to
	// break the symmetric phase-lock that arises when both bots are spawned
	// mirror-image with identical seeds and identical difficulty. Without the
	// offset both bots tick in lockstep through rotate→gate→rotate, never
	// reaching a fire opportunity. After the first decision, normal scheduling
	// (driven by ack timing + jitter from grid asymmetry) keeps them desynced.
	firstActionDone bool

	// G-3 counters: per-match action telemetry, read by cmd/bot-go/main.go
	// at GameOver to write the summary JSON consumed by cmd/bot-load. Counts
	// emitted intents (fire/mine key-down, committed move key-down), not
	// engine-confirmed effects. Increment sites: commitMove (move), fire path
	// in startNewAction (fire), mine path in startNewAction (mine).
	FireCount uint64
	MoveCount uint64
	MineCount uint64

	lastLogLine string
	decideCount int

	skillConfig struct {
		ThinkingDelayTicks          uint64
		MinShotsForFire             int
		EnableSelfDestructAwareness bool
		EnableWallShootingUnstick   bool
		StuckThreshold              int
		EnableWallShootingHunt      bool
		EnableProtectile            bool
		ProtectileScalar            int
		EnableEvasion               bool
		EngageRange                 int
		DetectionRangeScalar        float64
	}

	// Stuck detection: track when bot hasn't moved for StuckThreshold decisions
	stuckCounter int
	stuckX       int
	stuckY       int

	// Protectile dodge: snapshot of shots from most recent full keyframe (OnTick).
	// OnTickDelta carries no shots so we reuse the last snapshot (up to 1 tick stale).
	lastShots []botsdk.ShotInfo
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
	// Initialize skill config from table
	skill := _SKILL_TABLE[difficulty]
	bs.skillConfig.ThinkingDelayTicks = skill.ThinkingDelayTicks
	bs.skillConfig.MinShotsForFire = skill.MinShotsForFire
	bs.skillConfig.EnableWallShootingUnstick = skill.EnableWallShootingUnstick
	bs.skillConfig.StuckThreshold = skill.StuckThreshold
	bs.skillConfig.EnableWallShootingHunt = skill.EnableWallShootingHunt
	bs.skillConfig.EnableProtectile = skill.EnableProtectile
	bs.skillConfig.ProtectileScalar = skill.ProtectileScalar
	bs.skillConfig.EnableEvasion = skill.EnableEvasion
	bs.skillConfig.EngageRange = skill.EngageRange
	bs.skillConfig.DetectionRangeScalar = skill.DetectionRangeScalar
	bs.skillConfig.EnableSelfDestructAwareness = skill.EnableSelfDestructAwareness
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
	moveAckTimeoutTicks = 30
	fireAckTimeoutTicks = 10
	mineAckTimeoutTicks = 10
	hysteresisTicks     = 2
)

func thinkingDelayTicks(difficulty int) uint64 {
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 10 {
		difficulty = 10
	}
	return _SKILL_TABLE[difficulty].ThinkingDelayTicks
}

// Direction constants mirror engine/constants.go; the SDK's TankInfo.Dir
// carries the engine's int value. Local copy avoids importing the engine
// package into the bot binary.
const (
	dirUp        = 1
	dirDown      = 2
	dirLeft      = 3
	dirRight     = 4
	dirUpLeft    = 5
	dirUpRight   = 6
	dirDownLeft  = 7
	dirDownRight = 8
)

func dirToward(myX, myY, exX, exY int) int {
	dx := exX - myX
	dy := exY - myY
	if dx == 0 && dy == 0 {
		return 0
	}
	// Exact 45-degree diagonal: prefer diagonal facing so canFireWithLOS can
	// detect alignment and the engine can spawn a diagonal projectile (engine
	// supports DirUpLeft..DirDownRight with DirectionVectors {+/-1, +/-1}).
	if dx != 0 && dy != 0 && abs(dx) == abs(dy) {
		if dx > 0 && dy > 0 {
			return dirDownRight
		}
		if dx > 0 && dy < 0 {
			return dirUpRight
		}
		if dx < 0 && dy > 0 {
			return dirDownLeft
		}
		return dirUpLeft
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
	case dirUpLeft:
		return "up_left"
	case dirUpRight:
		return "up_right"
	case dirDownLeft:
		return "down_left"
	case dirDownRight:
		return "down_right"
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

	// Watchdog: if no action is pending but the gate is still locked far past
	// any legitimate thinking delay, force-clear it. This catches future bugs
	// where a code path sets nextActionTick but never resolves (e.g. ack-path
	// regression, tick-counter wraparound, or a held-key state we haven't
	// thought of yet) so a single defect cannot livelock the bot indefinitely.
	if tick > bs.nextActionTick+bs.skillConfig.ThinkingDelayTicks*3 {
		bs.nextActionTick = 0
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
			// Rotation-only ack (barrel-curl first input): the engine consumed our
			// input as a turn, not a translation, so we accomplished only half of
			// the intended action. Clearing nextActionTick lets startNewAction()
			// either fire (if now aligned with enemy) or commit the second input
			// (the actual move) on the very next tick instead of idling out the
			// remainder of the thinking-delay gate. Without this reset the bot
			// burns ~28 of 30 gate ticks doing nothing after a rotation acks in
			// 1-2 ticks — root cause of the 0.9.7 bot-vs-bot livelock.
			if rotated && !moved {
				bs.nextActionTick = tick
			}
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

	// WAVE 8: Protectile dodge — highest priority survival action.
	// Runs before any other decision and ignores nextActionTick gate.
	if bs.skillConfig.EnableProtectile && len(bs.lastShots) > 0 && enemy != nil && enemy.Active {
		if hasThreat, dodgeDir := bs.threateningShot(my, bs.lastShots); hasThreat && dodgeDir != "" {
			return bs.commitMove(tick, my, dodgeDir)
		}
	}

	if tick < bs.nextActionTick {
		return nil
	}

	// WAVE 5: Wall-shooting to open line-of-sight to enemy.
	// Only check straight lines; if a cellWall blocks the path, rotate toward it
	// (or fire if already aimed) to destroy it.
	if bs.skillConfig.EnableWallShootingHunt && enemy != nil && enemy.Active {
		if found, wallX, wallY := bs.wallBetween(my, enemy); found {
			desiredDir := dirToward(my.X, my.Y, wallX, wallY)
			if desiredDir != 0 && my.Dir != desiredDir {
				key := dirToKey(desiredDir)
				if key != "" {
					return bs.commitMove(tick, my, key)
				}
			}
			// Already aimed at wall → fire to destroy it.
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
			bs.nextActionTick = tick + bs.skillConfig.ThinkingDelayTicks
			bs.FireCount++
			return []InputAction{{Key: "fire", Action: "down"}}
		}
	}

	// NORMAL FIRE at enemy.
	if enemy != nil && enemy.Active && bs.canFireWithLOS(my, enemy) {
		desiredFireDir := dirToward(my.X, my.Y, enemy.X, enemy.Y)
		if desiredFireDir != 0 && my.Dir != desiredFireDir {
			key := dirToKey(desiredFireDir)
			if key != "" {
				return bs.commitMove(tick, my, key)
			}
		}

		// WAVE 3: Self-destruct awareness — refuse last shot unless guaranteed kill.
		if bs.skillConfig.EnableSelfDestructAwareness && my.ShotsLeft == 1 {
			dx := enemy.X - my.X
			dy := enemy.Y - my.Y
			isAdjacent := abs(dx) <= 1 && abs(dy) <= 1
			isAimed := my.Dir == desiredFireDir
			if !isAdjacent || !isAimed {
				// Skip fire; fall through to move/mine.
			}
		} else {
			// Normal fire (or self-destruct disabled).
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
			bs.nextActionTick = tick + bs.skillConfig.ThinkingDelayTicks
			bs.FireCount++
			return []InputAction{{Key: "fire", Action: "down"}}
		}
	}

	// MINE.
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
			bs.nextActionTick = tick + bs.skillConfig.ThinkingDelayTicks
			bs.MineCount++
			return []InputAction{{Key: "mine", Action: "down"}}
		}
	}

	// WAVE 4: Stuck detection → shoot adjacent wall to get unstuck.
	if bs.skillConfig.EnableWallShootingUnstick && enemy != nil && enemy.Active {
		if my.X == bs.stuckX && my.Y == bs.stuckY {
			bs.stuckCounter++
		} else {
			bs.stuckX, bs.stuckY = my.X, my.Y
			bs.stuckCounter = 0
		}
		if bs.stuckCounter >= bs.skillConfig.StuckThreshold && bs.skillConfig.StuckThreshold > 0 {
			bs.stuckCounter = 0
			if dir := bs.findAdjacentWallDir(my); dir != "" {
				// Map string direction to int direction constant.
				var wallDirInt int
				switch dir {
				case "up": wallDirInt = dirUp
				case "down": wallDirInt = dirDown
				case "left": wallDirInt = dirLeft
				case "right": wallDirInt = dirRight
				}
				if my.Dir == wallDirInt {
					// Already aimed at wall → fire.
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
					bs.nextActionTick = tick + bs.skillConfig.ThinkingDelayTicks
					bs.FireCount++
					return []InputAction{{Key: "fire", Action: "down"}}
				}
				// Not aimed → rotate toward wall.
				return bs.commitMove(tick, my, dir)
			}
		}
	}

	// MOVE toward enemy (or fallback).
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

	// WAVE 6: Evasive movement — dodge perpendicular when enemy aligned+in-range.
	if bs.skillConfig.EnableEvasion && enemy != nil && enemy.Active {
		dist := abs(my.X-enemy.X) + abs(my.Y-enemy.Y)
		if dist <= bs.skillConfig.EngageRange && bs.isAlignedWith(my, enemy) {
			// Pick perpendicular direction that avoids move into wall.
			dx := my.X - enemy.X
			dy := my.Y - enemy.Y
			var perpA, perpB string
			if dx == 0 {
				perpA, perpB = "left", "right"
			} else if dy == 0 {
				perpA, perpB = "up", "down"
			} else {
				if (dx > 0 && dy > 0) || (dx < 0 && dy < 0) {
					perpA, perpB = "up_right", "down_left"
				} else {
					perpA, perpB = "up_left", "down_right"
				}
			}
			if !bs.isWall(my.X, my.Y, perpA) {
				desired = perpA
			} else if !bs.isWall(my.X, my.Y, perpB) {
				desired = perpB
			}
		}
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
	bs.nextActionTick = tick + bs.skillConfig.ThinkingDelayTicks
	bs.MoveCount++
	if !bs.firstActionDone {
		bs.firstActionDone = true
		if bs.MyID == 2 {
			bs.nextActionTick++
		}
	}
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

// canFireWithLOS returns true iff shots remain, we're strictly same-row/column
// or 45° diagonally aligned with the enemy, and every cell strictly between us
// is non-blocking. Fire-through cells: empty floor, our own barrel, enemy
// barrel. Blocking cells: walls, mines, wreckage, barrels of other tanks.
//
// Engine-correctness contract: the spawn-cell check must match
// engine.ShotSpawnPosition exactly — it rejects ONLY CellWall (out-of-bounds
// is impossible here because barrelPos derives from a tank inside the grid).
// Earlier 0.9.7.3 used bs.isOpen() which excluded own barrel, but the bot's
// own barrel cell is ALWAYS encoded as CellBarrel1/2 in the keyframe — so that
// guard rejected every legitimate fire and broke the bot completely (observed
// in 0.9.8 playtest: P2 traversed entire row aligned with P1, never fired).
func (bs *BotState) canFireWithLOS(my, enemy *botsdk.TankInfo) bool {
	if my.ShotsLeft <= 0 {
		return false
	}
	// WAVE 2: Ammo conservation — refuse fire when low on ammo and enemy not adjacent
	if bs.skillConfig.MinShotsForFire > 0 && my.ShotsLeft <= bs.skillConfig.MinShotsForFire {
		if abs(my.X-enemy.X) > 1 || abs(my.Y-enemy.Y) > 1 {
			return false
		}
	}
	// WAVE 6/7: Detection range — refuse fire if enemy beyond effective range
	maxRange := bs.maxFireRange()
	manhattanDist := abs(my.X-enemy.X) + abs(my.Y-enemy.Y)
	if manhattanDist > maxRange {
		return false
	}
	myBarrelX, myBarrelY := barrelPos(my)
	exBarrelX, exBarrelY := barrelPos(enemy)
	if !bs.canSpawnShot(myBarrelX, myBarrelY) {
		return false
	}
	openForLOS := func(x, y int) bool {
		if x == myBarrelX && y == myBarrelY {
			return true
		}
		if x == exBarrelX && y == exBarrelY {
			return true
		}
		return bs.isOpen(x, y)
	}
	if my.Y == enemy.Y {
		y := my.Y
		x1, x2 := my.X, enemy.X
		if x1 > x2 {
			x1, x2 = x2, x1
		}
		for x := x1 + 1; x < x2; x++ {
			if !openForLOS(x, y) {
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
			if !openForLOS(x, y) {
				return false
			}
		}
		return true
	}
	// 45-degree diagonal alignment: walk one cell diagonally per step. Engine
	// Shot.Step uses DirectionVectors {+/-1, +/-1} so a diagonal projectile
	// traverses exactly the same cells we check here. Without this, two bots
	// pacing at offsets like (12,10) vs (15,9) never get a fire opportunity
	// even though the engine fully supports the shot.
	dx := enemy.X - my.X
	dy := enemy.Y - my.Y
	if dx != 0 && dy != 0 && abs(dx) == abs(dy) {
		stepX := 1
		if dx < 0 {
			stepX = -1
		}
		stepY := 1
		if dy < 0 {
			stepY = -1
		}
		x, y := my.X+stepX, my.Y+stepY
		for x != enemy.X {
			if !openForLOS(x, y) {
				return false
			}
			x += stepX
			y += stepY
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

func (bs *BotState) canSpawnShot(x, y int) bool {
	if y < 0 || y >= len(bs.grid) {
		return false
	}
	if x < 0 || x >= len(bs.grid[y]) {
		return false
	}
	return bs.grid[y][x] != cellWall
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
	// Both cardinal options are walled. Engine supports 8 movement directions
	// and spawns reserve a 3x3 grid, so at least one diagonal toward the enemy
	// should be traversable. Pick the diagonal whose dx/dy signs match the
	// vector to the enemy; this prevents the spawn-pocket idle livelock where
	// a 4-cardinal-only bot has all neighbors walled in tight terrain.
	if dx != 0 && dy != 0 {
		var diag string
		if dx > 0 && dy > 0 {
			diag = "down_right"
		} else if dx > 0 && dy < 0 {
			diag = "up_right"
		} else if dx < 0 && dy > 0 {
			diag = "down_left"
		} else {
			diag = "up_left"
		}
		if !bs.isWall(my.X, my.Y, diag) {
			return diag
		}
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
	directions := []string{"up", "down", "left", "right", "up_left", "up_right", "down_left", "down_right"}
	start := rand.Intn(len(directions))
	for i := range directions {
		d := directions[(start+i)%len(directions)]
		if !bs.isWall(x, y, d) {
			return d
		}
	}
	return ""
}

func (bs *BotState) findAdjacentWallDir(my *botsdk.TankInfo) string {
	dirs := []string{"up", "down", "left", "right"}
	for _, d := range dirs {
		nx, ny := my.X, my.Y
		switch d {
		case "up":
			ny--
		case "down":
			ny++
		case "left":
			nx--
		case "right":
			nx++
		}
		if ny >= 0 && ny < len(bs.grid) && nx >= 0 && nx < len(bs.grid[ny]) {
			if bs.grid[ny][nx] == cellWall {
				return d
			}
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

// barrelPos returns the cell occupied by a tank's barrel, mirroring engine
// DirectionVectors (engine/game.go). Returns sentinel (-1,-1) for an unknown
// direction; sentinel never matches a real grid cell (cells start at 0,0) so
// it safely no-ops in canFireWithLOS comparisons.
func barrelPos(t *botsdk.TankInfo) (int, int) {
	switch t.Dir {
	case dirUp:
		return t.X, t.Y - 1
	case dirDown:
		return t.X, t.Y + 1
	case dirLeft:
		return t.X - 1, t.Y
	case dirRight:
		return t.X + 1, t.Y
	case dirUpLeft:
		return t.X - 1, t.Y - 1
	case dirUpRight:
		return t.X + 1, t.Y - 1
	case dirDownLeft:
		return t.X - 1, t.Y + 1
	case dirDownRight:
		return t.X + 1, t.Y + 1
	default:
		return -1, -1
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (bs *BotState) maxFireRange() int {
	const baseRange = 15
	return int(float64(baseRange) * bs.skillConfig.DetectionRangeScalar)
}

func (bs *BotState) threateningShot(my *botsdk.TankInfo, shots []botsdk.ShotInfo) (bool, string) {
	if !bs.skillConfig.EnableProtectile || bs.skillConfig.ProtectileScalar <= 0 {
		return false, ""
	}
	scalar := bs.skillConfig.ProtectileScalar
	for _, s := range shots {
		if s.OwnerID == bs.MyID || !s.Active {
			continue
		}
		dist := abs(my.X-s.X) + abs(my.Y-s.Y)
		if dist > scalar {
			continue
		}
		shotV := directionVector(s.Dir)
		botFromShotX := my.X - s.X
		botFromShotY := my.Y - s.Y
		if shotV[0]*botFromShotX+shotV[1]*botFromShotY <= 0 {
			continue
		}
		myV := directionVector(my.Dir)
		shotFromBotX := s.X - my.X
		shotFromBotY := s.Y - my.Y
		if myV[0]*shotFromBotX+myV[1]*shotFromBotY <= 0 {
			continue
		}
		dodgeDir := bs.perpendicularDodgeDir(my, s.Dir)
		if dodgeDir != "" {
			return true, dodgeDir
		}
	}
	return false, ""
}

func directionVector(d int) [2]int {
	switch d {
	case dirUp:
		return [2]int{0, -1}
	case dirDown:
		return [2]int{0, 1}
	case dirLeft:
		return [2]int{-1, 0}
	case dirRight:
		return [2]int{1, 0}
	case dirUpLeft:
		return [2]int{-1, -1}
	case dirUpRight:
		return [2]int{1, -1}
	case dirDownLeft:
		return [2]int{-1, 1}
	case dirDownRight:
		return [2]int{1, 1}
	}
	return [2]int{0, 0}
}

func (bs *BotState) perpendicularDodgeDir(my *botsdk.TankInfo, shotDir int) string {
	var p1, p2 string
	switch shotDir {
	case dirUp, dirDown:
		p1, p2 = "left", "right"
	case dirLeft, dirRight:
		p1, p2 = "up", "down"
	case dirUpLeft, dirDownRight:
		p1, p2 = "up_right", "down_left"
	case dirUpRight, dirDownLeft:
		p1, p2 = "up_left", "down_right"
	}
	if !bs.isWall(my.X, my.Y, p1) {
		return p1
	}
	if !bs.isWall(my.X, my.Y, p2) {
		return p2
	}
	return ""
}

// wallBetween scans the straight line (row, col, or 45° diagonal) between
// my and enemy. Returns (found, wallX, wallY) if a cellWall is found.
func (bs *BotState) wallBetween(my, enemy *botsdk.TankInfo) (bool, int, int) {
	dx := enemy.X - my.X
	dy := enemy.Y - my.Y
	sameRow := my.Y == enemy.Y
	sameCol := my.X == enemy.X
	isDiag := dx != 0 && dy != 0 && abs(dx) == abs(dy)
	if !sameRow && !sameCol && !isDiag {
		return false, 0, 0
	}

	stepX := 0; if dx > 0 { stepX = 1 }; if dx < 0 { stepX = -1 }
	stepY := 0; if dy > 0 { stepY = 1 }; if dy < 0 { stepY = -1 }

	x, y := my.X, my.Y
	for {
		x += stepX
		y += stepY
		if x == enemy.X && y == enemy.Y {
			break
		}
		if y >= 0 && y < len(bs.grid) && x >= 0 && x < len(bs.grid[y]) {
			if bs.grid[y][x] == cellWall {
				return true, x, y
			}
		}
	}
	return false, 0, 0
}

// isAlignedWith returns true if my and enemy share a row, column, or 45° diagonal.
func (bs *BotState) isAlignedWith(my, enemy *botsdk.TankInfo) bool {
	if my.X == enemy.X || my.Y == enemy.Y {
		return true
	}
	if abs(my.X-enemy.X) == abs(my.Y-enemy.Y) {
		return true
	}
	return false
}
