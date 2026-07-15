import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Activity,
  Download,
  FileText,
  FolderOpen,
  HelpCircle,
  Play,
  RefreshCw,
  Save,
  Square,
  Terminal,
  Upload,
} from 'lucide-react';
import './styles.css';

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:9091';
const CHART_API_BASE = import.meta.env.VITE_CHART_API_BASE ?? 'http://localhost:5001';

const defaultConfig = {
  system: {
    shard_num: 4,
    node_num: 4,
    limit: 5000,
    consensus_type: 'static_relay',
  },
  consensus_node: {
    block_interval: 2000,
  },
  supervisor: {
    tx_number: 100000,
    tx_injection_speed: 20000,
    epoch_duration: 50,
    tx_source_type: 'random_source',
    tx_source_file: '',
    exclude_contract_txs: true,
  },
  network: {
    bandwidth: 1000000,
    latency: 0,
  },
};

const consensusOptions = [
  ['static_relay', 'Static Relay'],
  ['static_broker', 'Static Broker'],
  ['clpa_relay', 'CLPA Relay'],
  ['clpa_broker', 'CLPA Broker'],
];

function App() {
  const [config, setConfig] = useState(defaultConfig);
  const [status, setStatus] = useState({ status: 'idle', pids: [], message: 'ready' });
  const [results, setResults] = useState({ columns: [], rows: [], files: [] });
  const [logs, setLogs] = useState('');
  const [logSource, setLogSource] = useState('all');
  const logSourceRef = useRef(logSource);
  const [logSources, setLogSources] = useState(['all']);
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [rawInputs, setRawInputs] = useState({});
  const [tick, setTick] = useState(0);
  const fileInputRef = useRef(null);
  const configFileInputRef = useRef(null);

  useEffect(() => { logSourceRef.current = logSource; }, [logSource]);

  useEffect(() => {
    refreshAll();
    const timer = setInterval(() => {
      refreshRuntime();
      refreshLogSources().catch(() => {});
      setTick((t) => t + 1);
    }, 3000);
    return () => clearInterval(timer);
  }, []);

  const modeLabel = useMemo(() => {
    if (config.system.consensus_type.includes('broker')) return 'Broker';
    return 'Relay';
  }, [config.system.consensus_type]);

  async function api(path, options = {}) {
    const res = await fetch(`${API_BASE}${path}`, {
      headers: { 'Content-Type': 'application/json' },
      ...options,
    });
    const payload = await res.json();
    if (!payload.ok) throw new Error(payload.error || 'request failed');
    return payload.data;
  }

  async function refreshAll() {
    setBusy(true);
    try {
      const cfg = await api('/api/config');
      setConfig(cfg);
      setRawInputs({});
      await refreshRuntime();
      await refreshLogSources();
      setNotice('Configuration loaded from BlockEmulator-X');
    } catch (err) {
      setNotice(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function refreshLogSources() {
    try {
      const sources = await api('/api/experiments/logs/sources');
      if (sources && sources.length > 0) {
        setLogSources(sources);
        // Reset to 'all' if current source no longer exists
        if (!sources.includes(logSourceRef.current)) {
          setLogSource('all');
        }
      }
    } catch (_) {
      // silently ignore — sources endpoint may not be available yet
    }
  }

  async function refreshRuntime() {
    try {
      const [nextStatus, nextResults, logData] = await Promise.all([
        api('/api/experiments/status'),
        api('/api/results'),
        api(`/api/experiments/logs?source=${encodeURIComponent(logSourceRef.current)}`),
      ]);
      setStatus(nextStatus);
      setResults(nextResults || { columns: [], rows: [], files: [] });
      setLogs(logData?.text || '');
    } catch (err) {
      setNotice(err.message);
    }
  }

  async function validateConfig() {
    const errors = [];
    if (config.system.shard_num <= 0) errors.push('Shard count must be greater than 0');
    if (config.system.node_num <= 0) errors.push('Nodes per shard must be greater than 0');
    if (config.system.limit <= 0) errors.push('Block size must be greater than 0');
    if (config.consensus_node.block_interval <= 0) errors.push('Block interval must be greater than 0');
    if (config.supervisor.tx_number <= 0) errors.push('Transaction count must be greater than 0');
    if (config.supervisor.tx_injection_speed <= 0) errors.push('Injection speed must be greater than 0');
    if (config.supervisor.tx_source_type === 'csv_source' && !config.supervisor.tx_source_file.trim()) {
      errors.push('CSV file path is required for CSV source');
    }
    if (errors.length > 0) throw new Error(errors.join('; '));
    await api('/api/config/validate', { method: 'POST', body: JSON.stringify(config) });
  }

  async function saveConfig() {
    setBusy(true);
    try {
      await validateConfig();
      await api('/api/config', { method: 'POST', body: JSON.stringify(config) });
      await api('/api/ip-table', { method: 'POST', body: JSON.stringify(config) });
      setNotice('Configuration and ip_table.json written to BlockEmulator-X');
    } catch (err) {
      setNotice(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function startExperiment() {
    setBusy(true);
    try {
      await validateConfig();
      const nextStatus = await api('/api/experiments/start', { method: 'POST', body: JSON.stringify(config) });
      setStatus(nextStatus);
      setNotice('Experiment started');
      await refreshRuntime();
    } catch (err) {
      setNotice(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function stopExperiment() {
    setBusy(true);
    try {
      const nextStatus = await api('/api/experiments/stop', { method: 'POST' });
      setStatus(nextStatus);
      setNotice('Experiment stopped');
      await refreshRuntime();
    } catch (err) {
      setNotice(err.message);
    } finally {
      setBusy(false);
    }
  }

  function downloadConfig() {
    const json = JSON.stringify(config, null, 2);
    const blob = new Blob([json], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `blockemulator-config.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    setNotice('Configuration downloaded');
  }

  function handleConfigFileLoad(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
      try {
        const cfg = JSON.parse(event.target.result);
        // Deep-merge with defaults so missing keys don't crash the UI
        const merged = {
          system: { ...defaultConfig.system, ...(cfg.system || {}) },
          consensus_node: { ...defaultConfig.consensus_node, ...(cfg.consensus_node || {}) },
          supervisor: { ...defaultConfig.supervisor, ...(cfg.supervisor || {}) },
          network: { ...defaultConfig.network, ...(cfg.network || {}) },
        };
        setConfig(merged);
        setRawInputs({});
        setNotice('Configuration loaded from ' + file.name);
      } catch (err) {
        setNotice('Failed to parse config file: ' + err.message);
      } finally {
        if (configFileInputRef.current) configFileInputRef.current.value = '';
      }
    };
    reader.readAsText(file);
  }

  async function handleFileUpload(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    setBusy(true);
    try {
      const formData = new FormData();
      formData.append('file', file);
      const res = await fetch(`${API_BASE}/api/upload/tx-source`, {
        method: 'POST',
        body: formData,
      });
      const payload = await res.json();
      if (!payload.ok) throw new Error(payload.error || 'upload failed');
      update('supervisor.tx_source_file', payload.data.path);
      setNotice('File uploaded: ' + file.name);
    } catch (err) {
      setNotice(err.message);
    } finally {
      setBusy(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }

  function update(path, rawValue) {
    // Keep raw input for display
    setRawInputs((prev) => ({ ...prev, [path]: rawValue }));
    // Parse sci notation for config value
    const value = typeof rawValue === 'string' && /^-?\d+(\.\d+)?(e[+-]?\d+)?$/i.test(rawValue) ? Number(rawValue) : rawValue;
    setConfig((prev) => {
      const next = structuredClone(prev);
      const keys = path.split('.');
      let cursor = next;
      for (const key of keys.slice(0, -1)) cursor = cursor[key];
      cursor[keys.at(-1)] = value;
      return next;
    });
  }

  return (
    <main className="app-shell">
      <aside className="config-panel">
        <div className="brand">
          <div>
            <p className="eyebrow">Local Console</p>
            <h1>BlockEmulator-X</h1>
          </div>
          <Activity size={28} />
        </div>

        <section className="form-section">
          <SectionTitle title="Config File" />
          <div style={{ display: 'flex', gap: 8, marginTop: 4 }}>
            <button type="button" className="icon-button" onClick={downloadConfig} disabled={busy}>
              <Save size={16} /> <span>Save Config</span>
            </button>
            <button type="button" className="icon-button" onClick={() => configFileInputRef.current?.click()} disabled={busy}>
              <FolderOpen size={16} /> <span>Load Config</span>
            </button>
            <input ref={configFileInputRef} type="file" accept=".json" style={{ display: 'none' }} onChange={handleConfigFileLoad} />
          </div>
        </section>

        <section className="form-section">
          <SectionTitle title="Chain" />
          <NumberField
            label="Shard Count"
            value={config.system.shard_num}
            onChange={(v) => update('system.shard_num', v)}
            path="system.shard_num"
            rawInputs={rawInputs}
            help="The number of shards in the blockchain system. Each shard operates as an independent sub-blockchain with its own set of nodes. Must be a positive integer."
          />
          <NumberField
            label="Nodes per Shard"
            value={config.system.node_num}
            onChange={(v) => update('system.node_num', v)}
            path="system.node_num"
            rawInputs={rawInputs}
            help="The number of consensus nodes per shard. More nodes improve decentralization and fault tolerance but may reduce overall throughput. Must be a positive integer."
          />
          <NumberField
            label="Block Tx Limit"
            value={config.system.limit}
            onChange={(v) => update('system.limit', v)}
            path="system.limit"
            rawInputs={rawInputs}
            help="Maximum number of transactions per block. This limits the block size to control propagation time and resource usage across the network."
          />
          <label className="field">
            <span>
              Consensus Type
              <HelpIcon text={"Cross-shard transaction handling mode:\n\n• Static Relay — Accounts remain in their original shards; cross-shard txs handled by relay nodes.\n• Static Broker — Accounts remain in their original shards; cross-shard txs handled by dedicated broker accounts.\n• CLPA Relay — Accounts are dynamically migrated across shards by CLPA at each epoch; cross-shard txs handled by relay nodes.\n• CLPA Broker — Accounts are dynamically migrated by CLPA; cross-shard txs handled by broker accounts."} />
            </span>
            <select value={config.system.consensus_type} onChange={(e) => update('system.consensus_type', e.target.value)}>
              {consensusOptions.map(([value, label]) => (
                <option key={value} value={value}>{label}</option>
              ))}
            </select>
          </label>
        </section>

        <section className="form-section">
          <SectionTitle title="Consensus Node" />
          <NumberField
            label="Block Interval (ms)"
            value={config.consensus_node.block_interval}
            onChange={(v) => update('consensus_node.block_interval', v)}
            path="consensus_node.block_interval"
            rawInputs={rawInputs}
            help="Time interval between two consecutive blocks, in milliseconds. Lower values increase transaction throughput but may lead to more forks and higher computational overhead."
          />
        </section>

        <section className="form-section">
          <SectionTitle title="Supervisor" />
          <NumberField
            label="Total Transactions"
            value={config.supervisor.tx_number}
            onChange={(v) => update('supervisor.tx_number', v)}
            path="supervisor.tx_number"
            rawInputs={rawInputs}
            help="Total number of transactions the supervisor will inject into the system during the experiment run."
          />
          <NumberField
            label="Injection Speed (tx/s)"
            value={config.supervisor.tx_injection_speed}
            onChange={(v) => update('supervisor.tx_injection_speed', v)}
            path="supervisor.tx_injection_speed"
            rawInputs={rawInputs}
            help="Transaction injection rate in transactions per second (tx/s). The supervisor injects transactions at this constant rate into the blockchain network."
          />
          <NumberField
            label="Reconfiguration Interval (s)"
            value={config.supervisor.epoch_duration}
            onChange={(v) => update('supervisor.epoch_duration', v)}
            path="supervisor.epoch_duration"
            rawInputs={rawInputs}
            help="Duration of one epoch in seconds. At the end of each epoch, performance metrics are recorded and CLPA may migrate accounts between shards to rebalance load."
          />
          <label className="field">
            <span>
              Transaction Source
              <HelpIcon text="Source of injected transactions:\n\n• Random Source — Supervisor generates random transactions automatically.\n• CSV Source — Supervisor reads transactions from a specified CSV file." />
            </span>

            <select value={config.supervisor.tx_source_type} onChange={(e) => update('supervisor.tx_source_type', e.target.value)}>
              <option value="random_source">Random Source</option>
              <option value="csv_source">CSV Source</option>
            </select>
          </label>
          <label className="field">
            <span>
              CSV Path
              <HelpIcon text="File path to the CSV file containing pre-generated transactions. This field is only used when the transaction source type is set to 'CSV Source'." />
            </span>
            <div className="csv-path-row">
              <input value={config.supervisor.tx_source_file} onChange={(e) => update('supervisor.tx_source_file', e.target.value)} placeholder="./data/txs.csv" />
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv"
                className="csv-file-input"
                onChange={handleFileUpload}
              />
              <button
                type="button"
                className="icon-button"
                onClick={() => fileInputRef.current?.click()}
                disabled={busy}
                title="Upload CSV from local machine"
              >
                <Upload size={16} />
              </button>
            </div>
          </label>
          <label className="check-field">
            <input
              type="checkbox"
              checked={Boolean(config.supervisor.exclude_contract_txs)}
              onChange={(e) => update('supervisor.exclude_contract_txs', e.target.checked)}
            />
            <span>Exclude contract transactions</span>
            <HelpIcon side="left" text="When enabled, smart contract-related transactions are filtered out when reading from a CSV source. Useful for benchmarking pure transfer workloads without smart contract overhead." />
          </label>
        </section>

        <section className="form-section">
          <SectionTitle title="Network" />
          <NumberField
            label="Bandwidth"
            value={config.network.bandwidth}
            onChange={(v) => update('network.bandwidth', v)}
            path="network.bandwidth"
            rawInputs={rawInputs}
            help="Network bandwidth limit for inter-node communication. Controls the maximum data transfer rate between consensus nodes in the blockchain network."
          />
          <NumberField
            label="Latency (ms)"
            value={config.network.latency}
            onChange={(v) => update('network.latency', v)}
            path="network.latency"
            rawInputs={rawInputs}
            help="Artificial network latency added to all inter-node messages, in milliseconds. Use this to simulate real-world network conditions such as WAN delays."
          />
        </section>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">Experiment</p>
            <h2>{config.system.shard_num} shards x {config.system.node_num} nodes · {modeLabel}</h2>
          </div>
          <div className="actions">
            <IconButton icon={<RefreshCw size={17} />} label="Refresh" onClick={refreshAll} disabled={busy} />
            <IconButton icon={<Play size={17} />} label="Start" onClick={startExperiment} disabled={busy || status.status === 'running'} primary />
            <IconButton icon={<Square size={17} />} label="Stop" onClick={stopExperiment} disabled={busy || status.status !== 'running'} danger />
          </div>
        </header>

        <div className="status-grid">
          <StatusCard label="Status" value={status.status || 'idle'} tone={status.status} />
          <StatusCard label="Processes" value={status.pids?.length || 0} />
          <StatusCard label="Result Files" value={results.files?.length || 0} />
          <StatusCard label="Message" value={status.message || notice || 'ready'} wide />
        </div>

        {notice && <div className="notice">{notice}</div>}

        <section className="charts">
          <p className="chart-disclaimer">Charts shown here are for reference only. Please generate your own charts for specific experiment analysis.</p>
          <ChartCard
            title="TPS"
            subtitle="Transactions Per Second"
            src={`${CHART_API_BASE}/api/charts/tps?type=line&v=${tick}`}
            fallbackSrc={`${CHART_API_BASE}/api/charts/tps?type=bar&v=${tick}`}
            hasData={(results.rows || []).length > 0}
          />
          <ChartCard
            title="CTX Ratio"
            subtitle="Cross-Shard Transaction Ratio"
            src={`${CHART_API_BASE}/api/charts/ctx_ratio?type=line&v=${tick}`}
            fallbackSrc={`${CHART_API_BASE}/api/charts/ctx_ratio?type=bar&v=${tick}`}
            hasData={(results.rows || []).length > 0}
          />
          <ChartCard
            title="TCL"
            subtitle="Transaction Confirmation Latency"
            src={`${CHART_API_BASE}/api/charts/tcl?type=line&v=${tick}`}
            fallbackSrc={`${CHART_API_BASE}/api/charts/tcl?type=bar&v=${tick}`}
            hasData={(results.rows || []).length > 0}
          />
        </section>

        <section className="data-section">
          <div className="section-head">
            <div>
              <p className="eyebrow">Results</p>
              <h3>{results.brief_file || 'No results yet'}</h3>
            </div>
            {results.brief_file && (
              <a className="download" href={`${API_BASE}${results.download_url}`}>
                <Download size={16} />
                CSV
              </a>
            )}
          </div>
          <ResultsTable columns={results.columns || []} rows={results.rows || []} />
        </section>

        <section className="log-section">
          <div className="section-head">
            <div>
              <p className="eyebrow">Runtime</p>
              <h3>Logs</h3>
            </div>
            <div className="log-controls">
              <select
                className="log-source-select"
                value={logSource}
                onChange={(e) => setLogSource(e.target.value)}
              >
                {logSources.map((src) => (
                  <option key={src} value={src}>
                    {src === 'all' ? 'All Nodes' : src === 'supervisor' ? 'Supervisor' : src.replace('_', ' ')}
                  </option>
                ))}
              </select>
              <Terminal size={18} />
            </div>
          </div>
          <pre>{logs || 'No logs yet.'}</pre>
        </section>
      </section>

    </main>
  );
}

function SectionTitle({ title }) {
  return (
    <div className="section-title">
      <FileText size={16} />
      <span>{title}</span>
    </div>
  );
}

function HelpIcon({ text, side }) {
  return (
    <span className="help-icon-wrap">
      <HelpCircle size={14} className="help-icon" />
      <span className={`tooltip${side === 'left' ? ' tooltip-left' : ''}`}>{text}</span>
    </span>
  );
}

function NumberField({ label, value, onChange, help, path, rawInputs }) {
  const displayValue = rawInputs?.[path] ?? value;
  return (
    <label className="field">
      <span>
        {label}
        {help ? <HelpIcon text={help} /> : null}
      </span>
      <input type="text" inputMode="decimal" value={displayValue} onChange={(e) => onChange(e.target.value)} />
    </label>
  );
}

function IconButton({ icon, label, onClick, disabled, primary, danger }) {
  return (
    <button className={`icon-button ${primary ? 'primary' : ''} ${danger ? 'danger' : ''}`} onClick={onClick} disabled={disabled} title={label}>
      {icon}
      <span>{label}</span>
    </button>
  );
}

function StatusCard({ label, value, tone, wide }) {
  return (
    <div className={`status-card ${wide ? 'wide' : ''} ${tone || ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function ChartCard({ title, subtitle, src, fallbackSrc, hasData }) {
  const [chartType, setChartType] = useState('line');
  const [retryCount, setRetryCount] = useState(0);

  const currentSrc = chartType === 'line'
    ? src
    : (fallbackSrc || src.replace('type=line', 'type=bar'));

  const handleError = useCallback(() => {
    if (retryCount === 0) {
      // first error: try switching to bar chart before giving up
      setChartType('bar');
      setRetryCount(1);
    }
    // after bar also fails, stay at broken-image state (user can toggle manually)
  }, [retryCount]);

  const switchTo = (type) => {
    setChartType(type);
    setRetryCount(0);
  };

  if (!hasData) {
    return (
      <article className="metric chart-card">
        <div className="metric-head">
          <div className="metric-head-left">
            <span>{title}</span>
            <small>{subtitle}</small>
          </div>
        </div>
        <div className="chart-img-wrap">
          <div className="empty-chart">Waiting for experiment data...</div>
        </div>
      </article>
    );
  }

  return (
    <article className="metric chart-card">
      <div className="metric-head">
        <div className="metric-head-left">
          <span>{title}</span>
          <small>{subtitle}</small>
        </div>
        <div className="chart-toggle">
          <button
            className={`toggle-btn ${chartType === 'line' ? 'active' : ''}`}
            onClick={() => switchTo('line')}
            title="Line Chart"
          >
            📈
          </button>
          <button
            className={`toggle-btn ${chartType === 'bar' ? 'active' : ''}`}
            onClick={() => switchTo('bar')}
            title="Bar Chart"
          >
            📊
          </button>
        </div>
      </div>
      <div className="chart-img-wrap">
        <img
          key={`${chartType}-${retryCount}`}
          src={currentSrc}
          alt={`${title} — ${chartType} chart`}
          onError={handleError}
        />
      </div>
    </article>
  );
}

function ResultsTable({ columns, rows }) {
  if (!columns.length || !rows.length) {
    return <div className="empty">After the experiment completes, epoch metrics from the brief CSV will be displayed here.</div>;
  }
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {columns.map((column) => <th key={column}>{column}</th>)}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, idx) => (
            <tr key={idx}>
              {columns.map((column) => <td key={column}>{row[column] || ''}</td>)}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

createRoot(document.getElementById('root')).render(<App />);
