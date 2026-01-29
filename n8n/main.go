package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Concurrency      int           `json:"concurrency"`
	HTTPTimeout      Duration      `json:"http_timeout"`
	InsecureTLS      bool          `json:"insecure_tls"`
	DefaultTailLines int           `json:"default_tail_lines"`
	Agents           []AgentConfig `json:"agents"`
}

type AgentConfig struct {
	Name       string       `json:"name"`
	BaseURL    string       `json:"base_url"`
	APIKey     string       `json:"api_key,omitempty"`
	APIVersion int          `json:"api_version,omitempty"` // default 1
	Checks     ChecksConfig `json:"checks"`
	Logs       LogConfig    `json:"logs"`
}

type ChecksConfig struct {
	Healthz   bool `json:"healthz"`
	Readiness bool `json:"readiness"`
	Metrics   bool `json:"metrics"`
	API       bool `json:"api"`
}

type LogConfig struct {
	// type: "" | "file" | "docker" | "command" | "ssh"
	Type string `json:"type"`

	// how many lines to tail (0 => use global default)
	TailLines int `json:"tail_lines,omitempty"`

	// file
	Path string `json:"path,omitempty"`

	// docker
	Container  string   `json:"container,omitempty"`
	DockerArgs []string `json:"docker_args,omitempty"`

	// command (local)
	Cmd string `json:"cmd,omitempty"`

	// ssh (remote command)
	SSHHost string   `json:"ssh_host,omitempty"`
	SSHArgs []string `json:"ssh_args,omitempty"`
	SSHCMD  string   `json:"ssh_cmd,omitempty"`
}

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "" || s == "null" {
		d.Duration = 0
		return nil
	}
	dd, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dd
	return nil
}

