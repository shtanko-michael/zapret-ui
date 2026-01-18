package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// repoLatestURL points to the redirect URL that reveals the latest tag.
	repoLatestURL = "https://github.com/Flowseal/zapret-discord-youtube/releases/latest"
	// downloadTemplate builds the direct zip download URL for a given tag.
	downloadTemplate = "https://github.com/Flowseal/zapret-discord-youtube/releases/download/%s/zapret-discord-youtube-%s.zip"
	// createNewConsole is the Windows flag to spawn a process in a new console window.
	createNewConsole = 0x00000010
)

// Service coordinates config, downloads, strategy listing, test runs, and process launches.
type Service struct {
	baseDir     string
	configPath  string
	releasesDir string
	logsDir     string
	config      *Config
	client      *http.Client
}

// Config is persisted state across app launches.
type Config struct {
	Version        string                 `json:"version"`
	LastStrategy   string                 `json:"lastStrategy"`
	LastTestAt     time.Time              `json:"lastTestAt"`
	TestResults    map[string]TestResult  `json:"testResults"`
	BestStrategy   string                 `json:"bestStrategy"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	Running        *RunningInfo           `json:"running,omitempty"`
	TestInProgress bool                   `json:"testInProgress"`
}

// TestResult captures analytics from the official PowerShell test script.
type TestResult struct {
	Name         string    `json:"name"`
	HTTP_OK      int       `json:"httpOk"`
	HTTP_ERR     int       `json:"httpErr"`
	HTTP_UNSUP   int       `json:"httpUnsup"`
	PingOK       int       `json:"pingOk"`
	PingFail     int       `json:"pingFail"`
	Fail         int       `json:"fail"`
	Blocked      int       `json:"blocked"`
	Status       string    `json:"status"` // ok | fail
	LastTestedAt time.Time `json:"lastTestedAt"`
}

// Strategy is a single general*.bat with its last known test result.
type Strategy struct {
	Name   string     `json:"name"`
	File   string     `json:"file"`
	Result TestResult `json:"result"`
	Best   bool       `json:"best"`
}

// State is the DTO returned to the UI.
type State struct {
	Config      *Config      `json:"config"`
	Strategies  []Strategy   `json:"strategies"`
	LatestTag   string       `json:"latestTag"`
	HasUpdate   bool         `json:"hasUpdate"`
	CurrentPath string       `json:"currentPath"`
	LastTestLog string       `json:"lastTestLog"`
	Running     *RunningInfo `json:"running,omitempty"`
}

// RunningInfo tracks the last launched strategy process.
type RunningInfo struct {
	File      string    `json:"file"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
}

// NewService sets up paths and an HTTP client.
func NewService() *Service {
	base := defaultBaseDir()
	return &Service{
		baseDir:     base,
		configPath:  filepath.Join(base, "config.json"),
		releasesDir: filepath.Join(base, "releases"),
		logsDir:     filepath.Join(base, "logs"),
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func defaultBaseDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "ZapretUI")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "ZapretUI")
}

// ensureDirs prepares required folders.
func (s *Service) ensureDirs() error {
	for _, d := range []string{s.baseDir, s.releasesDir, s.logsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) loadConfig() (*Config, error) {
	if s.config != nil {
		return s.config, nil
	}
	if err := s.ensureDirs(); err != nil {
		return nil, err
	}
	cfg := &Config{
		TestResults: make(map[string]TestResult),
		Meta:        make(map[string]interface{}),
	}
	data, err := os.ReadFile(s.configPath)
	if err == nil {
		_ = json.Unmarshal(data, cfg)
	}
	if cfg.Version == "" {
		// Seed from bundled release if present
		if v, err := s.seedLocalRelease(); err == nil && v != "" {
			cfg.Version = v
		}
	}
	s.config = cfg
	return cfg, nil
}

func (s *Service) saveConfig() error {
	if s.config == nil {
		return errors.New("config nil")
	}
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configPath, data, 0o644)
}

// seedLocalRelease copies a bundled ./release/<ver> into cache and returns the detected version.
func (s *Service) seedLocalRelease() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	releaseRoot := filepath.Join(cwd, "release")
	entries, err := os.ReadDir(releaseRoot)
	if err != nil {
		return "", err
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return "", errors.New("no bundled releases")
	}
	sort.Strings(versions)
	latest := versions[len(versions)-1]
	src := filepath.Join(releaseRoot, latest)
	dst := filepath.Join(s.releasesDir, latest)
	if _, err := os.Stat(dst); err == nil {
		return latest, nil
	}
	if err := copyDir(src, dst); err != nil {
		return "", err
	}
	return latest, nil
}

