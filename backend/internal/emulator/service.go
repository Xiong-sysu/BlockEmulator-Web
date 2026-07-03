package emulator

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Service struct {
	emulatorRoot string
	workdir      string
	logPath      string
	logDir       string

	mu         sync.Mutex
	status     ExperimentStatus
	commands   []*exec.Cmd
	lastConfig WebConfig
}

func NewService(emulatorRoot, workdir string) (*Service, error) {
	if _, err := os.Stat(filepath.Join(emulatorRoot, "config.yaml")); err != nil {
		return nil, fmt.Errorf("BlockEmulator-X root not found at %s: %w", emulatorRoot, err)
	}
	if err := os.MkdirAll(workdir, 0755); err != nil {
		return nil, err
	}
	s := &Service{
		emulatorRoot: emulatorRoot,
		workdir:      workdir,
		logPath:      filepath.Join(workdir, "experiment.log"),
		logDir:       filepath.Join(workdir, "exp", "logs"),
		status: ExperimentStatus{
			Status:    "idle",
			Message:   "ready",
			UpdatedAt: time.Now(),
		},
	}
	return s, nil
}

func (s *Service) configPath() string {
	return filepath.Join(s.emulatorRoot, "config.yaml")
}

func (s *Service) generatedConfigPath() string {
	return filepath.Join(s.workdir, "generated_config.yaml")
}

func (s *Service) ipTablePath() string {
	return filepath.Join(s.workdir, "generated_ip_table.json")
}

func (s *Service) experimentDir() string {
	return filepath.Join(s.workdir, "exp")
}

func (s *Service) StartExperiment(cfg WebConfig) (ExperimentStatus, error) {
	if err := ValidateConfig(cfg); err != nil {
		return ExperimentStatus{}, err
	}

	s.mu.Lock()
	if s.status.Status == "running" {
		defer s.mu.Unlock()
		return s.status, fmt.Errorf("%w: stop the current experiment before starting a new one", ErrRunning)
	}
	s.status = ExperimentStatus{
		Status:    "running",
		StartedAt: time.Now().Format(time.RFC3339),
		Message:   "preparing experiment",
		UpdatedAt: time.Now(),
	}
	s.commands = nil
	s.lastConfig = cfg
	s.mu.Unlock()

	if err := s.resetExperimentDir(); err != nil {
		return s.markFailed(err), err
	}
	if err := s.SaveConfig(cfg); err != nil {
		return s.markFailed(err), err
	}
	if _, err := s.SaveIPTable(cfg.System.ShardNum, cfg.System.NodeNum); err != nil {
		return s.markFailed(err), err
	}
	if err := s.runBuild(); err != nil {
		return s.markFailed(err), err
	}
	if err := s.startProcesses(cfg); err != nil {
		return s.markFailed(err), err
	}

	status := s.Status()
	status.Message = "experiment is running"
	s.setStatus(status)
	return status, nil
}

func (s *Service) StopExperiment(message string) (ExperimentStatus, error) {
	s.mu.Lock()
	commands := append([]*exec.Cmd(nil), s.commands...)
	s.mu.Unlock()

	for _, cmd := range commands {
		if cmd.Process == nil {
			continue
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}

	status := s.Status()
	status.Status = "stopped"
	status.FinishedAt = time.Now().Format(time.RFC3339)
	status.Message = message
	status.UpdatedAt = time.Now()
	status.PIDs = nil
	s.setStatus(status)
	return status, nil
}

func (s *Service) Status() ExperimentStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status
	status.PIDs = append([]int(nil), s.status.PIDs...)
	return status
}

func (s *Service) Logs(source string) map[string]string {
	var path string
	switch {
	case source == "" || source == "all":
		path = s.logPath
	case source == "supervisor":
		path = filepath.Join(s.logDir, "shard=2147483647_node=0", "info.log")
	default:
		// expected format: "s{shardId}_n{nodeId}"
		path = filepath.Join(s.logDir, source, "info.log")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{"text": ""}
	}
	text := string(data)
	if len(text) > 50000 {
		text = text[len(text)-50000:]
	}
	return map[string]string{"text": text}
}

func (s *Service) LogSources() []string {
	// 1. Scan log directory for actual per-node log files
	entries, err := os.ReadDir(s.logDir)
	if err == nil {
		sources := []string{"all"}
		hasSupervisor := false
		hasNodes := false
		for _, entry := range entries {
			name := entry.Name()
			re := regexp.MustCompile(`^shard=(\d+)_node=(\d+)$`)
			if name == "shard=2147483647_node=0" {
				hasSupervisor = true
				continue
			}
			if re.MatchString(name) {
				sources = append(sources, name)
				hasNodes = true
			}
		}
		if hasSupervisor || hasNodes {
			if hasSupervisor {
				sources = append([]string{"all", "supervisor"}, sources[1:]...)
			}
			return sources
		}
	}

	// 2. Fallback: compute from config
	s.mu.Lock()
	cfg := s.lastConfig
	s.mu.Unlock()

	if cfg.System.ShardNum == 0 {
		cfg = s.LoadConfig()
	}

	sources := []string{"all"}
	if cfg.System.ShardNum > 0 {
		sources = append(sources, "supervisor")
		for sid := int64(0); sid < cfg.System.ShardNum; sid++ {
			for nid := int64(0); nid < cfg.System.NodeNum; nid++ {
				sources = append(sources, fmt.Sprintf("shard=%d_node=%d", sid, nid))
			}
		}
	}
	return sources
}