type EndpointResult struct {
	URL      string        `json:"url"`
	OK       bool          `json:"ok"`
	Status   int           `json:"status"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
	BodyHint string        `json:"body_hint,omitempty"`
}

type AgentResult struct {
	Name       string          `json:"name"`
	BaseURL    string          `json:"base_url"`
	Working    bool            `json:"working"`
	CheckedAt  time.Time       `json:"checked_at"`
	Healthz    *EndpointResult `json:"healthz,omitempty"`
	Readiness  *EndpointResult `json:"readiness,omitempty"`
	Metrics    *EndpointResult `json:"metrics,omitempty"`
	API        *EndpointResult `json:"api,omitempty"`
	LogTail    string          `json:"log_tail,omitempty"`
	LogSummary string          `json:"log_summary,omitempty"`
	Notes      []string        `json:"notes,omitempty"`
}

func main() {
	var (
		cfgPath    = flag.String("config", "n8n-agents.json", "Path to config JSON")
		jsonOut    = flag.Bool("json", false, "Output JSON")
		writeCfg   = flag.Bool("write-config", false, "If config file is missing and you enter values, write it to -config")
	)
	flag.Parse()

	cfg, cfgFromCLI, err := loadConfigOrPrompt(*cfgPath)
	if err != nil {
		fatal(err)
	}
	applyDefaults(&cfg)

	if cfgFromCLI && *writeCfg {
		if err := writeConfigFile(*cfgPath, cfg); err != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to write config:", err)
		} else {
			fmt.Fprintln(os.Stderr, "saved config to", *cfgPath)
		}
	}

	client := makeHTTPClient(cfg.HTTPTimeout, cfg.InsecureTLS)

	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup
	results := make([]AgentResult, 0, len(cfg.Agents))
	var mu sync.Mutex

	for _, agent := range cfg.Agents {
		agent := agent
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := checkAgent(client, cfg, agent)
			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}()
	}

	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
		return
	}

	for _, r := range results {
		status := "OK"
		if !r.Working {
			status = "FAIL"
		}
		fmt.Printf("\n== %s (%s) -> %s ==\n", r.Name, r.BaseURL, status)
		for _, n := range r.Notes {
			fmt.Printf("  note: %s\n", n)
		}
		printEndpoint("healthz", r.Healthz)
		printEndpoint("readiness", r.Readiness)
		printEndpoint("metrics", r.Metrics)
		printEndpoint("api", r.API)

		if r.LogSummary != "" {
			fmt.Printf("  logs: %s\n", r.LogSummary)
		}
		if r.LogTail != "" {
			fmt.Printf("---- log tail ----\n%s\n------------------\n", r.LogTail)
		}
	}
}

func printEndpoint(label string, r *EndpointResult) {
	if r == nil {
		return
	}
	ok := "OK"
	if !r.OK {
		ok = "BAD"
	}
	hint := ""
	if r.BodyHint != "" {
		hint = " | " + r.BodyHint
	}
	if r.Error != "" {
		fmt.Printf("  %-9s %s (%d) in %s | err=%s%s\n", label, ok, r.Status, r.Duration, r.Error, hint)
	} else {
		fmt.Printf("  %-9s %s (%d) in %s%s\n", label, ok, r.Status, r.Duration, hint)
	}
}

// ---------- Config loading / prompting ----------

func loadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// returns (cfg, fromCLI, err)
func loadConfigOrPrompt(path string) (Config, bool, error) {
	cfg, err := loadConfig(path)
	if err == nil {
		return cfg, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, false, err
	}

	fmt.Fprintf(os.Stderr, "config file not found (%s) — using interactive CLI input\n", path)
	cfg, err = promptConfigFromCLI()
	return cfg, true, err
}

func writeConfigFile(path string, cfg Config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// 0640 is reasonable for configs that may include API keys
	return os.WriteFile(path, b, 0o640)
}

func promptConfigFromCLI() (Config, error) {
	in := bufio.NewReader(os.Stdin)

	// Defaults
	cfg := Config{
		Concurrency:      5,
		HTTPTimeout:      Duration{Duration: 8 * time.Second},
		InsecureTLS:      false,
		DefaultTailLines: 200,
	}

	cfg.Concurrency = promptInt(in, "Concurrency", cfg.Concurrency)
	cfg.HTTPTimeout = Duration{Duration: promptDuration(in, "HTTP timeout (e.g. 8s, 2m)", cfg.HTTPTimeout.Duration)}
	cfg.InsecureTLS = promptBool(in, "Allow insecure TLS (self-signed certs)", cfg.InsecureTLS)
	cfg.DefaultTailLines = promptInt(in, "Default log tail lines", cfg.DefaultTailLines)

	nAgents := promptInt(in, "How many agents", 1)
	if nAgents < 1 {
		nAgents = 1
	}
	cfg.Agents = make([]AgentConfig, 0, nAgents)

	for i := 1; i <= nAgents; i++ {
		fmt.Printf("\n--- Agent %d ---\n", i)

		a := AgentConfig{
			Name:       fmt.Sprintf("agent-%d", i),
			BaseURL:    "https://n8n-mutevazipeynircilik.com:5678",
			APIVersion: 1,
			Checks: ChecksConfig{
				Healthz:   true,
				Readiness: true,
				Metrics:   false,
				API:       false,
			},
			Logs: LogConfig{
				Type:      "",
				TailLines: 0,
			},
		}

		a.Name = promptString(in, "Name", a.Name)
		a.BaseURL = promptString(in, "Base URL", a.BaseURL)


		a.APIKey = promptString(in, "API key (optional)", "de92a284ds39-f8u303-d8dj9-28hdak83nb3rt")
		a.APIVersion = promptInt(in, "API version", a.APIVersion)

		a.Checks.Healthz = promptBool(in, "Check /healthz", a.Checks.Healthz)
		a.Checks.Readiness = promptBool(in, "Check /healthz/readiness", a.Checks.Readiness)
		a.Checks.Metrics = promptBool(in, "Check /metrics", a.Checks.Metrics)

		defaultAPICheck := strings.TrimSpace(a.APIKey) != ""
		a.Checks.API = promptBool(in, "Check API (/api/vX/... requires API key)", defaultAPICheck)

		logType := promptString(in, `Logs type: "" (none), file, docker, command, ssh`, a.Logs.Type)
		a.Logs.Type = strings.ToLower(strings.TrimSpace(logType))

		if a.Logs.Type != "" {
			a.Logs.TailLines = promptInt(in, "Log tail lines (0 = use default)", 0)
		}

		switch a.Logs.Type {
		case "":
			// none
		case "file":
			a.Logs.Path = promptString(in, "Log file path", "/var/log/n8n/n8n.log")
		case "docker":
			a.Logs.Container = promptString(in, "Docker container name", "n8n")
			extra := promptString(in, `Docker extra args (optional, space-separated, e.g. "--since 10m --timestamps")`, "")
			if strings.TrimSpace(extra) != "" {
				a.Logs.DockerArgs = strings.Fields(extra)
			}
		case "command":
			a.Logs.Cmd = promptString(in, `Command to run (e.g. "journalctl -u n8n --no-pager -n 200")`, "")
			if strings.TrimSpace(a.Logs.Cmd) == "" {
				return Config{}, errors.New("logs.type=command selected but command was empty")
			}
		case "ssh":
			a.Logs.SSHHost = promptString(in, `SSH host (e.g. "agentdev@agent.mutevazipeynircilik.com")`, "")
			a.Logs.SSHCMD = promptString(in, `Remote command (e.g. "docker logs --tail 200 n8n --timestamps")`, "")
			sshArgs := promptString(in, `SSH args (optional, space-separated, e.g. "-i ~/.ssh -p 22")`, "")
			if strings.TrimSpace(sshArgs) != "" {
				a.Logs.SSHArgs = strings.Fields(sshArgs)
			}
			if strings.TrimSpace(a.Logs.SSHHost) == "" || strings.TrimSpace(a.Logs.SSHCMD) == "" {
				return Config{}, errors.New("logs.type=ssh selected but ssh_host or ssh_cmd was empty")
			}
		default:
			return Config{}, fmt.Errorf("unknown logs.type: %q", a.Logs.Type)
		}

		cfg.Agents = append(cfg.Agents, a)
	}

	return cfg, nil
}

func promptString(in *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	s, err := in.ReadString('\n')
	if err != nil && strings.TrimSpace(s) == "" {
		return def
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	return s
}

func promptInt(in *bufio.Reader, label string, def int) int {
	for {
		fmt.Printf("%s [%d]: ", label, def)
		s, err := in.ReadString('\n')
		if err != nil && strings.TrimSpace(s) == "" {
			return def
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		v, err := strconv.Atoi(s)
		if err == nil {
			return v
		}
		fmt.Println("  please enter a valid integer")
	}
}

func promptBool(in *bufio.Reader, label string, def bool) bool {
	defStr := "n"
	if def {
		defStr = "y"
	}
	for {
		fmt.Printf("%s [y/n] (default %s): ", label, defStr)
		s, err := in.ReadString('\n')
		if err != nil && strings.TrimSpace(s) == "" {
			return def
		}
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return def
		}
		if s == "y" || s == "yes" || s == "true" || s == "1" {
			return true
		}
		if s == "n" || s == "no" || s == "false" || s == "0" {
			return false
		}
		fmt.Println("  please enter y or n")
	}
}

func promptDuration(in *bufio.Reader, label string, def time.Duration) time.Duration {
	for {
		fmt.Printf("%s [%s]: ", label, def.String())
		s, err := in.ReadString('\n')
		if err != nil && strings.TrimSpace(s) == "" {
			return def
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		d, err := time.ParseDuration(s)
		if err == nil {
			return d
		}
		fmt.Println("  please enter a valid duration like 8s, 1m, 2h")
	}
}

// ---------- Defaults / HTTP ----------

func applyDefaults(cfg *Config) {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5
	}
	if cfg.HTTPTimeout.Duration <= 0 {
		cfg.HTTPTimeout.Duration = 8 * time.Second
	}
	if cfg.DefaultTailLines <= 0 {
		cfg.DefaultTailLines = 200
	}
	for i := range cfg.Agents {
		if cfg.Agents[i].APIVersion <= 0 {
			cfg.Agents[i].APIVersion = 1
		}
	}
}

func makeHTTPClient(timeout Duration, insecureTLS bool) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureTLS}, // optional for self-signed
	}
	return &http.Client{Timeout: timeout.Duration, Transport: tr}
}

// ---------- Agent checks ----------

func checkAgent(client *http.Client, cfg Config, a AgentConfig) AgentResult {
	res := AgentResult{
		Name:      a.Name,
		BaseURL:   a.BaseURL,
		CheckedAt: time.Now(),
	}

	base, err := url.Parse(a.BaseURL)
	if err != nil {
		res.Notes = append(res.Notes, "invalid base_url: "+err.Error())
		res.Working = false
		return res
	}

	if a.Checks.Healthz {
		u := resolve(base, "/healthz")
		r := doGET(client, u, nil)
		res.Healthz = &r
		if r.Status == 404 {
			res.Notes = append(res.Notes, "/healthz returned 404 (likely not enabled)")
		}
	}

	if a.Checks.Readiness {
		u := resolve(base, "/healthz/readiness")
		r := doGET(client, u, nil)
		res.Readiness = &r
		if r.Status == 404 {
			res.Notes = append(res.Notes, "/healthz/readiness returned 404 (likely not enabled)")
		}
	}

	if a.Checks.Metrics {
		u := resolve(base, "/metrics")
		r := doGET(client, u, nil)
		res.Metrics = &r
		if r.Status == 404 {
			res.Notes = append(res.Notes, "/metrics returned 404 (likely not enabled)")
		}
	}

	if a.Checks.API {
		if strings.TrimSpace(a.APIKey) == "" {
			res.Notes = append(res.Notes, "api check enabled but api_key missing; skipping API check")
		} else {
			path := fmt.Sprintf("/api/v%d/workflows?active=true&limit=1", a.APIVersion)
			u := resolve(base, path)
			h := map[string]string{"X-N8N-API-KEY": a.APIKey}
			r := doGET(client, u, h)
			res.API = &r
		}
	}

	// Working if any key check is OK (prefer readiness)
	res.Working = anyOK(res.Readiness, res.Healthz, res.API)

	// Logs
	lines := a.Logs.TailLines
	if lines <= 0 {
		lines = cfg.DefaultTailLines
	}

	logTail, logErr := fetchLogs(a.Logs, lines)
	if logErr != nil {
		res.Notes = append(res.Notes, "log fetch failed: "+logErr.Error())
	} else {
		res.LogTail = strings.TrimRight(logTail, "\n")
		if res.LogTail != "" {
			res.LogSummary = summarizeLogs(res.LogTail)
		}
	}

	return res
}

func anyOK(rs ...*EndpointResult) bool {
	for _, r := range rs {
		if r != nil && r.OK {
			return true
		}
	}
	return false
}

func resolve(base *url.URL, path string) string {
	u := *base
	ref := &url.URL{Path: strings.TrimPrefix(path, "/")}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + ref.Path
	return u.String()
}

func doGET(client *http.Client, urlStr string, headers map[string]string) EndpointResult {
	start := time.Now()
	r := EndpointResult{URL: urlStr}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		r.Error = err.Error()
		r.Duration = time.Since(start)
		return r
	}
	req.Header.Set("User-Agent", "n8n-agent-check/1.0")
	req.Header.Set("Cache-Control", "no-cache")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		r.Error = err.Error()
		r.Duration = time.Since(start)
		return r
	}
	defer resp.Body.Close()

	r.Status = resp.StatusCode
	r.Duration = time.Since(start)

	if (resp.StatusCode >= 200 && resp.StatusCode <= 299) || resp.StatusCode == 304 {
		r.OK = true
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if len(body) > 0 {
		s := strings.TrimSpace(string(body))
		if len(s) > 160 {
			s = s[:160] + "..."
		}
		r.BodyHint = s
	}

	return r
}

// ---------- Logs ----------

func fetchLogs(lc LogConfig, tailLines int) (string, error) {
	switch strings.ToLower(strings.TrimSpace(lc.Type)) {
	case "":
		return "", nil
	case "file":
		if lc.Path == "" {
			return "", errors.New("logs.type=file requires logs.path")
		}
		return tailFile(lc.Path, tailLines)
	case "docker":
		if lc.Container == "" {
			return "", errors.New("logs.type=docker requires logs.container")
		}
		args := []string{"logs", "--tail", fmt.Sprintf("%d", tailLines)}
		args = append(args, lc.DockerArgs...)
		args = append(args, lc.Container)
		return runCmd("docker", args...)
	case "command":
		if lc.Cmd == "" {
			return "", errors.New("logs.type=command requires logs.cmd")
		}
		return runShell(lc.Cmd)
	case "ssh":
		if lc.SSHHost == "" || lc.SSHCMD == "" {
			return "", errors.New("logs.type=ssh requires logs.ssh_host and logs.ssh_cmd")
		}
		args := append([]string{}, lc.SSHArgs...)
		args = append(args, lc.SSHHost, lc.SSHCMD)
		return runCmd("ssh", args...)
	default:
		return "", fmt.Errorf("unknown logs.type: %q", lc.Type)
	}
}

func runCmd(bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var out bytes.Buffer
	var errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("%s timed out", bin)
	}
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), errors.New(msg)
	}
	return out.String(), nil
}

func runShell(command string) (string, error) {
	return runCmd("/bin/sh", "-c", command)
}

func tailFile(path string, lines int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.Size() == 0 {
		return "", nil
	}

	const chunk = 32 * 1024
	var (
		buf     []byte
		pos     = st.Size()
		lineCnt = 0
	)

	for pos > 0 && lineCnt <= lines {
		readSize := int64(chunk)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize

		tmp := make([]byte, readSize)
		if _, err := f.ReadAt(tmp, pos); err != nil && err != io.EOF {
			return "", err
		}
		buf = append(tmp, buf...)
		lineCnt = bytes.Count(buf, []byte{'\n'})
		if lineCnt > lines+5 {
			break
		}
	}

	parts := strings.Split(string(buf), "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n"), nil
}

func summarizeLogs(tail string) string {
	l := strings.ToLower(tail)
	errCnt := strings.Count(l, " error ") + strings.Count(l, "\terror") + strings.Count(l, "error:")
	warnCnt := strings.Count(l, " warn ") + strings.Count(l, "\twarn") + strings.Count(l, "warn:")
	errCnt += strings.Count(l, `"level":"error"`)
	warnCnt += strings.Count(l, `"level":"warn"`)

	s := fmt.Sprintf("tail=%d chars", len(tail))
	if errCnt > 0 || warnCnt > 0 {
		s += fmt.Sprintf(", errors~%d, warns~%d", errCnt, warnCnt)
	}
	return s
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