func (s *Service) State() (*State, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}

	// Rehydrate last test results from disk for initial UI load.
	// Try to refresh in-memory results from the latest test_results file on disk,
	// so cards are populated immediately on app start without re-running tests.
	if current := s.currentReleasePath(); current != "" {
		if latest, err := s.parseLatestResult(current); err == nil && len(latest.Results) > 0 {
			cfg.TestResults = latest.Results
			cfg.BestStrategy = latest.Best
			_ = s.saveConfig()
		}
		// Validate running process if we have one recorded.
		if cfg.Running != nil {
			if !isPIDRunning(cfg.Running.PID) {
				cfg.Running = nil
				_ = s.saveConfig()
			}
		}
	}

	latest, _ := s.latestTag()
	hasUpdate := latest != "" && latest != cfg.Version

	strategies, _ := s.listStrategies()
	for i := range strategies {
		res, ok := cfg.TestResults[strategies[i].Name]
		if ok {
			strategies[i].Result = res
		}
		if cfg.BestStrategy != "" && cfg.BestStrategy == strategies[i].Name {
			strategies[i].Best = true
		}
	}

	return &State{
		Config:      cfg,
		Strategies:  strategies,
		LatestTag:   latest,
		HasUpdate:   hasUpdate,
		CurrentPath: s.currentReleasePath(),
		Running:     cfg.Running,
	}, nil
}

func (s *Service) currentReleasePath() string {
	cfg, err := s.loadConfig()
	if err != nil || cfg.Version == "" {
		return ""
	}
	return filepath.Join(s.releasesDir, cfg.Version)
}

func (s *Service) latestTag() (string, error) {
	req, err := http.NewRequest("GET", repoLatestURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "zapret-ui/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc == "" {
		// maybe already at final URL
		loc = resp.Request.URL.String()
	}
	parts := strings.Split(strings.TrimRight(loc, "/"), "/")
	if len(parts) == 0 {
		return "", errors.New("cannot parse latest tag")
	}
	tag := parts[len(parts)-1]
	return tag, nil
}

func (s *Service) CheckAndUpdate() (*State, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	latest, err := s.latestTag()
	if err != nil {
		return nil, err
	}
	if cfg.Version == latest && latest != "" {
		return s.State()
	}
	if err := s.downloadAndUnpack(latest); err != nil {
		return nil, err
	}
	cfg.Version = latest
	if err := s.saveConfig(); err != nil {
		return nil, err
	}
	return s.State()
}

