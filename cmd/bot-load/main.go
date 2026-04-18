// Phase 6 G-3: Load-test harness for bot-vs-bot matches.
//
// Spawns the tank-server and two bot-go subprocesses per match, runs N matches
// with bounded concurrency, and verifies the server has no goroutine leak after
// all matches drain.
//
// Usage:
//
//	./bin/bot-load -matches 100 -concurrency 4
//	./bin/bot-load -matches 10 -concurrency 4   # smoke
//	./bin/bot-load -matches 100 -seed 42        # reproducible
//
// Requires built binaries: bin/tank-server, bin/bot-go.
//
// Match flow:
//  1. POST /api/room (vs_ai:false) - returns room_code, p1 token
//  2. POST /api/room/{code}/join   - returns p2 token
//  3. Spawn 2 bot-go subprocesses with -room/-token/-player-id/-skill/-exit-after-gameover/-summary-file
//  4. Wait for both to exit, read both summary JSONs, validate winner agreement
//  5. Record per-match result; aggregate by skill pair
//
// Stratified scheduler: 10x10=100 skill pairs. -matches=100 runs each pair
// once shuffled. -matches<100 samples without replacement. -matches>100
// cycles the shuffled list.
//
// Leak detection: snapshot /debug/pprof/goroutine?debug=1 count before any
// match and after all matches drain (with runtime.GC + 2s settle on the
// server side via a brief pause). Delta > leakThreshold = FAIL.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// matchSummary mirrors cmd/bot-go/main.go's matchSummary. Kept private
// duplicate (priority 4: prevents cross-package coupling for a stable wire
// format consumed only by this harness).
type matchSummary struct {
	Winner        int    `json:"winner"`
	FinalWins     [2]int `json:"final_wins"`
	MyPlayerID    int    `json:"my_player_id"`
	Difficulty    int    `json:"difficulty"`
	FireCount     uint64 `json:"fire_count"`
	MoveCount     uint64 `json:"move_count"`
	MineCount     uint64 `json:"mine_count"`
	DroppedInputs uint64 `json:"dropped_inputs"`
	Ticks         uint64 `json:"ticks"`
}

type matchResult struct {
	Index       int           `json:"index"`
	P1Skill     int           `json:"p1_skill"`
	P2Skill     int           `json:"p2_skill"`
	Status      string        `json:"status"`
	FailReason  string        `json:"fail_reason,omitempty"`
	Winner      int           `json:"winner,omitempty"`
	Ticks       uint64        `json:"ticks,omitempty"`
	P1Summary   *matchSummary `json:"p1_summary,omitempty"`
	P2Summary   *matchSummary `json:"p2_summary,omitempty"`
	WallTimeSec float64       `json:"wall_time_sec"`
	RoomCode    string        `json:"room_code,omitempty"`
}

type pairAggregate struct {
	P1Skill    int     `json:"p1_skill"`
	P2Skill    int     `json:"p2_skill"`
	Played     int     `json:"played"`
	P1Wins     int     `json:"p1_wins"`
	P2Wins     int     `json:"p2_wins"`
	Failures   int     `json:"failures"`
	AvgTicks   float64 `json:"avg_ticks"`
	AvgWallSec float64 `json:"avg_wall_sec"`
}

type tournamentReport struct {
	Seed               int64           `json:"seed"`
	Matches            int             `json:"matches"`
	Concurrency        int             `json:"concurrency"`
	Passed             int             `json:"passed"`
	Failed             int             `json:"failed"`
	OverallStatus      string          `json:"overall_status"`
	BaselineGoroutines int             `json:"baseline_goroutines"`
	FinalGoroutines    int             `json:"final_goroutines"`
	GoroutineDelta     int             `json:"goroutine_delta"`
	LeakThreshold      int             `json:"leak_threshold"`
	LeakStatus         string          `json:"leak_status"`
	WallTimeSec        float64         `json:"wall_time_sec"`
	PairAggregates     []pairAggregate `json:"pair_aggregates"`
	Matches_Detail     []matchResult   `json:"matches_detail"`
}

type joinResponse struct {
	RoomCode string `json:"room_code"`
	PlayerID int    `json:"player_id"`
	Token    string `json:"token"`
}

type createRoomResponse struct {
	RoomCode string `json:"room_code"`
	PlayerID int    `json:"player_id"`
	Token    string `json:"token"`
}

