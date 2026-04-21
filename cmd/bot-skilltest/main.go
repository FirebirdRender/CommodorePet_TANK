// bot-skilltest validates that higher-skill bots beat lower-skill bots.
//
// For each room difficulty (1-10), runs N matches with random bot skills (0-9)
// and records win rates. Outputs CSV and JSON for analysis.
//
// Usage:
//
//	./bin/bot-skilltest -matches 100 -concurrency 4
//	./bin/bot-skilltest -matches 500 -concurrency 8 -csv out.csv
//
// Requires: make build-all
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	matchesFlag     = flag.Int("matches", 100, "Matches per difficulty (1-10)")
	concurrencyFlag = flag.Int("concurrency", 4, "Max simultaneous matches")
	serverBin       = flag.String("server-bin", "bin/tank-server", "Path to tank-server")
	botBin          = flag.String("bot-bin", "bin/tank-bot", "Path to bot-go")
	csvFile         = flag.String("csv", "skilltest.csv", "CSV output path")
	jsonFile        = flag.String("json", "skilltest.json", "JSON output path")
	MatchTimeout    = flag.Duration("match-timeout", 300*time.Second, "Per-match wall-clock timeout (draw if exceeded)")
)

type matchResult struct {
	Difficulty int     `json:"difficulty"`
	P1Skill    int     `json:"p1_skill"`
	P2Skill    int     `json:"p2_skill"`
	Winner     int     `json:"winner"`
	Ticks      uint64  `json:"ticks"`
	WallSec    float64 `json:"wall_sec"`
	Status     string  `json:"status"`
	FailReason string  `json:"fail_reason,omitempty"`
}

type difficultySummary struct {
	Difficulty    int     `json:"difficulty"`
	Matches       int     `json:"matches"`
	Passed        int     `json:"passed"`
	Failed        int     `json:"failed"`
	Draws         int     `json:"draws"`
	HigherWins    int     `json:"higher_skill_wins"`
	LowerWins     int     `json:"lower_skill_wins"`
	SameSkillWins int     `json:"same_skill_wins"`
	AvgWallSec    float64 `json:"avg_wall_sec"`
}

type report struct {
	TotalMatches      int                 `json:"total_matches"`
	Passed            int                 `json:"passed"`
	Failed            int                 `json:"failed"`
	WallTimeSec       float64             `json:"wall_time_sec"`
	DifficultyResults []difficultySummary `json:"difficulty_results"`
	RawMatches        []matchResult       `json:"raw_matches"`
}

func main() {
	flag.Parse()

	absServer, absBot := resolveBins()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	tmpDir, _ := os.MkdirTemp("", "skilltest-*")
	defer os.RemoveAll(tmpDir)

	port, srvCmd := startServer(absServer)
	defer srvCmd.Process.Signal(syscall.SIGTERM)

	httpBase := fmt.Sprintf("http://127.0.0.1:%s", port)
	wsBase := fmt.Sprintf("ws://127.0.0.1:%s/ws", port)

	sem := make(chan struct{}, *concurrencyFlag)
	var wg sync.WaitGroup

	startTime := time.Now()
	var totalPassed, totalFailed int64

	allResults := make([][]matchResult, 10)
	var mu sync.Mutex

	for diff := 1; diff <= 10; diff++ {
		fmt.Printf("[skilltest] difficulty %d starting %d matches\n", diff, *matchesFlag)
		for m := 0; m < *matchesFlag; m++ {
			sem <- struct{}{}
			wg.Add(1)
			go func(d, idx int) {
				defer wg.Done()
				defer func() { <-sem }()
				p1Skill := rng.Intn(10)
				p2Skill := rng.Intn(10)
				res := runOneMatch(d, idx, p1Skill, p2Skill, httpBase, wsBase, absBot, tmpDir)
				mu.Lock()
				allResults[d-1] = append(allResults[d-1], res)
				mu.Unlock()
				if res.Status == "PASS" {
					atomic.AddInt64(&totalPassed, 1)
					fmt.Printf("[diff=%d match=%d] PASS skill %d vs %d → P%d wins (%d ticks, %.1fs)\n",
						d, idx, p1Skill, p2Skill, res.Winner, res.Ticks, res.WallSec)
				} else if res.Status == "DRAW" {
					fmt.Printf("[diff=%d match=%d] DRAW skill %d vs %d: %s (%.1fs)\n",
						d, idx, p1Skill, p2Skill, res.FailReason, res.WallSec)
				} else {
					atomic.AddInt64(&totalFailed, 1)
					fmt.Printf("[diff=%d match=%d] FAIL skill %d vs %d: %s (%.1fs)\n",
						d, idx, p1Skill, p2Skill, res.FailReason, res.WallSec)
				}
			}(diff, m)
		}
	}

	wg.Wait()
	wallTime := time.Since(startTime).Seconds()

	r := buildReport(allResults, int(totalPassed), int(totalFailed), wallTime)
	writeCSV(r, *csvFile)
	writeJSON(r, *jsonFile)

	fmt.Printf("\n[skilltest] ===== SUMMARY =====\n")
	fmt.Printf("[skilltest] total:   %d matches (passed=%d failed=%d)\n", r.TotalMatches, r.Passed, r.Failed)
	fmt.Printf("[skilltest] wall:    %.1fs\n", wallTime)
	fmt.Printf("[skilltest] csv:     %s\n", *csvFile)
	fmt.Printf("[skilltest] json:    %s\n", *jsonFile)
	fmt.Printf("\nPer-difficulty higher-skill win rate:\n")
	for _, ds := range r.DifficultyResults {
		played := ds.HigherWins + ds.LowerWins + ds.SameSkillWins
		var rate float64
		if played > 0 {
			rate = float64(ds.HigherWins) / float64(played) * 100
		}
		fmt.Printf("  d=%d: %.1f%% (%d higher / %d lower / %d same / %d played)\n",
			ds.Difficulty, rate, ds.HigherWins, ds.LowerWins, ds.SameSkillWins, played)
	}
}

