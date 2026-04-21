package main

// SkillConfig gates bot behaviors per difficulty level.
type SkillConfig struct {
	ThinkingDelayTicks          uint64
	MinShotsForFire             int
	EnableSelfDestructAwareness bool
	EnableWallShootingUnstick   bool
	StuckThreshold              int
	EnableWallShootingHunt      bool
	DetectionRangeScalar        float64
	ProtectileScalar            int
	EnableProtectile            bool
	EnableEvasion               bool
	EnableDiagonalFire          bool
	EngageRange                 int
}

// _SKILL_TABLE is indexed by difficulty (1-10).
var _SKILL_TABLE = [11]SkillConfig{
	{}, // index 0 unused
	{ThinkingDelayTicks: 54, MinShotsForFire: 1, EnableSelfDestructAwareness: false, EnableWallShootingUnstick: false, StuckThreshold: 0, EnableWallShootingHunt: false, DetectionRangeScalar: 0.50, ProtectileScalar: 2, EnableProtectile: true, EnableEvasion: false, EnableDiagonalFire: false, EngageRange: 0},
	{ThinkingDelayTicks: 48, MinShotsForFire: 1, EnableSelfDestructAwareness: false, EnableWallShootingUnstick: false, StuckThreshold: 0, EnableWallShootingHunt: false, DetectionRangeScalar: 0.50, ProtectileScalar: 2, EnableProtectile: true, EnableEvasion: false, EnableDiagonalFire: false, EngageRange: 0},
	{ThinkingDelayTicks: 42, MinShotsForFire: 1, EnableSelfDestructAwareness: false, EnableWallShootingUnstick: false, StuckThreshold: 0, EnableWallShootingHunt: false, DetectionRangeScalar: 0.55, ProtectileScalar: 4, EnableProtectile: true, EnableEvasion: false, EnableDiagonalFire: false, EngageRange: 0},
	{ThinkingDelayTicks: 36, MinShotsForFire: 1, EnableSelfDestructAwareness: false, EnableWallShootingUnstick: true, StuckThreshold: 10, EnableWallShootingHunt: false, DetectionRangeScalar: 0.60, ProtectileScalar: 4, EnableProtectile: true, EnableEvasion: false, EnableDiagonalFire: true, EngageRange: 0},
	{ThinkingDelayTicks: 30, MinShotsForFire: 2, EnableSelfDestructAwareness: true, EnableWallShootingUnstick: true, StuckThreshold: 10, EnableWallShootingHunt: false, DetectionRangeScalar: 0.65, ProtectileScalar: 6, EnableProtectile: true, EnableEvasion: false, EnableDiagonalFire: true, EngageRange: 0},
	{ThinkingDelayTicks: 24, MinShotsForFire: 2, EnableSelfDestructAwareness: true, EnableWallShootingUnstick: true, StuckThreshold: 10, EnableWallShootingHunt: true, DetectionRangeScalar: 0.70, ProtectileScalar: 6, EnableProtectile: true, EnableEvasion: false, EnableDiagonalFire: true, EngageRange: 0},
	{ThinkingDelayTicks: 18, MinShotsForFire: 2, EnableSelfDestructAwareness: true, EnableWallShootingUnstick: true, StuckThreshold: 8, EnableWallShootingHunt: true, DetectionRangeScalar: 0.80, ProtectileScalar: 7, EnableProtectile: true, EnableEvasion: true, EnableDiagonalFire: true, EngageRange: 4},
	{ThinkingDelayTicks: 12, MinShotsForFire: 2, EnableSelfDestructAwareness: true, EnableWallShootingUnstick: true, StuckThreshold: 8, EnableWallShootingHunt: true, DetectionRangeScalar: 0.90, ProtectileScalar: 7, EnableProtectile: true, EnableEvasion: true, EnableDiagonalFire: true, EngageRange: 4},
	{ThinkingDelayTicks: 6, MinShotsForFire: 2, EnableSelfDestructAwareness: true, EnableWallShootingUnstick: true, StuckThreshold: 5, EnableWallShootingHunt: true, DetectionRangeScalar: 0.95, ProtectileScalar: 8, EnableProtectile: true, EnableEvasion: true, EnableDiagonalFire: true, EngageRange: 3},
	{ThinkingDelayTicks: 0, MinShotsForFire: 0, EnableSelfDestructAwareness: true, EnableWallShootingUnstick: true, StuckThreshold: 5, EnableWallShootingHunt: true, DetectionRangeScalar: 1.00, ProtectileScalar: 8, EnableProtectile: true, EnableEvasion: true, EnableDiagonalFire: true, EngageRange: 3},
}