func main() {
	matches := flag.Int("matches", 100, "Number of matches to run")
	concurrency := flag.Int("concurrency", 4, "Max concurrent matches")
	seed := flag.Int64("seed", 0, "Random seed (0 = time.Now().UnixNano())")
	serverBin := flag.String("server-bin", "bin/tank-server", "Path to tank-server binary")
	botBin := flag.String("bot-bin", "bin/bot-go", "Path to bot-go binary")
	pprofAddr := flag.String("pprof-addr", "127.0.0.1:6160", "pprof address for the server (used for leak detection)")
	matchTimeout := flag.Duration("match-timeout", 120*time.Second, "Per-match wall-clock cap; exceeded = SIGKILL + FAIL")
	leakThreshold := flag.Int("leak-threshold", 5, "Goroutine count delta above baseline that constitutes a leak")
	reportFile := flag.String("report", "bot-load-report.json", "JSON report output path")
	keepArtifacts := flag.Bool("keep-artifacts", false, "Keep per-match summary JSONs and bot logs (default: cleanup tmpdir)")
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(*seed))

	fmt.Printf("[bot-load] seed=%d matches=%d concurrency=%d match-timeout=%v leak-threshold=%d\n",
		*seed, *matches, *concurrency, *matchTimeout, *leakThreshold)

	// Resolve binary paths to absolute (subprocesses inherit cwd).
	absServer, err := filepath.Abs(*serverBin)
	if err != nil {
		fatalf("resolve server-bin: %v", err)
	}
	absBot, err := filepath.Abs(*botBin)
	if err != nil {
		fatalf("resolve bot-bin: %v", err)
	}
	if _, err := os.Stat(absServer); err != nil {
		fatalf("server binary not found at %s: %v (run: make build-all)", absServer, err)
	}
	if _, err := os.Stat(absBot); err != nil {
		fatalf("bot binary not found at %s: %v (run: make build-all)", absBot, err)
	}

	tmpDir, err := os.MkdirTemp("", "bot-load-*")
	if err != nil {
		fatalf("mkdtemp: %v", err)
	}
	if !*keepArtifacts {
		defer os.RemoveAll(tmpDir)
	}
	fmt.Printf("[bot-load] tmpdir=%s (keep=%v)\n", tmpDir, *keepArtifacts)

	srv, port, err := startServer(absServer, *pprofAddr)
	if err != nil {
		fatalf("start server: %v", err)
	}
	defer func() {
		if srv.process != nil {
			_ = srv.process.Signal(syscall.SIGTERM)
			_, _ = srv.process.Wait()
		}
	}()

	httpBase := fmt.Sprintf("http://127.0.0.1:%s", port)
	wsBase := fmt.Sprintf("ws://127.0.0.1:%s/ws", port)
	fmt.Printf("[bot-load] server up: http=%s ws=%s pprof=http://%s\n", httpBase, wsBase, *pprofAddr)

	// Server-death watchdog: if the server process exits before tournament
	// completes, we abort with a clear error (Oracle Q6 refinement).
	serverDied := make(chan struct{})
	go func() {
		srv.waitErr = srv.cmd.Wait()
		close(serverDied)
	}()

	// pprof HTTP server starts asynchronously after the main listener; retry up
	// to 3s to avoid a race where we query before its socket is bound.
	var baseline int
	for attempt := 0; attempt < 30; attempt++ {
		baseline, err = goroutineCount(*pprofAddr)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		fatalf("baseline goroutine count: %v", err)
	}
	fmt.Printf("[bot-load] baseline goroutines: %d\n", baseline)

	schedule := buildSchedule(*matches, rng)

	tournamentStart := time.Now()
	results := make([]matchResult, *matches)
	var passed, failed int64
	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup
	abort := make(chan struct{})

	go func() {
		select {
		case <-serverDied:
			fmt.Fprintf(os.Stderr, "[bot-load] FATAL: server process exited mid-tournament: %v\n", srv.waitErr)
			close(abort)
		case <-time.After(time.Duration(*matches) * (*matchTimeout) / time.Duration(*concurrency) * 2):
			fmt.Fprintf(os.Stderr, "[bot-load] FATAL: global wall-clock cap exceeded\n")
			close(abort)
		}
	}()

	for i := 0; i < *matches; i++ {
		select {
		case <-abort:
			fmt.Fprintf(os.Stderr, "[bot-load] aborting tournament at match %d\n", i)
			break
		case sem <- struct{}{}:
		}
		wg.Add(1)
		idx := i
		pair := schedule[i]
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res := runMatch(idx, pair[0], pair[1], httpBase, wsBase, absBot, tmpDir, *matchTimeout)
			results[idx] = res
			if res.Status == "PASS" {
				atomic.AddInt64(&passed, 1)
				fmt.Printf("[match %d/%d] PASS skill %d vs %d → P%d wins (%d ticks, %.1fs)\n",
					idx+1, *matches, pair[0], pair[1], res.Winner, res.Ticks, res.WallTimeSec)
			} else {
				atomic.AddInt64(&failed, 1)
				fmt.Printf("[match %d/%d] FAIL skill %d vs %d: %s (%.1fs)\n",
					idx+1, *matches, pair[0], pair[1], res.FailReason, res.WallTimeSec)
			}
		}()
	}

	doneAll := make(chan struct{})
	go func() { wg.Wait(); close(doneAll) }()
	select {
	case <-doneAll:
	case <-abort:
		<-doneAll
	}
	tournamentWall := time.Since(tournamentStart).Seconds()

	fmt.Printf("[bot-load] settling 5s for server-side GC and cleanup goroutines to exit...\n")
	time.Sleep(5 * time.Second)
	finalCount, err := goroutineCount(*pprofAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[bot-load] WARN: failed to read final goroutine count: %v\n", err)
		finalCount = -1
	}
	delta := finalCount - baseline
	leakStatus := "PASS"
	if finalCount < 0 || delta > *leakThreshold {
		leakStatus = "FAIL"
	}

	aggregates := aggregateByPair(results)

	overallStatus := "PASS"
	if int(failed) > 0 || leakStatus == "FAIL" {
		overallStatus = "FAIL"
	}
	report := tournamentReport{
		Seed:               *seed,
		Matches:            *matches,
		Concurrency:        *concurrency,
		Passed:             int(passed),
		Failed:             int(failed),
		OverallStatus:      overallStatus,
		BaselineGoroutines: baseline,
		FinalGoroutines:    finalCount,
		GoroutineDelta:     delta,
		LeakThreshold:      *leakThreshold,
		LeakStatus:         leakStatus,
		WallTimeSec:        tournamentWall,
		PairAggregates:     aggregates,
		Matches_Detail:     results,
	}

	if data, err := json.MarshalIndent(report, "", "  "); err != nil {
		fmt.Fprintf(os.Stderr, "[bot-load] WARN: marshal report: %v\n", err)
	} else if err := os.WriteFile(*reportFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[bot-load] WARN: write report: %v\n", err)
	} else {
		fmt.Printf("[bot-load] report written to %s\n", *reportFile)
	}

	fmt.Printf("\n[bot-load] ===== TOURNAMENT SUMMARY =====\n")
	fmt.Printf("[bot-load] matches:    %d (passed=%d failed=%d)\n", *matches, passed, failed)
	fmt.Printf("[bot-load] wall time:  %.1fs\n", tournamentWall)
	fmt.Printf("[bot-load] goroutines: baseline=%d final=%d delta=%d (threshold=%d) [%s]\n",
		baseline, finalCount, delta, *leakThreshold, leakStatus)
	fmt.Printf("[bot-load] OVERALL:    %s\n", overallStatus)

	if overallStatus != "PASS" {
		os.Exit(1)
	}
}

