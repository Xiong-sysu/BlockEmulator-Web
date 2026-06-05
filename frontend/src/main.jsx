import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Activity,
  BarChart3,
  Download,
  FileText,
  Play,
  RefreshCw,
  Save,
  Square,
  Terminal,
} from 'lucide-react';
import './styles.css';

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080';
const CHART_API_BASE = import.meta.env.VITE_CHART_API_BASE ?? 'http://localhost:5000';

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
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const [tick, setTick] = useState(0);

  useEffect(() => {
    refreshAll();
    const timer = setInterval(() => {
      refreshRuntime();
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
      await refreshRuntime();
      setNotice('配置已从 BlockEmulator-X 读取');
    } catch (err) {
      setNotice(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function refreshRuntime() {
    try {
      const [nextStatus, nextResults, logData] = await Promise.all([
        api('/api/experiments/status'),
        api('/api/results'),
        api('/api/experiments/logs'),
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
    if (config.system.shard_num <= 0) errors.push('分片数必须大于 0');
    if (config.system.node_num <= 0) errors.push('每片节点数必须大于 0');
    if (config.system.limit <= 0) errors.push('区块大小必须大于 0');
    if (config.consensus_node.block_interval <= 0) errors.push('区块间隔必须大于 0');
    if (config.supervisor.tx_number <= 0) errors.push('交易总数必须大于 0');
    if (config.supervisor.tx_injection_speed <= 0) errors.push('注入速度必须大于 0');
    if (config.supervisor.tx_source_type === 'csv_source' && !config.supervisor.tx_source_file.trim()) {
      errors.push('CSV 交易源需要填写文件路径');
    }
    if (errors.length > 0) throw new Error(errors.join('；'));
    await api('/api/config/validate', { method: 'POST', body: JSON.stringify(config) });
  }

  async function saveConfig() {
    setBusy(true);
    try {
      await validateConfig();
      await api('/api/config', { method: 'POST', body: JSON.stringify(config) });
      await api('/api/ip-table', { method: 'POST', body: JSON.stringify(config) });
      setNotice('配置和 ip_table.json 已写入 BlockEmulator-X');
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
      setNotice('实验已启动');
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
      setNotice('实验已停止');
      await refreshRuntime();
    } catch (err) {
      setNotice(err.message);
    } finally {
      setBusy(false);
    }
  }

  function update(path, rawValue) {
    const value = typeof rawValue === 'string' && /^-?\d+$/.test(rawValue) ? Number(rawValue) : rawValue;
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
          <SectionTitle title="System" />
          <NumberField label="分片数" value={config.system.shard_num} onChange={(v) => update('system.shard_num', v)} />
          <NumberField label="每片节点数" value={config.system.node_num} onChange={(v) => update('system.node_num', v)} />
          <NumberField label="区块交易上限" value={config.system.limit} onChange={(v) => update('system.limit', v)} />
          <label className="field">
            <span>共识类型</span>
            <select value={config.system.consensus_type} onChange={(e) => update('system.consensus_type', e.target.value)}>
              {consensusOptions.map(([value, label]) => (
                <option key={value} value={value}>{label}</option>
              ))}
            </select>
          </label>
        </section>

        <section className="form-section">
          <SectionTitle title="Consensus Node" />
          <NumberField label="区块间隔 ms" value={config.consensus_node.block_interval} onChange={(v) => update('consensus_node.block_interval', v)} />
        </section>

        <section className="form-section">
          <SectionTitle title="Supervisor" />
          <NumberField label="交易总数" value={config.supervisor.tx_number} onChange={(v) => update('supervisor.tx_number', v)} />
          <NumberField label="注入速度 tx/s" value={config.supervisor.tx_injection_speed} onChange={(v) => update('supervisor.tx_injection_speed', v)} />
          <NumberField label="Epoch 秒" value={config.supervisor.epoch_duration} onChange={(v) => update('supervisor.epoch_duration', v)} />
          <label className="field">
            <span>交易来源</span>
            <select value={config.supervisor.tx_source_type} onChange={(e) => update('supervisor.tx_source_type', e.target.value)}>
              <option value="random_source">Random Source</option>
              <option value="csv_source">CSV Source</option>
            </select>
          </label>
          <label className="field">
            <span>CSV 路径</span>
            <input value={config.supervisor.tx_source_file} onChange={(e) => update('supervisor.tx_source_file', e.target.value)} placeholder="./data/txs.csv" />
          </label>
          <label className="check-field">
            <input
              type="checkbox"
              checked={Boolean(config.supervisor.exclude_contract_txs)}
              onChange={(e) => update('supervisor.exclude_contract_txs', e.target.checked)}
            />
            <span>Exclude contract transactions</span>
          </label>
        </section>

        <section className="form-section">
          <SectionTitle title="Network" />
          <NumberField label="带宽" value={config.network.bandwidth} onChange={(v) => update('network.bandwidth', v)} />
          <NumberField label="延迟 ms" value={config.network.latency} onChange={(v) => update('network.latency', v)} />
        </section>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">Experiment</p>
            <h2>{config.system.shard_num} shards x {config.system.node_num} nodes · {modeLabel}</h2>
          </div>
          <div className="actions">
            <IconButton icon={<RefreshCw size={17} />} label="刷新" onClick={refreshAll} disabled={busy} />
            <IconButton icon={<Save size={17} />} label="保存配置" onClick={saveConfig} disabled={busy} />
            <IconButton icon={<Play size={17} />} label="启动" onClick={startExperiment} disabled={busy || status.status === 'running'} primary />
            <IconButton icon={<Square size={17} />} label="停止" onClick={stopExperiment} disabled={busy || status.status !== 'running'} danger />
          </div>
        </header>

        <div className="status-grid">
          <StatusCard label="状态" value={status.status || 'idle'} tone={status.status} />
          <StatusCard label="进程数" value={status.pids?.length || 0} />
          <StatusCard label="结果文件" value={results.files?.length || 0} />
          <StatusCard label="消息" value={status.message || notice || 'ready'} wide />
        </div>

        {notice && <div className="notice">{notice}</div>}

        <section className="charts">
          <ChartCard
            title="TPS"
            subtitle="Transactions Per Second"
            icon={<BarChart3 size={18} />}
            src={`${CHART_API_BASE}/api/charts/tps?type=line&v=${tick}`}
            fallbackSrc={`${CHART_API_BASE}/api/charts/tps?type=bar&v=${tick}`}
            hasData={(results.rows || []).length > 0}
          />
          <ChartCard
            title="CTX Ratio"
            subtitle="Cross-Shard Transaction Ratio"
            icon={<BarChart3 size={18} />}
            src={`${CHART_API_BASE}/api/charts/ctx_ratio?type=line&v=${tick}`}
            fallbackSrc={`${CHART_API_BASE}/api/charts/ctx_ratio?type=bar&v=${tick}`}
            hasData={(results.rows || []).length > 0}
          />
          <ChartCard
            title="TCL"
            subtitle="Transaction Confirmation Latency"
            icon={<BarChart3 size={18} />}
            src={`${CHART_API_BASE}/api/charts/tcl?type=line&v=${tick}`}
            fallbackSrc={`${CHART_API_BASE}/api/charts/tcl?type=bar&v=${tick}`}
            hasData={(results.rows || []).length > 0}
          />
        </section>

        <section className="data-section">
          <div className="section-head">
            <div>
              <p className="eyebrow">Results</p>
              <h3>{results.brief_file || '暂无结果'}</h3>
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
              <h3>日志</h3>
            </div>
            <Terminal size={18} />
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

function NumberField({ label, value, onChange }) {
  return (
    <label className="field">
      <span>{label}</span>
      <input type="number" value={value} onChange={(e) => onChange(e.target.value)} />
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

function ChartCard({ title, subtitle, icon, src, fallbackSrc, hasData }) {
  const [chartType, setChartType] = useState('line');
  const [imgError, setImgError] = useState(false);

  const currentSrc = chartType === 'line'
    ? src
    : (fallbackSrc || src.replace('type=line', 'type=bar'));

  const handleError = useCallback(() => {
    if (!imgError && chartType === 'line') {
      // try bar chart on error
      setChartType('bar');
      setImgError(true);
    }
  }, [imgError, chartType]);

  return (
    <article className="metric chart-card">
      <div className="metric-head">
        <div className="metric-head-left">
          <span>{icon}{title}</span>
          <small>{subtitle}</small>
        </div>
        <div className="chart-toggle">
          <button
            className={`toggle-btn ${chartType === 'line' ? 'active' : ''}`}
            onClick={() => { setChartType('line'); setImgError(false); }}
            title="折线图"
          >
            📈
          </button>
          <button
            className={`toggle-btn ${chartType === 'bar' ? 'active' : ''}`}
            onClick={() => { setChartType('bar'); setImgError(false); }}
            title="柱状图"
          >
            📊
          </button>
        </div>
      </div>
      <div className="chart-img-wrap">
        {hasData ? (
          <img
            src={currentSrc}
            alt={`${title} ${chartType} chart`}
            onError={handleError}
          />
        ) : (
          <div className="empty-chart">等待实验数据...</div>
        )}
      </div>
    </article>
  );
}

function ResultsTable({ columns, rows }) {
  if (!columns.length || !rows.length) {
    return <div className="empty">实验结束后，这里会展示 brief CSV 的 epoch 指标。</div>;
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

function formatNumber(value) {
  if (Math.abs(value) >= 1000000) return value.toExponential(2);
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 }).format(value);
}

createRoot(document.getElementById('root')).render(<App />);