func (s *Service) Results() (Results, error) {
	resultDir := filepath.Join(s.experimentDir(), "results")
	files, _ := filepath.Glob(filepath.Join(resultDir, "*.csv"))
	fileNames := make([]string, 0, len(files))
	for _, file := range files {
		fileNames = append(fileNames, filepath.Base(file))
	}

	brief := chooseFirstExisting(resultDir, "relay_stats_brief_info.csv", "broker_stats_brief_info.csv")
	if brief == "" {
		return Results{Files: fileNames}, nil
	}

	columns, rows, err := readCSVTable(brief)
	if err != nil {
		return Results{}, err
	}

	base := filepath.Base(brief)
	mode := "relay"
	detail := "relay_stats_detail_tx_info.csv"
	if strings.HasPrefix(base, "broker") {
		mode = "broker"
		detail = "broker_stats_detail_tx_info.csv"
	}

	return Results{
		Mode:        mode,
		BriefFile:   base,
		DetailFile:  detail,
		Columns:     columns,
		Rows:        rows,
		Files:       fileNames,
		DownloadURL: "/api/results/download/" + base,
	}, nil
}

func (s *Service) ResultFile(name string) (string, error) {
	clean := filepath.Base(name)
	if clean == "." || clean == string(filepath.Separator) || !strings.HasSuffix(clean, ".csv") {
		return "", fmt.Errorf("%w: invalid result file", ErrNotFound)
	}
	path := filepath.Join(s.experimentDir(), "results", clean)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%w: result file %s", ErrNotFound, clean)
	}
	return path, nil
}

func (s *Service) resetExperimentDir() error {
	if err := os.RemoveAll(s.experimentDir()); err != nil {
		return err
	}
	if err := os.MkdirAll(s.logDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(s.logPath, []byte(""), 0644)
}

func (s *Service) runBuild() error {
	s.appendLog("running go build ./...\n")
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = s.emulatorRoot
	out, err := cmd.CombinedOutput()
	s.appendLog(string(out))
	if err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	return nil
}

func (s *Service) startProcesses(cfg WebConfig) error {
	logFile, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	configPath := s.generatedConfigPath()
	ipTablePath := s.ipTablePath()
	for sid := int64(0); sid < cfg.System.ShardNum; sid++ {
		for nid := int64(0); nid < cfg.System.NodeNum; nid++ {
			perLogPath := filepath.Join(s.logDir, fmt.Sprintf("./shard=%d_node=%d/info.log", sid, nid))
			perLogFile, perr := os.OpenFile(perLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if perr != nil {
				_ = logFile.Close()
				return perr
			}
			multiWriter := io.MultiWriter(logFile, perLogFile)

			cmd := exec.Command("go", "run", "cmd/consensusnode/main.go", "-config", configPath, "-ip_table", ipTablePath, "-shard_id", fmt.Sprintf("%d", sid), "-node_id", fmt.Sprintf("%d", nid))
			if err := s.startCommand(cmd, multiWriter, perLogFile); err != nil {
				_ = logFile.Close()
				return err
			}
		}
	}

	supervisorLogPath := filepath.Join(s.logDir, "supervisor.log")
	supervisorLogFile, serr := os.OpenFile(supervisorLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if serr != nil {
		_ = logFile.Close()
		return serr
	}
	supervisorWriter := io.MultiWriter(logFile, supervisorLogFile)

	cmd := exec.Command("go", "run", "cmd/supervisor/main.go", "-config", configPath, "-ip_table", ipTablePath, "-shard_id=0x7fffffff", "-node_id=0")
	if err := s.startCommand(cmd, supervisorWriter, supervisorLogFile); err != nil {
		_ = logFile.Close()
		return err
	}

	go s.watchProcesses(logFile)
	return nil
}

func (s *Service) startCommand(cmd *exec.Cmd, output io.Writer, perLogFile io.Closer) error {
	cmd.Dir = s.emulatorRoot
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = perLogFile.Close()
		return err
	}

	s.mu.Lock()
	s.commands = append(s.commands, cmd)
	s.status.PIDs = append(s.status.PIDs, cmd.Process.Pid)
	s.status.UpdatedAt = time.Now()
	s.mu.Unlock()

	s.appendLog(fmt.Sprintf("started pid=%d command=%s\n", cmd.Process.Pid, strings.Join(cmd.Args, " ")))
	return nil
}

func (s *Service) watchProcesses(logFile *os.File) {
	defer logFile.Close()
	var failed error
	for _, cmd := range s.commands {
		if err := cmd.Wait(); err != nil && failed == nil {
			failed = err
		}
	}

	current := s.Status()
	if current.Status != "running" {
		return
	}
	current.FinishedAt = time.Now().Format(time.RFC3339)
	current.UpdatedAt = time.Now()
	current.PIDs = nil
	if failed != nil {
		current.Status = "failed"
		current.Message = failed.Error()
	} else {
		current.Status = "finished"
		current.Message = "experiment finished"
	}
	s.setStatus(current)
}

func (s *Service) markFailed(err error) ExperimentStatus {
	status := s.Status()
	status.Status = "failed"
	status.FinishedAt = time.Now().Format(time.RFC3339)
	status.Message = err.Error()
	status.UpdatedAt = time.Now()
	s.setStatus(status)
	return status
}

func (s *Service) setStatus(status ExperimentStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	data, _ := json.MarshalIndent(status, "", "  ")
	_ = os.WriteFile(filepath.Join(s.workdir, "status.json"), data, 0644)
}

func (s *Service) appendLog(text string) {
	file, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(text)
}

func chooseFirstExisting(dir string, names ...string) string {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func readCSVTable(path string) ([]string, []map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return nil, nil, nil
	}

	columns := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string)
		for idx, column := range columns {
			if idx < len(record) {
				row[column] = record[idx]
			}
		}
		rows = append(rows, row)
	}
	return columns, rows, nil
}