type serverProc struct {
	cmd     *exec.Cmd
	process *os.Process
	waitErr error
}

// startServer spawns tank-server with -addr :0 -pprof-addr <addr> and
// TANK_ENABLE_BOTS=1 (required for bot-vs-bot). Parses the resolved port
// from stdout via the "listening on :PORT" log line.
func startServer(serverBin, pprofAddr string) (*serverProc, string, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, serverBin, "-addr", ":0", "-pprof-addr", pprofAddr)
	cmd.Env = append(os.Environ(), "TANK_ENABLE_BOTS=1")

	// Server uses Go's standard logger -> writes to stderr. Scan stderr for the
	// "listening on :PORT" line; pipe stdout straight through for visibility.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, "", fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start: %w", err)
	}

	go func() { _, _ = io.Copy(os.Stdout, stdout) }()

	portRegex := regexp.MustCompile(`listening on :(\d+)`)
	pprofErrRegex := regexp.MustCompile(`\[pprof\] server error`)
	portCh := make(chan string, 1)
	pprofErrCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		var portFound bool
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(os.Stderr, "[server] "+line)
			if !portFound {
				if m := portRegex.FindStringSubmatch(line); m != nil {
					portCh <- m[1]
					portFound = true
				}
			}
			if pprofErrRegex.MatchString(line) {
				select {
				case pprofErrCh <- line:
				default:
				}
			}
		}
		if !portFound {
			close(portCh)
		}
	}()

	select {
	case errLine := <-pprofErrCh:
		_ = cmd.Process.Kill()
		return nil, "", fmt.Errorf("pprof bind failed (stale server holding port?): %s", errLine)
	case port, ok := <-portCh:
		if !ok {
			_ = cmd.Process.Kill()
			return nil, "", fmt.Errorf("server stdout closed before 'listening on' log line")
		}
		return &serverProc{cmd: cmd, process: cmd.Process}, port, nil
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		return nil, "", fmt.Errorf("timeout waiting for server 'listening on' log line")
	}
}

