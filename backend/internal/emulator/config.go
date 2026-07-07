package emulator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
	ErrRunning    = errors.New("experiment is running")
)

var allowedConsensus = map[string]bool{
	"static_relay":  true,
	"static_broker": true,
	"clpa_relay":    true,
	"clpa_broker":   true,
}

func (s *Service) LoadConfig() WebConfig {
	path := s.generatedConfigPath()
	if _, err := os.Stat(path); err != nil {
		path = s.configPath()
	}
	cfg, err := readFullConfig(path)
	if err != nil {
		return DefaultConfig()
	}
	return extractWebConfig(cfg)
}

func (s *Service) SaveConfig(webCfg WebConfig) error {
	if err := ValidateConfig(webCfg); err != nil {
		return err
	}
	cfg, err := readFullConfig(s.configPath())
	if err != nil {
		return fmt.Errorf("read base config: %w", err)
	}
	applyWebConfig(cfg, webCfg)
	s.applyRuntimePaths(cfg)
	if err := s.writeConfigSnapshot(webCfg); err != nil {
		return err
	}
	return writeYAML(s.generatedConfigPath(), cfg)
}

func ValidateConfig(cfg WebConfig) error {
	if cfg.System.ShardNum <= 0 {
		return fmt.Errorf("%w: shard_num must be greater than 0", ErrValidation)
	}
	if cfg.System.NodeNum <= 0 {
		return fmt.Errorf("%w: node_num must be greater than 0", ErrValidation)
	}
	if cfg.System.Limit <= 0 {
		return fmt.Errorf("%w: limit must be greater than 0", ErrValidation)
	}
	if !allowedConsensus[cfg.System.ConsensusType] {
		return fmt.Errorf("%w: unsupported consensus_type %q", ErrValidation, cfg.System.ConsensusType)
	}
	if cfg.ConsensusNode.BlockInterval <= 0 {
		return fmt.Errorf("%w: block_interval must be greater than 0", ErrValidation)
	}
	if cfg.Supervisor.TxNumber <= 0 {
		return fmt.Errorf("%w: tx_number must be greater than 0", ErrValidation)
	}
	if cfg.Supervisor.TxInjectionSpeed <= 0 {
		return fmt.Errorf("%w: tx_injection_speed must be greater than 0", ErrValidation)
	}
	if cfg.Supervisor.EpochDuration <= 0 {
		return fmt.Errorf("%w: epoch_duration must be greater than 0", ErrValidation)
	}
	if cfg.Supervisor.TxSourceType != "random_source" && cfg.Supervisor.TxSourceType != "csv_source" {
		return fmt.Errorf("%w: tx_source_type must be random_source or csv_source", ErrValidation)
	}
	if cfg.Supervisor.TxSourceType == "csv_source" && strings.TrimSpace(cfg.Supervisor.TxSourceFile) == "" {
		return fmt.Errorf("%w: tx_source_file is required for csv_source", ErrValidation)
	}
	if cfg.Network.Bandwidth <= 0 {
		return fmt.Errorf("%w: bandwidth must be greater than 0", ErrValidation)
	}
	if cfg.Network.Latency < 0 {
		return fmt.Errorf("%w: latency cannot be negative", ErrValidation)
	}
	return nil
}

func DefaultConfig() WebConfig {
	return WebConfig{
		System: SystemConfig{
			ShardNum:      4,
			NodeNum:       4,
			Limit:         5000,
			ConsensusType: "static_relay",
		},
		ConsensusNode: ConsensusNodeConfig{BlockInterval: 2000},
		Supervisor: SupervisorConfig{
			TxNumber:           100000,
			TxInjectionSpeed:   20000,
			EpochDuration:      50,
			TxSourceType:       "random_source",
			ExcludeContractTxs: true,
		},
		Network: NetworkConfig{Bandwidth: 1000000, Latency: 0},
	}
}

func readFullConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeYAML(path string, cfg map[string]interface{}) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func extractWebConfig(cfg map[string]interface{}) WebConfig {
	def := DefaultConfig()
	def.System.ShardNum = int64FromPath(cfg, def.System.ShardNum, "system", "shard_num")
	def.System.NodeNum = int64FromPath(cfg, def.System.NodeNum, "system", "node_num")
	def.System.Limit = int64FromPath(cfg, def.System.Limit, "system", "limit")
	def.System.ConsensusType = stringFromPath(cfg, def.System.ConsensusType, "system", "consensus_type")
	def.ConsensusNode.BlockInterval = int64FromPath(cfg, def.ConsensusNode.BlockInterval, "consensus_node", "block_interval")
	def.Supervisor.TxNumber = int64FromPath(cfg, def.Supervisor.TxNumber, "supervisor", "tx_number")
	def.Supervisor.TxInjectionSpeed = int64FromPath(cfg, def.Supervisor.TxInjectionSpeed, "supervisor", "tx_injection_speed")
	def.Supervisor.EpochDuration = int64FromPath(cfg, def.Supervisor.EpochDuration, "supervisor", "epoch_duration")
	def.Supervisor.TxSourceType = stringFromPath(cfg, def.Supervisor.TxSourceType, "supervisor", "tx_source", "tx_source_type")
	def.Supervisor.TxSourceFile = stringFromPath(cfg, def.Supervisor.TxSourceFile, "supervisor", "tx_source", "tx_source_file")
	def.Supervisor.ExcludeContractTxs = boolFromPath(cfg, def.Supervisor.ExcludeContractTxs, "supervisor", "tx_source", "exclude_contract_txs")
	def.Network.Bandwidth = int64FromPath(cfg, def.Network.Bandwidth, "network", "bandwidth")
	def.Network.Latency = int64FromPath(cfg, def.Network.Latency, "network", "latency")
	return def
}

func applyWebConfig(cfg map[string]interface{}, webCfg WebConfig) {
	setPath(cfg, webCfg.System.ShardNum, "system", "shard_num")
	setPath(cfg, webCfg.System.NodeNum, "system", "node_num")
	setPath(cfg, webCfg.System.Limit, "system", "limit")
	setPath(cfg, webCfg.System.ConsensusType, "system", "consensus_type")
	setPath(cfg, "direct", "network", "communication_mode")
	setPath(cfg, webCfg.ConsensusNode.BlockInterval, "consensus_node", "block_interval")
	setPath(cfg, webCfg.Supervisor.TxNumber, "supervisor", "tx_number")
	setPath(cfg, webCfg.Supervisor.TxInjectionSpeed, "supervisor", "tx_injection_speed")
	setPath(cfg, webCfg.Supervisor.EpochDuration, "supervisor", "epoch_duration")
	setPath(cfg, webCfg.Supervisor.TxSourceType, "supervisor", "tx_source", "tx_source_type")
	setPath(cfg, webCfg.Supervisor.TxSourceFile, "supervisor", "tx_source", "tx_source_file")
	setPath(cfg, webCfg.Supervisor.ExcludeContractTxs, "supervisor", "tx_source", "exclude_contract_txs")
	setPath(cfg, webCfg.Network.Bandwidth, "network", "bandwidth")
	setPath(cfg, webCfg.Network.Latency, "network", "latency")
}

func (s *Service) SaveIPTable(shards, nodes int64) (map[string]map[string]string, error) {
	if shards <= 0 || nodes <= 0 {
		return nil, fmt.Errorf("%w: shard_num and node_num must be greater than 0", ErrValidation)
	}
	table := make(map[string]map[string]string)
	for sid := int64(0); sid < shards; sid++ {
		table[fmt.Sprintf("%d", sid)] = make(map[string]string)
		for nid := int64(0); nid < nodes; nid++ {
			table[fmt.Sprintf("%d", sid)][fmt.Sprintf("%d", nid)] = fmt.Sprintf("127.0.0.1:%d", 32217+sid*100+nid*10)
		}
	}
	table["2147483647"] = map[string]string{"0": "127.0.0.1:38800"}
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return nil, err
	}
	return table, os.WriteFile(s.ipTablePath(), data, 0644)
}

