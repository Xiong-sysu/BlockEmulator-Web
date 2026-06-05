package emulator

import "time"

type WebConfig struct {
	System        SystemConfig        `json:"system"`
	ConsensusNode ConsensusNodeConfig `json:"consensus_node"`
	Supervisor    SupervisorConfig    `json:"supervisor"`
	Network       NetworkConfig       `json:"network"`
}

type SystemConfig struct {
	ShardNum      int64  `json:"shard_num"`
	NodeNum       int64  `json:"node_num"`
	Limit         int64  `json:"limit"`
	ConsensusType string `json:"consensus_type"`
}

type ConsensusNodeConfig struct {
	BlockInterval int64 `json:"block_interval"`
}

type SupervisorConfig struct {
	TxNumber           int64  `json:"tx_number"`
	TxInjectionSpeed   int64  `json:"tx_injection_speed"`
	EpochDuration      int64  `json:"epoch_duration"`
	TxSourceType       string `json:"tx_source_type"`
	TxSourceFile       string `json:"tx_source_file"`
	ExcludeContractTxs bool   `json:"exclude_contract_txs"`
}

type NetworkConfig struct {
	Bandwidth int64 `json:"bandwidth"`
	Latency   int64 `json:"latency"`
}

type ExperimentStatus struct {
	Status     string    `json:"status"`
	StartedAt  string    `json:"started_at"`
	FinishedAt string    `json:"finished_at"`
	PIDs       []int     `json:"pids"`
	Message    string    `json:"message"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Results struct {
	Mode        string              `json:"mode"`
	BriefFile   string              `json:"brief_file"`
	DetailFile  string              `json:"detail_file"`
	Columns     []string            `json:"columns"`
	Rows        []map[string]string `json:"rows"`
	Files       []string            `json:"files"`
	DownloadURL string              `json:"download_url"`
}