// goroutineCount fetches /debug/pprof/goroutine?debug=1 and parses the
// "goroutine profile: total NNN" header. Reliable on all Go versions.
func goroutineCount(pprofAddr string) (int, error) {
	url := fmt.Sprintf("http://%s/debug/pprof/goroutine?debug=1", pprofAddr)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile(`goroutine profile: total (\d+)`)
	m := re.FindSubmatch(body)
	if m == nil {
		return 0, fmt.Errorf("could not parse goroutine count from pprof output")
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0, fmt.Errorf("parse count: %w", err)
	}
	return n, nil
}

// buildSchedule generates the skill-pair sequence per Oracle Q2:
// matches==100 → each of 10x10 pairs once shuffled.
// matches<100  → sample without replacement.
// matches>100  → cycle the shuffled 100-pair list.
func buildSchedule(matches int, rng *rand.Rand) [][2]int {
	allPairs := make([][2]int, 0, 100)
	for p1 := 0; p1 < 10; p1++ {
		for p2 := 0; p2 < 10; p2++ {
			allPairs = append(allPairs, [2]int{p1, p2})
		}
	}
	rng.Shuffle(len(allPairs), func(i, j int) { allPairs[i], allPairs[j] = allPairs[j], allPairs[i] })

	out := make([][2]int, matches)
	if matches <= 100 {
		copy(out, allPairs[:matches])
	} else {
		for i := 0; i < matches; i++ {
			out[i] = allPairs[i%100]
		}
	}
	return out
}

// runMatch executes one bot-vs-bot match end-to-end:
//  1. Create room, p1 joins implicitly via creation
//  2. p2 joins via /api/room/{code}/join
//  3. Spawn 2 bot subprocesses with proper credentials
//  4. Wait for both to exit (or timeout → kill)
//  5. Read both summaries; validate winner agreement
func runMatch(idx, p1Skill, p2Skill int, httpBase, wsBase, botBin, tmpDir string, timeout time.Duration) matchResult {
	res := matchResult{
		Index:   idx,
		P1Skill: p1Skill,
		P2Skill: p2Skill,
	}
	start := time.Now()
	defer func() { res.WallTimeSec = time.Since(start).Seconds() }()

	cr, err := createRoom(httpBase, fmt.Sprintf("LoadP1-%d", idx))
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "create_room: " + err.Error()
		return res
	}
	res.RoomCode = cr.RoomCode
	if cr.PlayerID != 1 {
		res.Status = "FAIL"
		res.FailReason = fmt.Sprintf("create_room returned player_id=%d (expected 1)", cr.PlayerID)
		return res
	}

	jr, err := joinRoom(httpBase, cr.RoomCode, fmt.Sprintf("LoadP2-%d", idx))
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "join_room: " + err.Error()
		return res
	}
	if jr.PlayerID != 2 {
		res.Status = "FAIL"
		res.FailReason = fmt.Sprintf("join_room returned player_id=%d (expected 2)", jr.PlayerID)
		return res
	}

	p1Summary := filepath.Join(tmpDir, fmt.Sprintf("match-%04d-p1.json", idx))
	p2Summary := filepath.Join(tmpDir, fmt.Sprintf("match-%04d-p2.json", idx))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Spawn both bots in parallel with their own captured output buffers
	// (Oracle Q8: drain pipes to prevent buffer-deadlock).
	p1Out, p1Err := runBot(ctx, botBin, wsBase, cr.RoomCode, 1, cr.Token, p1Skill, p1Summary, fmt.Sprintf("LoadP1-%d", idx))
	p2Out, p2Err := runBot(ctx, botBin, wsBase, cr.RoomCode, 2, jr.Token, p2Skill, p2Summary, fmt.Sprintf("LoadP2-%d", idx))

	wait1 := <-p1Out
	wait2 := <-p2Out
	out1 := <-p1Err
	out2 := <-p2Err

	if wait1 != nil || wait2 != nil {
		res.Status = "FAIL"
		res.FailReason = fmt.Sprintf("bot exit: p1=%v p2=%v | p1tail=%s | p2tail=%s",
			wait1, wait2, lastNLines(out1, 5), lastNLines(out2, 5))
		return res
	}

	s1, err := readSummary(p1Summary)
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "read p1 summary: " + err.Error()
		return res
	}
	s2, err := readSummary(p2Summary)
	if err != nil {
		res.Status = "FAIL"
		res.FailReason = "read p2 summary: " + err.Error()
		return res
	}
	res.P1Summary = s1
	res.P2Summary = s2

	if s1.Winner != s2.Winner {
		res.Status = "FAIL"
		res.FailReason = fmt.Sprintf("winner disagreement: p1=%d p2=%d", s1.Winner, s2.Winner)
		return res
	}
	if s1.Winner < 1 || s1.Winner > 2 {
		res.Status = "FAIL"
		res.FailReason = fmt.Sprintf("invalid winner: %d", s1.Winner)
		return res
	}

	res.Winner = s1.Winner
	res.Ticks = s1.Ticks
	res.Status = "PASS"
	return res
}