// ---------- helpers ----------

func resolveBins() (server, bot string) {
	var err error
	server, err = filepath.Abs(*serverBin)
	if err != nil {
		fatalf("resolve server: %v", err)
	}
	bot, err = filepath.Abs(*botBin)
	if err != nil {
		fatalf("resolve bot: %v", err)
	}
	if _, err := os.Stat(server); err != nil {
		fatalf("server binary not found: %s (run: make build-all)", server)
	}
	if _, err := os.Stat(bot); err != nil {
		fatalf("bot binary not found: %s (run: make build-all)", bot)
	}
	return
}

func startServer(bin string) (string, *exec.Cmd) {
	cmd := exec.Command(bin, "-addr", ":0", "-rate-burst", "0")
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		fatalf("start server: %v", err)
	}
	go io.Copy(io.Discard, stdout)

	port := make(chan string, 1)
	go func() {
		buf := make([]byte, 1024)
		var b []byte
		portFound := false
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				b = append(b, buf[:n]...)
				if !portFound {
					if p, ok := parsePort(string(b)); ok {
						port <- p
						portFound = true
					}
				}
			}
			if err != nil {
				break
			}
		}
	}()

	select {
	case p := <-port:
		return p, cmd
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		fatalf("timeout waiting for server port")
	}
	return "", nil
}

func parsePort(s string) (string, bool) {
	for i := 0; i <= len(s)-14; i++ {
		if s[i:i+14] == "listening on :" {
			start := i + 14
			end := start
			for end < len(s) && s[end] >= '0' && s[end] <= '9' {
				end++
			}
			if end > start {
				return s[start:end], true
			}
		}
	}
	return "", false
}

type matchSummary struct {
	Winner    int    `json:"winner"`
	FinalWins [2]int `json:"final_wins"`
	Ticks     uint64 `json:"ticks"`
	FireCount uint64 `json:"fire_count"`
	MoveCount uint64 `json:"move_count"`
	MineCount uint64 `json:"mine_count"`
}

