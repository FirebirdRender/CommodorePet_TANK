package main

import (
	"math/rand"

	botsdk "github.com/FirebirdRender/CommodorePet_TANK/bot-sdk-go"
)

// BotState represents the AI state for the bot
type BotState struct {
	MyID             int
	Difficulty       int
	GridW            int
	GridH            int
	lastDecisionTick uint64
	currentKey       string
	fireCooldown     int
}

// NewBotState creates a new bot state
func NewBotState(playerID, difficulty int, gridW, gridH int) *BotState {
	return &BotState{
		MyID:             playerID,
		Difficulty:       difficulty,
		GridW:            gridW,
		GridH:            gridH,
		lastDecisionTick: 0,
		currentKey:       "",
		fireCooldown:     0,
	}
}

// Decide returns the next action (key + action) to send.
func (bs *BotState) Decide(tick *botsdk.TickPayload, delta *botsdk.TickDeltaPayload) (key string, action string) {
	payload := tick
	if payload == nil && delta != nil {
		payload = &botsdk.TickPayload{
			Tick:           delta.Tick,
			Tanks:          delta.Tanks,
			Shots:          delta.Shots,
			Mines:          delta.Mines,
			Explosions:     delta.Explosions,
			BarrelWreckage: delta.BarrelWreckage,
		}
		payload.Grid = make([][]int, bs.GridH)
		for i := range payload.Grid {
			payload.Grid[i] = make([]int, bs.GridW)
		}
	}

	myTank, enemyTank := bs.findTanks(payload.Tanks)

	if myTank == nil || !myTank.Active {
		return "", ""
	}

	if enemyTank != nil && enemyTank.Active {
		dist := abs(myTank.X-enemyTank.X) + abs(myTank.Y-enemyTank.Y)
		closeRange := dist < 3

		if bs.canFireAligned(myTank, enemyTank) {
			return "fire", "down"
		}

		key, action := bs.moveTowardEnemy(myTank, enemyTank, payload.Grid)

		if closeRange && rand.Float64() < 0.1 {
			if myTank.MinesLeft > 0 {
				return "mine", "down"
			}
		}

		return key, action
	}

	return bs.fallbackMove(0, 0, payload.Grid)
}

func (bs *BotState) findTanks(tanks [2]botsdk.TankInfo) (myTank, enemyTank *botsdk.TankInfo) {
	for _, t := range tanks {
		if t.PlayerID == bs.MyID {
			myTank = &t
		} else {
			enemyTank = &t
		}
	}
	return
}

func (bs *BotState) canFireAligned(my, enemy *botsdk.TankInfo) bool {
	if my.ShotsLeft <= 0 {
		return false
	}
	rowDiff := my.Y - enemy.Y
	colDiff := my.X - enemy.X
	if abs(rowDiff) <= 1 || abs(colDiff) <= 1 {
		return true
	}
	return false
}

func (bs *BotState) moveTowardEnemy(my, enemy *botsdk.TankInfo, grid [][]int) (key, action string) {
	dx := enemy.X - my.X
	dy := enemy.Y - my.Y

	var moveKey string
	if abs(dx) > abs(dy) {
		if dx > 0 {
			moveKey = "right"
		} else {
			moveKey = "left"
		}
	} else {
		if dy > 0 {
			moveKey = "down"
		} else {
			moveKey = "up"
		}
	}

	if bs.isWall(my.X, my.Y, moveKey, grid) {
		var perpKey string
		if moveKey == "up" || moveKey == "down" {
			if dx > 0 {
				perpKey = "right"
			} else {
				perpKey = "left"
			}
		} else {
			if dy > 0 {
				perpKey = "down"
			} else {
				perpKey = "up"
			}
		}

		if bs.isWall(my.X, my.Y, perpKey, grid) {
			if moveKey == "up" {
				moveKey = "down"
			} else if moveKey == "down" {
				moveKey = "up"
			} else if moveKey == "left" {
				moveKey = "right"
			} else {
				moveKey = "left"
			}
		} else {
			moveKey = perpKey
		}
	}

	return moveKey, "down"
}

func (bs *BotState) isWall(x, y int, direction string, grid [][]int) bool {
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
	}

	if nx < 0 || nx >= bs.GridW || ny < 0 || ny >= bs.GridH {
		return true
	}

	if ny < len(grid) && nx < len(grid[ny]) {
		return grid[ny][nx] > 0
	}

	return true
}

func (bs *BotState) fallbackMove(x, y int, grid [][]int) (key, action string) {
	directions := []string{"up", "down", "left", "right"}
	// Start from random offset to avoid deterministic wall-hugging
	start := rand.Intn(len(directions))
	for i := range directions {
		d := directions[(start+i)%len(directions)]
		if !bs.isWall(x, y, d, grid) {
			return d, "down"
		}
	}

	return "", ""
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
