package engine

func (gc *GameController) tankForCellType(ct CellType) *Tank {
	switch ct {
	case CellTank1:
		return gc.Tanks[0]
	case CellTank2:
		return gc.Tanks[1]
	default:
		return nil
	}
}

// ExplodeMine triggers an explosion at (mx, my) with the given radius.
// radius=1 means 3×3 area, radius=2 means 5×5 area.
// isChain=true means this was triggered by another explosion (visual distinction + longer duration).
func (gc *GameController) ExplodeMine(mx, my int, radius int, isChain bool) {
	duration := ExplosionDuration
	if isChain {
		duration = ChainExplosionDuration
	}

	gc.Explosions = append(gc.Explosions, &Explosion{
		X:               mx,
		Y:               my,
		StartTime:       gc.SimTime,
		Duration:        duration,
		IsChainReaction: isChain,
	})

	// Critical ordering: clear mine before scanning radius to avoid self-recursion.
	gc.Board.SetCellType(mx, my, CellEmpty)

	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			nx, ny := mx+dx, my+dy
			if !gc.Board.InBounds(nx, ny) {
				continue
			}

			ct := gc.Board.GetCell(nx, ny)
			switch ct {
			case CellWall:
				gc.Board.SetCellType(nx, ny, CellEmpty)
			case CellWreckageP1, CellWreckageP2:
				gc.Board.SetCellType(nx, ny, CellEmpty)
			case CellMine:
				for _, mine := range gc.Mines {
					if mine.X == nx && mine.Y == ny && mine.Active {
						// Critical ordering: deactivate before recursive explode.
						mine.Active = false
						gc.ExplodeMine(nx, ny, 2, true)
						break
					}
				}
			case CellTank1, CellTank2:
				tank := gc.tankForCellType(ct)
				if tank != nil {
					gc.TankHit(tank, [2]int{nx, ny})
				}
			case CellBarrel1, CellBarrel2:
				// Explosions pass through barrels.
			case CellShot:
				gc.Board.SetCellType(nx, ny, CellEmpty)
			}
		}
	}
}

// ResolveTankMineCollision checks if a tank is standing on an active mine.
// If yes: TankHit first, then ExplodeMine with default radius=1.
func (gc *GameController) ResolveTankMineCollision(tank *Tank) {
	for _, mine := range gc.Mines {
		if mine.Active && mine.X == tank.X && mine.Y == tank.Y {
			gc.TankHit(tank, [2]int{tank.X, tank.Y})
			mine.Active = false
			gc.ExplodeMine(mine.X, mine.Y, 1, false)
			break
		}
	}
}