// runBot spawns a single bot subprocess and returns two channels:
// the first delivers the final exec error (nil on clean exit) and the
// second delivers the captured combined stdout+stderr. Stdout/stderr are
// both drained into an in-memory bytes.Buffer (capped) to prevent pipe
// buffer deadlock.
func runBot(ctx context.Context, botBin, wsBase, room string, playerID int, token string, skill int, summaryPath, name string) (chan error, chan []byte) {
	errCh := make(chan error, 1)
	outCh := make(chan []byte, 1)

	var buf bytes.Buffer
	var bufMu sync.Mutex
	cappedWriter := &cappedBuffer{buf: &buf, mu: &bufMu, cap: 64 * 1024}

	cmd := exec.CommandContext(ctx, botBin,
		"-server", wsBase,
		"-room", room,
		"-player-id", strconv.Itoa(playerID),
		"-token", token,
		"-name", name,
		"-skill", strconv.Itoa(skill),
		"-exit-after-gameover",
		"-summary-file", summaryPath,
	)
	cmd.Stdout = cappedWriter
	cmd.Stderr = cappedWriter

	if err := cmd.Start(); err != nil {
		errCh <- fmt.Errorf("bot start: %w", err)
		outCh <- nil
		return errCh, outCh
	}

	go func() {
		err := cmd.Wait()
		errCh <- err
		bufMu.Lock()
		out := append([]byte(nil), buf.Bytes()...)
		bufMu.Unlock()
		outCh <- out
	}()

	return errCh, outCh
}

// cappedBuffer is a thread-safe writer that discards bytes once cap is reached.
// Prevents unbounded memory growth from chatty bots over 100 matches.
type cappedBuffer struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
	cap int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := c.cap - c.buf.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		c.buf.Write(p[:remaining])
		return len(p), nil
	}
	c.buf.Write(p)
	return len(p), nil
}

func lastNLines(b []byte, n int) string {
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, " | ")
	}
	return strings.Join(lines[len(lines)-n:], " | ")
}

func createRoom(httpBase, playerName string) (*createRoomResponse, error) {
	body := fmt.Sprintf(`{"difficulty":5,"player_name":%q,"vs_ai":false,"auto_fill_bot":false}`, playerName)
	req, _ := http.NewRequest("POST", httpBase+"/api/room", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	var r createRoomResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func joinRoom(httpBase, code, playerName string) (*joinResponse, error) {
	body := fmt.Sprintf(`{"player_name":%q}`, playerName)
	url := fmt.Sprintf("%s/api/room/%s/join", httpBase, code)
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}
	var r joinResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func readSummary(path string) (*matchSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s matchSummary
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal: %w (raw: %s)", err, string(data))
	}
	return &s, nil
}

func aggregateByPair(results []matchResult) []pairAggregate {
	type key struct{ p1, p2 int }
	bucket := make(map[key]*pairAggregate)
	for _, r := range results {
		if r.Status == "" {
			continue
		}
		k := key{r.P1Skill, r.P2Skill}
		agg, ok := bucket[k]
		if !ok {
			agg = &pairAggregate{P1Skill: r.P1Skill, P2Skill: r.P2Skill}
			bucket[k] = agg
		}
		agg.Played++
		if r.Status == "FAIL" {
			agg.Failures++
			continue
		}
		if r.Winner == 1 {
			agg.P1Wins++
		} else if r.Winner == 2 {
			agg.P2Wins++
		}
		agg.AvgTicks += float64(r.Ticks)
		agg.AvgWallSec += r.WallTimeSec
	}
	out := make([]pairAggregate, 0, len(bucket))
	for _, agg := range bucket {
		nonFail := float64(agg.Played - agg.Failures)
		if nonFail > 0 {
			agg.AvgTicks /= nonFail
			agg.AvgWallSec /= nonFail
		}
		out = append(out, *agg)
	}
	return out
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[bot-load] FATAL: "+format+"\n", args...)
	os.Exit(2)
}