var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9_\-\.]+`)

func sanitizeFilename(desc string) string {
	cleaned := safeFilenameRe.ReplaceAllString(desc, "_")
	cleaned = strings.Trim(cleaned, "_-.")
	if len(cleaned) > 60 {
		cleaned = cleaned[:60]
	}
	cleaned = strings.Trim(cleaned, "_-.")
	if cleaned == "" {
		cleaned = "config"
	}
	return cleaned
}

func (s *Service) savedConfigsDir() string {
	return filepath.Join(s.workdir, "saved_configs")
}

func (s *Service) ListSavedConfigs() ([]SavedConfigMeta, error) {
	dir := s.savedConfigsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []SavedConfigMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var sf savedConfigFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}
		result = append(result, SavedConfigMeta{
			ID:          entry.Name(),
			Description: sf.Description,
			SavedAt:     sf.SavedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].SavedAt > result[j].SavedAt
	})
	return result, nil
}

func (s *Service) SaveConfigNamed(description string, cfg WebConfig) (SavedConfigMeta, error) {
	if err := ValidateConfig(cfg); err != nil {
		return SavedConfigMeta{}, err
	}
	safe := sanitizeFilename(description)
	filename := fmt.Sprintf("%s_%d.json", safe, time.Now().Unix())
	dir := s.savedConfigsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return SavedConfigMeta{}, err
	}
	sf := savedConfigFile{
		Description: description,
		SavedAt:     time.Now().UTC().Format(time.RFC3339),
		Config:      cfg,
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return SavedConfigMeta{}, err
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return SavedConfigMeta{}, err
	}
	return SavedConfigMeta{
		ID:          filename,
		Description: description,
		SavedAt:     sf.SavedAt,
	}, nil
}

func (s *Service) LoadSavedConfig(id string) (WebConfig, error) {
	base := filepath.Base(id)
	if base != id || base == "." || base == ".." {
		return WebConfig{}, fmt.Errorf("%w: invalid id", ErrNotFound)
	}
	path := filepath.Join(s.savedConfigsDir(), base)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return WebConfig{}, fmt.Errorf("%w: saved config not found", ErrNotFound)
		}
		return WebConfig{}, err
	}
	var sf savedConfigFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return WebConfig{}, err
	}
	return sf.Config, nil
}

func (s *Service) UploadDir() string {
	return filepath.Join(s.workdir, "uploads")
}

func (s *Service) SaveUploadedFile(src io.Reader, filename string) (string, error) {
	dir := s.UploadDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	safe := filepath.Base(filename)
	if safe == "." || safe == ".." || safe == "" {
		safe = "uploaded.csv"
	}
	destPath := filepath.Join(dir, safe)
	f, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, src); err != nil {
		return "", err
	}
	return destPath, nil
}

func (s *Service) DeleteSavedConfig(id string) error {
	base := filepath.Base(id)
	if base != id || base == "." || base == ".." {
		return fmt.Errorf("%w: invalid id", ErrNotFound)
	}
	path := filepath.Join(s.savedConfigsDir(), base)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: saved config not found", ErrNotFound)
		}
		return err
	}
	return nil
}

func (s *Service) applyRuntimePaths(cfg map[string]interface{}) {
	expDir := s.experimentDir()
	setPath(cfg, expDir, "system", "log", "log_dir")
	setPath(cfg, filepath.Join(expDir, "boltdb"), "consensus_node", "blockchain", "storage", "bolt", "file_path_dir")
	setPath(cfg, filepath.Join(expDir, "trie_db"), "consensus_node", "blockchain", "storage", "eth_storage", "level_file_path_dir")
	setPath(cfg, filepath.Join(expDir, "block_record"), "consensus_node", "block_record_dir")
	setPath(cfg, filepath.Join(expDir, "results"), "supervisor", "result_output_dir")
}

func (s *Service) writeConfigSnapshot(cfg WebConfig) error {
	if err := os.MkdirAll(s.workdir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.workdir, "last_config.json"), data, 0644)
}

func setPath(root map[string]interface{}, value interface{}, keys ...string) {
	current := root
	for _, key := range keys[:len(keys)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			current[key] = next
		}
		current = next
	}
	current[keys[len(keys)-1]] = value
}

func int64FromPath(root map[string]interface{}, fallback int64, keys ...string) int64 {
	value, ok := valueAtPath(root, keys...)
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case uint64:
		return int64(v)
	default:
		return fallback
	}
}

func stringFromPath(root map[string]interface{}, fallback string, keys ...string) string {
	value, ok := valueAtPath(root, keys...)
	if !ok {
		return fallback
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fallback
}

func boolFromPath(root map[string]interface{}, fallback bool, keys ...string) bool {
	value, ok := valueAtPath(root, keys...)
	if !ok {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func valueAtPath(root map[string]interface{}, keys ...string) (interface{}, bool) {
	var current interface{} = root
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