func (s *Service) downloadAndUnpack(tag string) error {
	if tag == "" {
		return errors.New("tag empty")
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}
	targetDir := filepath.Join(s.releasesDir, tag)
	if fi, err := os.Stat(targetDir); err == nil && fi.IsDir() {
		return nil // already unpacked
	}
	url := fmt.Sprintf(downloadTemplate, tag, tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "zapret-ui/1.0")
	resp, err := (&http.Client{Timeout: 0}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return unzipBuffer(buf, targetDir)
}

func unzipBuffer(data []byte, dest string) error {
	br := bytes.NewReader(data)
	zr, err := zip.NewReader(br, int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		fp := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

func (s *Service) listStrategies() ([]Strategy, error) {
	current := s.currentReleasePath()
	if current == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(current)
	if err != nil {
		return nil, err
	}
	var res []Strategy
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(strings.ToLower(name), "service") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".bat") && strings.HasPrefix(strings.ToLower(name), "general") {
			res = append(res, Strategy{
				Name: name,
				File: filepath.Join(current, name),
			})
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	return res, nil
}

func (s *Service) RunTests() (*State, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	current := s.currentReleasePath()
	if current == "" {
		return nil, errors.New("no current release")
	}
	ps1 := filepath.Join(current, "utils", "test zapret.ps1")
	if _, err := os.Stat(ps1); err != nil {
		return nil, err
	}

	// Remove old test results files to ensure only fresh output is parsed
	resultsDir := filepath.Join(current, "utils", "test results")
	_ = os.RemoveAll(resultsDir)
	_ = os.MkdirAll(resultsDir, 0o755)

	// Clear config and mark tests as in progress for the UI
	cfg.TestResults = make(map[string]TestResult)
	cfg.BestStrategy = ""
	cfg.TestInProgress = true
	cfg.LastTestAt = time.Now()
	_ = s.saveConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	resultCh := make(chan *parsedResults, 1)
	errCh := make(chan error, 1)
	go s.waitForResultFile(ctx, current, resultCh, errCh)

	// auto answers: 1 (standard), 1 (all configs)
	input := bytes.NewBufferString("1\n1\n")

	logFile := filepath.Join(s.logsDir, fmt.Sprintf("test_%d.log", time.Now().Unix()))
	psCmd, psDone, startErr := startPowerShellToLog(ctx, current, ps1, input, logFile)
	if startErr != nil {
		cfg.TestResults = make(map[string]TestResult)
		cfg.BestStrategy = ""
		cfg.TestInProgress = false
		cfg.LastTestAt = time.Now()
		_ = s.saveConfig()
		state, stateErr := s.State()
		if stateErr != nil {
			return nil, stateErr
		}
		return state, startErr
	}

	var parsed *parsedResults
	var watchErr error
	var cmdErr error

waitLoop:
	for {
		select {
		case parsed = <-resultCh:
			watchErr = nil
			// We have results; stop PowerShell even if it is waiting for ReadKey.
			if psCmd != nil && psCmd.Process != nil {
				killProcessTree(psCmd.Process.Pid)
			}
			break waitLoop
		case watchErr = <-errCh:
			if psCmd != nil && psCmd.Process != nil {
				killProcessTree(psCmd.Process.Pid)
			}
			break waitLoop
		case cmdErr = <-psDone:
			// PowerShell exited; if results haven't appeared yet we can still wait until ctx timeout,
			// but usually this means the script failed early.
			if cmdErr != nil {
				// Give watcher a short chance to observe the result file.
				time.Sleep(500 * time.Millisecond)
				break waitLoop
			}
		case <-ctx.Done():
			watchErr = ctx.Err()
			if psCmd != nil && psCmd.Process != nil {
				killProcessTree(psCmd.Process.Pid)
			}
			break waitLoop
		default:
			// Avoid busy wait while the script runs and the watcher polls for the result file.
			time.Sleep(300 * time.Millisecond)
		}
	}

	if parsed != nil {
		cfg.TestResults = parsed.Results
		cfg.BestStrategy = parsed.Best
	} else {
		cfg.TestResults = make(map[string]TestResult)
		cfg.BestStrategy = ""
	}
	cfg.TestInProgress = false
	cfg.LastTestAt = time.Now()
	_ = s.saveConfig()

	state, stateErr := s.State()

	// Bubble up the most relevant error while still returning state for the UI.
	if parsed == nil {
		if watchErr != nil && watchErr != context.Canceled {
			return state, watchErr
		}
		if cmdErr != nil {
			return state, cmdErr
		}
		return state, errors.New("test results file not found")
	}

	if cmdErr != nil {
		return state, cmdErr
	}

	return state, stateErr
}

type parsedResults struct {
	Results map[string]TestResult
	Best    string
}

func (s *Service) parseLatestResult(current string) (*parsedResults, error) {
	dir := filepath.Join(current, "utils", "test results")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var latest os.DirEntry
	var latestTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latest = e
			latestTime = info.ModTime()
		}
	}
	if latest == nil {
		return nil, errors.New("no test results found")
	}
	path := filepath.Join(dir, latest.Name())
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseAnalytics(string(data))
}

// waitForResultFile polls the results directory until a test_results_*.txt file appears and can be parsed.
func (s *Service) waitForResultFile(ctx context.Context, current string, resultCh chan<- *parsedResults, errCh chan<- error) {
	resultsDir := filepath.Join(current, "utils", "test results")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		case <-ticker.C:
			entries, err := os.ReadDir(resultsDir)
			if err != nil {
				errCh <- err
				return
			}
			hasResultFile := false
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := strings.ToLower(e.Name())
				if strings.HasPrefix(name, "test_results_") && strings.HasSuffix(name, ".txt") {
					hasResultFile = true
					break
				}
			}
			if !hasResultFile {
				continue
			}
			parsed, err := s.parseLatestResult(current)
			if err != nil {
				// File may still be being written; keep waiting.
				continue
			}
			resultCh <- parsed
			return
		}
	}
}

