package main

import (
	"math/rand"

	botsdk "github.com/FirebirdRender/CommodorePet_TANK/bot-sdk-go"
)

type InputAction struct {
	Key    string
	Action string
}

// Engine cell type values (mirrors engine/constants.go — 1-indexed enum).
const cellEmpty = 1

type BotState struct {
	MyID       int
	Difficulty int
	GridW      int
	GridH      int

	// Persistent grid — keyframe loads it, deltas patch cells in place.
	grid [][]int

	decideCount int
}

func NewBotState(playerID, difficulty int, gridW, gridH int) *BotState {
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

// Decide returns the inputs to send this tick.
//
// Original PET semantics (no key repeat, single-key input): the server consumes
// one action per tick and treats every keypress as a discrete event. The bot
// must therefore choose ONE logical input per tick — either move one cell, or
// fire one shot, or drop one mine. Movement requires re-sending the direction
// key every tick the bot wants to advance; fire/mine are press+release within
// one tick to produce exactly one action.
//
// Priority: fire when aligned with an active enemy > drop mine when adjacent >
// move toward enemy.
func (bs *BotState) Decide(tanks [2]botsdk.TankInfo) []InputAction {
	bs.decideCount++
	myTank, enemyTank := bs.findTanks(tanks)

	if myTank == nil || !myTank.Active {
		return nil
	}

	if enemyTank != nil && enemyTank.Active && bs.canFireAligned(myTank, enemyTank) {
		return []InputAction{
			{Key: "fire", Action: "down"},
			{Key: "fire", Action: "up"},
		}
	}

	if enemyTank != nil && enemyTank.Active && myTank.MinesLeft > 0 {
		dist := abs(myTank.X-enemyTank.X) + abs(myTank.Y-enemyTank.Y)
		if dist < 3 && rand.Float64() < 0.05 {
			return []InputAction{
				{Key: "mine", Action: "down"},
				{Key: "mine", Action: "up"},
			}
		}
	}

	var desiredMove string
	if enemyTank != nil && enemyTank.Active {
		desiredMove = bs.moveTowardEnemy(myTank, enemyTank)
	} else {
		desiredMove = bs.fallbackMove(myTank.X, myTank.Y)
	}

	if desiredMove != "" {
		return []InputAction{{Key: desiredMove, Action: "down"}}
	}

	return nil
}

func (bs *BotState) findTanks(tanks [2]botsdk.TankInfo) (myTank, enemyTank *botsdk.TankInfo) {
	for i := range tanks {
		t := tanks[i]
		if t.PlayerID == bs.MyID {
			my := t
			myTank = &my
		} else {
			en := t
			enemyTank = &en
		}
	}
	return
}

func (bs *BotState) canFireAligned(my, enemy *botsdk.TankInfo) bool {
	if my.ShotsLeft <= 0 {
		return false
	}
	// Strict same-row or same-column alignment only — diagonals don't count.
	return my.Y == enemy.Y || my.X == enemy.X
}

func (bs *BotState) moveTowardEnemy(my, enemy *botsdk.TankInfo) string {
	dx := enemy.X - my.X
	dy := enemy.Y - my.Y

	var moveKey string
	if abs(dx) > abs(dy) {
		if dx > 0 {
			moveKey = "right"
		} else {
			moveKey = "left"
		}
	} else if dy != 0 {
		if dy > 0 {
			moveKey = "down"
		} else {
			moveKey = "up"
		}
	} else if dx != 0 {
		if dx > 0 {
			moveKey = "right"
		} else {
			moveKey = "left"
		}
	} else {
		// On top of enemy — pick any open direction.
		return bs.fallbackMove(my.X, my.Y)
	}

	if bs.isWall(my.X, my.Y, moveKey) {
		// Try perpendicular.
		var perpKey string
		if moveKey == "up" || moveKey == "down" {
			if dx >= 0 {
				perpKey = "right"
			} else {
				perpKey = "left"
			}
		} else {
			if dy >= 0 {
				perpKey = "down"
			} else {
				perpKey = "up"
			}
		}
		if !bs.isWall(my.X, my.Y, perpKey) {
			return perpKey
		}
		// Both blocked — fallback search.
		return bs.fallbackMove(my.X, my.Y)
	}

	return moveKey
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
	if ny < len(bs.grid) && nx < len(bs.grid[ny]) {
		// Engine cell types are 1-indexed: CellEmpty=1, CellWall=2, etc.
		// Any non-empty cell blocks pathing (walls, barrels, wreckage, mines, other tanks).
		return bs.grid[ny][nx] != cellEmpty
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

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