func runOneMatch(difficulty, idx, p1Skill, p2Skill int, httpBase, wsBase, botBin, tmpDir string) matchResult {
	res := matchResult{Difficulty: difficulty, P1Skill: p1Skill, P2Skill: p2Skill}
	start := time.Now()
	defer func() { res.WallSec = time.Since(start).Seconds() }()

	// Create room
	body := fmt.Sprintf(`{"difficulty":%d,"player_name":"T1","vs_ai":false,"auto_fill_bot":false}`, difficulty)
	resp, err := postJSON(httpBase+"/api/room", body)
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "create_room: " + err.Error()
		return res
	}
	var cr struct {
		RoomCode string `json:"room_code"`
		PlayerID int    `json:"player_id"`
		Token    string `json:"token"`
	}
	if err := decodeJSON(resp, &cr); err != nil {
		res.Status = "FAIL"
		res.FailReason = "decode_create: " + err.Error()
		return res
	}

	// P2 joins
	jrBody := fmt.Sprintf(`{"player_name":"T2"}`)
	jrResp, err := postJSON(httpBase+"/api/room/"+cr.RoomCode+"/join", jrBody)
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "join_room: " + err.Error()
		return res
	}
	var jr struct {
		PlayerID int    `json:"player_id"`
		Token    string `json:"token"`
	}
	if err := decodeJSON(jrResp, &jr); err != nil {
		res.Status = "FAIL"
		res.FailReason = "decode_join: " + err.Error()
		return res
	}

	p1File := filepath.Join(tmpDir, fmt.Sprintf("d%d-m%04d-p1.json", difficulty, idx))
	p2File := filepath.Join(tmpDir, fmt.Sprintf("d%d-m%04d-p2.json", difficulty, idx))

	ctx, cancel := context.WithTimeout(context.Background(), *MatchTimeout)
	defer cancel()

	var botWg sync.WaitGroup
	var p1Err, p2Err error
	botWg.Add(1)
	go func() {
		defer botWg.Done()
		p1Err = runBot(ctx, botBin, wsBase, cr.RoomCode, 1, cr.Token, p1Skill, p1File)
	}()
	botWg.Add(1)
	go func() {
		defer botWg.Done()
		p2Err = runBot(ctx, botBin, wsBase, cr.RoomCode, 2, jr.Token, p2Skill, p2File)
	}()
	botWg.Wait()

	if p1Err != nil || p2Err != nil {
		res.Winner = 0
		res.Status = "DRAW"
		res.FailReason = fmt.Sprintf("match timeout: p1=%v p2=%v", p1Err, p2Err)
		return res
	}

	s1, err := readSummary(p1File)
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "read p1 summary: " + err.Error()
		return res
	}
	s2, err := readSummary(p2File)
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "read p2 summary: " + err.Error()
		return res
	}

	if s1.Winner != s2.Winner {
		res.Status = "FAIL"
		res.FailReason = fmt.Sprintf("winner mismatch: p1=%d p2=%d", s1.Winner, s2.Winner)
		return res
	}

	res.Winner = s1.Winner
	res.Ticks = s1.Ticks
	res.Status = "PASS"
	return res
}

func postJSON(url, body string) ([]byte, error) {
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	return b, nil
}

func decodeJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func runBot(ctx context.Context, botBin, wsBase, room string, playerID int, token string, skill int, summaryPath string) error {
	lfile := summaryPath + ".log"
	lf, err := os.Create(lfile)
	if err != nil {
		lf = os.Stderr
	}
	defer lf.Close()
	cmd := exec.CommandContext(ctx, botBin,
		"-server", wsBase, "-room", room,
		"-player-id", strconv.Itoa(playerID),
		"-token", token,
		"-name", fmt.Sprintf("Skill%d", skill),
		"-skill", strconv.Itoa(skill),
		"-exit-after-gameover",
		"-summary-file", summaryPath,
	)
	cmd.Stdout = lf
	cmd.Stderr = lf
	return cmd.Run()
}

func readSummary(path string) (*matchSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s matchSummary
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func buildReport(raw [][]matchResult, passed, failed int, wall float64) report {
	r := report{
		TotalMatches: 10 * *matchesFlag,
		Passed:       passed,
		Failed:       failed,
		WallTimeSec:  wall,
	}
	for d := 1; d <= 10; d++ {
		matches := raw[d-1]
		ds := difficultySummary{Difficulty: d, Matches: len(matches)}
		var wallSec float64
		for _, m := range matches {
			r.RawMatches = append(r.RawMatches, m)
			if m.Status == "DRAW" {
				ds.Draws++
				continue
			}
			if m.Status != "PASS" {
				ds.Failed++
				continue
			}
			ds.Passed++
			wallSec += m.WallSec
			if m.P1Skill == m.P2Skill {
				ds.SameSkillWins++
			} else if (m.Winner == 1 && m.P1Skill > m.P2Skill) || (m.Winner == 2 && m.P2Skill > m.P1Skill) {
				ds.HigherWins++
			} else {
				ds.LowerWins++
			}
		}
		if ds.Passed > 0 {
			ds.AvgWallSec = wallSec / float64(ds.Passed)
		}
		r.DifficultyResults = append(r.DifficultyResults, ds)
	}
	return r
}

func writeCSV(r report, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[skilltest] WARN: create csv: %v\n", err)
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"difficulty", "matches", "passed", "failed", "draws", "higher_skill_wins", "lower_skill_wins", "same_skill_wins", "avg_wall_sec"})
	for _, ds := range r.DifficultyResults {
		w.Write([]string{
			strconv.Itoa(ds.Difficulty),
			strconv.Itoa(ds.Matches),
			strconv.Itoa(ds.Passed),
			strconv.Itoa(ds.Failed),
			strconv.Itoa(ds.Draws),
			strconv.Itoa(ds.HigherWins),
			strconv.Itoa(ds.LowerWins),
			strconv.Itoa(ds.SameSkillWins),
			fmt.Sprintf("%.1f", ds.AvgWallSec),
		})
	}
	w.Flush()
}

func writeJSON(r report, path string) {
	data, _ := json.MarshalIndent(r, "", "  ")
	os.WriteFile(path, data, 0644)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[skilltest] FATAL: "+format+"\n", args...)
	os.Exit(1)
}
