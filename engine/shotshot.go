package engine

const ChainExplosionDuration = 0.8

// ShotsCrossedHeadOn returns true if two active cardinal shots swapped positions
// (discrete head-on pass-through). Only cardinal pairs: UP↔DOWN, LEFT↔RIGHT.
// Diagonal shots NEVER trigger head-on detection.
func ShotsCrossedHeadOn(a, b *Shot) bool {
	if a == nil || b == nil {
		return false
	}
	if !a.Active || !b.Active {
		return false
	}

	saX, saY := a.StepStartX, a.StepStartY
	sbX, sbY := b.StepStartX, b.StepStartY

	// Horizontal: LEFT/RIGHT on same row.
	if saY == sbY && a.Y == b.Y {
		aHoriz := a.Dir == DirLeft || a.Dir == DirRight
		bHoriz := b.Dir == DirLeft || b.Dir == DirRight
		if !aHoriz || !bHoriz {
			return false
		}
		if saX == sbX {
			return false
		}

		left, right := a, b
		if saX > sbX {
			left, right = b, a
		}

		if left.StepStartX >= right.StepStartX {
			return false
		}
		if left.Dir != DirRight || right.Dir != DirLeft {
			return false
		}
		return left.X > right.X
	}

	// Vertical: UP/DOWN on same column.
	if saX == sbX && a.X == b.X {
		aVert := a.Dir == DirUp || a.Dir == DirDown
		bVert := b.Dir == DirUp || b.Dir == DirDown
		if !aVert || !bVert {
			return false
		}
		if saY == sbY {
			return false
		}

		upper, lower := a, b
		if saY > sbY {
			upper, lower = b, a
		}

		if upper.StepStartY >= lower.StepStartY {
			return false
		}
		if upper.Dir != DirDown || lower.Dir != DirUp {
			return false
		}
		return upper.Y > lower.Y
	}

	return false
}

// ShotShotExplosionKey computes the explosion position for a shot-shot collision.
// If crossed (head-on swap): midpoint of post-step positions using integer division.
// If same-cell: returns shot1's current position.
func ShotShotExplosionKey(s1, s2 *Shot, crossed bool) [2]int {
	if !crossed {
		return [2]int{s1.X, s1.Y}
	}
	if s1.Y == s2.Y {
		return [2]int{(s1.X + s2.X) / 2, s1.Y}
	}
	return [2]int{s1.X, (s1.Y + s2.Y) / 2}
}

// DetectShotShotCollisions finds all pairs of active shots that collided.
// Checks: (1) same-cell (different owners, both active), (2) head-on swap.
func DetectShotShotCollisions(gc *GameController) [][2]int {
	if gc == nil || len(gc.Shots) < 2 {
		return nil
	}

	pairs := make([][2]int, 0)
	for i := 0; i < len(gc.Shots)-1; i++ {
		s1 := gc.Shots[i]
		if s1 == nil || !s1.Active {
			continue
		}

		for j := i + 1; j < len(gc.Shots); j++ {
			s2 := gc.Shots[j]
			if s2 == nil || !s2.Active {
				continue
			}

			sameCell := s1.OwnerID != s2.OwnerID && s1.X == s2.X && s1.Y == s2.Y
			if sameCell || ShotsCrossedHeadOn(s1, s2) {
				pairs = append(pairs, [2]int{i, j})
			}
		}
	}

	return pairs
}

// ResolveShotShotCollision deactivates both shots and creates an explosion.
// Uses chain-style explosion: duration=0.8s, isChainReaction=true.
func ResolveShotShotCollision(gc *GameController, i, j int) {
	if gc == nil || i < 0 || j < 0 || i >= len(gc.Shots) || j >= len(gc.Shots) {
		return
	}

	s1, s2 := gc.Shots[i], gc.Shots[j]
	if s1 == nil || s2 == nil {
		return
	}

	crossed := ShotsCrossedHeadOn(s1, s2)
	key := ShotShotExplosionKey(s1, s2, crossed)

	s1.Active = false
	s2.Active = false

	gc.Explosions = append(gc.Explosions, &Explosion{
		X:               key[0],
		Y:               key[1],
		StartTime:       gc.SimTime,
		Duration:        ChainExplosionDuration,
		IsChainReaction: true,
	})
}