func startPowerShellToLog(ctx context.Context, workdir, script string, input *bytes.Buffer, logFile string) (*exec.Cmd, <-chan error, error) {
	args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass"}
	if RUN_PROCESS_HIDDEN {
		// Keep the process non-intrusive for users. For debugging, set RUN_PROCESS_HIDDEN=false.
		args = append(args, "-WindowStyle", "Hidden")
	}
	args = append(args, "-File", script)
	cmd := exec.CommandContext(ctx, "powershell", args...)
	cmd.Dir = workdir
	// Some test scripts use interactive calls (e.g., ReadKey). We still create a console,
	// but hide it by default so it doesn't bother users.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewConsole,
		HideWindow:    RUN_PROCESS_HIDDEN,
	}
	if input != nil {
		cmd.Stdin = input
	}

	_ = os.MkdirAll(filepath.Dir(logFile), 0o755)
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, nil, err
	}
	cmd.Stdout = f
	cmd.Stderr = f

	if err := cmd.Start(); err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		_ = f.Close()
	}()

	return cmd, done, nil
}

func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T", "/F").Run()
}

func parseAnalytics(content string) (*parsedResults, error) {
	lines := strings.Split(content, "\n")
	// inAnalytics := false
	results := make(map[string]TestResult)
	best := ""

	reStd := regexp.MustCompile(`^(.*) : HTTP OK: (\d+), ERR: (\d+), UNSUP: (\d+), Ping OK: (\d+), Fail: (\d+)`)
	reDpi := regexp.MustCompile(`^(.*) : OK: (\d+), FAIL: (\d+), UNSUP: (\d+), BLOCKED: (\d+)`)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "=== ANALYTICS ===" {
			// inAnalytics = true
			continue
		}
		if strings.HasPrefix(line, "Best strategy:") {
			best = strings.TrimSpace(strings.TrimPrefix(line, "Best strategy:"))
			continue
		}
		if line == "" {
			continue
		}
		if m := reStd.FindStringSubmatch(line); len(m) == 7 {
			name := strings.TrimSpace(m[1])
			res := TestResult{
				Name:       name,
				HTTP_OK:    atoi(m[2]),
				HTTP_ERR:   atoi(m[3]),
				HTTP_UNSUP: atoi(m[4]),
				PingOK:     atoi(m[5]),
				PingFail:   atoi(m[6]),
			}
			if res.HTTP_ERR == 0 && res.PingFail == 0 {
				res.Status = "ok"
			} else {
				res.Status = "fail"
			}
			results[name] = res
			continue
		}
		if m := reDpi.FindStringSubmatch(line); len(m) == 6 {
			name := strings.TrimSpace(m[1])
			res := TestResult{
				Name:       name,
				HTTP_OK:    atoi(m[2]),
				Fail:       atoi(m[3]),
				HTTP_UNSUP: atoi(m[4]),
				Blocked:    atoi(m[5]),
			}
			if res.Fail == 0 && res.Blocked == 0 {
				res.Status = "ok"
			} else {
				res.Status = "fail"
			}
			results[name] = res
			continue
		}
	}
	if len(results) == 0 {
		return nil, errors.New("no analytics parsed")
	}
	return &parsedResults{Results: results, Best: best}, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// runPowerShellVisibleWithParsing runs PowerShell script in a visible console window,
// reads output in real-time from stdout/stderr (no temp files), parses for "=== ANALYTICS ===",
// and returns parsed results. Input data is passed via stdin.
func runPowerShellVisibleWithParsing(ctx context.Context, workdir, script string, input *bytes.Buffer, logFile string) (*parsedResults, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	cmd.Dir = workdir
	if input != nil {
		cmd.Stdin = input
	}
	// Show console window (similar to RunStrategy behavior)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	var buf bytes.Buffer
	var mu sync.Mutex
	copyStream := func(r io.Reader) {
		tmp := make([]byte, 4096)
		for {
			n, er := r.Read(tmp)
			if n > 0 {
				mu.Lock()
				buf.Write(tmp[:n])
				mu.Unlock()
			}
			if er != nil {
				if er != io.EOF {
					mu.Lock()
					buf.WriteString("\n[stream error] " + er.Error())
					mu.Unlock()
				}
				return
			}
		}
	}
	go copyStream(stdout)
	go copyStream(stderr)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var result *parsedResults
	var parseErr error

loop:
	for {
		select {
		case <-ctx.Done():
			parseErr = ctx.Err()
			break loop
		case <-ticker.C:
			mu.Lock()
			output := buf.String()
			mu.Unlock()
			if strings.Contains(output, "=== ANALYTICS ===") {
				parsed, err := parseAnalytics(output)
				if err == nil && parsed != nil {
					result = parsed
					parseErr = nil
					break loop
				}
				parseErr = err
				break loop
			}
		}
	}

	// Wait for process completion
	cmdErr := cmd.Wait()

	// Save full output to log
	mu.Lock()
	outCopy := buf.Bytes()
	mu.Unlock()
	_ = os.MkdirAll(filepath.Dir(logFile), 0o755)
	_ = os.WriteFile(logFile, outCopy, 0o644)

	if result == nil {
		if parseErr == nil {
			// If no analytics found but command failed, surface that error
			if cmdErr != nil {
				return nil, fmt.Errorf("command failed: %w", cmdErr)
			}
			return nil, errors.New("analytics section not found in output")
		}
		return nil, parseErr
	}

	// Prefer parsing error if present
	if parseErr != nil {
		return nil, parseErr
	}

	// Return parsed result; surface command error if any (process exit code)
	return result, cmdErr
}

func appendFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err
}

func (s *Service) RunStrategy(file string) (*State, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	// Stop previously running strategy if tracked
	_ = s.StopRunning()

	current := s.currentReleasePath()
	if current == "" {
		return nil, errors.New("no current release")
	}
	full := file
	if !filepath.IsAbs(full) {
		full = filepath.Join(current, file)
	}
	if _, err := os.Stat(full); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// Launch in a visible console window via PowerShell Start-Process and capture PID.
	windowStyle := "Normal"
	if RUN_PROCESS_HIDDEN {
		windowStyle = "Hidden"
	}
	psCmd := fmt.Sprintf("$p = Start-Process -FilePath %q -WorkingDirectory %q -WindowStyle %s -PassThru; Write-Output $p.Id", full, filepath.Dir(full), windowStyle)
	args := []string{"-NoProfile"}
	if RUN_PROCESS_HIDDEN {
		args = append(args, "-WindowStyle", "Hidden")
	}
	args = append(args, "-Command", psCmd)
	cmd := exec.CommandContext(ctx, "powershell", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: RUN_PROCESS_HIDDEN}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	pid := atoi(strings.TrimSpace(buf.String()))
	if pid > 0 {
		cfg.Running = &RunningInfo{
			File:      filepath.Base(full),
			PID:       pid,
			StartedAt: time.Now(),
		}
		_ = s.saveConfig()
	}

	cfg.LastStrategy = filepath.Base(full)
	_ = s.saveConfig()
	return s.State()
}

// StopRunning terminates the tracked running process and all related processes.
func (s *Service) StopRunning() error {
	cfg, err := s.loadConfig()
	if err != nil {
		return err
	}

	// Aggressively kill all winws.exe processes using multiple methods
	// Method 1: taskkill with tree kill (kills process and all children)
	cmd1 := exec.Command("taskkill", "/IM", "winws.exe", "/T", "/F")
	cmd1.Run() // Ignore errors, just try

	// Method 2: PowerShell - more reliable, waits for completion
	psScript := `
$procs = Get-Process -Name winws -ErrorAction SilentlyContinue
if ($procs) {
    $procs | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 200
    # Double-check and kill any remaining
    $remaining = Get-Process -Name winws -ErrorAction SilentlyContinue
    if ($remaining) {
        $remaining | Stop-Process -Force -ErrorAction Stop
    }
}
`
	cmd2 := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psScript)
	cmd2.Run() // Ignore errors, just try

	// Method 3: Also try wmic for additional reliability
	cmd3 := exec.Command("wmic", "process", "where", "name='winws.exe'", "delete")
	cmd3.Run() // Ignore errors

	if cfg.Running != nil {
		// Try to kill the tracked PID (might be cmd.exe or powershell.exe parent)
		if isPIDRunning(cfg.Running.PID) {
			// Use PowerShell Stop-Process for more reliable termination
			_ = exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf("Stop-Process -Id %d -Force -ErrorAction SilentlyContinue", cfg.Running.PID)).Run()
			// Also try taskkill as fallback with tree kill
			_ = exec.Command("taskkill", "/PID", fmt.Sprintf("%d", cfg.Running.PID), "/T", "/F").Run()
		}

		cfg.Running = nil
		_ = s.saveConfig()
	}

	return nil
}

// StopAllRunning is called on shutdown to ensure cleanup.
func (s *Service) StopAllRunning() {
	_ = s.StopRunning()
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// isPIDRunning checks if a process with given pid is alive.
func isPIDRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("pid eq %d", pid)).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%d", pid))
}
