// Package dashboard provides an optional HTTP monitoring dashboard for jig-mcp,
// exposing registered tools, execution metrics, and recent audit log entries.
package dashboard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/snipspin/jig-mcp/internal/audit"
	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// toolMetrics holds running counters for a single tool's invocations.
type toolMetrics struct {
	Count   int   `json:"count"`
	TotalMS int64 `json:"total_ms"`
}

var (
	metricsMu    sync.RWMutex
	metricsStore = make(map[string]*toolMetrics)
)

// RecordMetric updates the in-memory metrics for a tool after execution.
func RecordMetric(toolName string, durationMS int64) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	m, ok := metricsStore[toolName]
	if !ok {
		m = &toolMetrics{}
		metricsStore[toolName] = m
	}
	m.Count++
	m.TotalMS += durationMS
}

// getMetrics returns a snapshot of the current in-memory metrics.
func getMetrics() map[string]map[string]any {
	metricsMu.RLock()
	defer metricsMu.RUnlock()
	res := make(map[string]map[string]any, len(metricsStore))
	for tool, m := range metricsStore {
		avg := 0.0
		if m.Count > 0 {
			avg = float64(m.TotalMS) / float64(m.Count)
		}
		res[tool] = map[string]any{
			"count":           m.Count,
			"avg_duration_ms": avg,
		}
	}
	return res
}

// seedMetricsFromLog populates the in-memory metrics store by scanning the
// existing audit log file. Called once at startup so metrics survive restarts.
func SeedMetricsFromLog() {
	logDir := audit.GetLogDir()
	if strings.Contains(logDir, "..") {
		return
	}
	root, err := os.OpenRoot(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		slog.Error("failed to open audit log directory root", "err", err)
		return
	}
	defer root.Close()

	f, err := root.Open("audit.jsonl")
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		slog.Error("failed to seed metrics from audit log", "err", err)
		return
	}
	defer f.Close()

	metricsMu.Lock()
	defer metricsMu.Unlock()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var line auditLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err == nil {
			m, ok := metricsStore[line.Tool]
			if !ok {
				m = &toolMetrics{}
				metricsStore[line.Tool] = m
			}
			m.Count++
			m.TotalMS += line.DurationMS
		}
	}
}

// authenticate wraps an http.HandlerFunc with token authentication.
func authenticate(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tr := auth.GlobalTokens()
		if !tr.AuthRequired() {
			// No tokens configured — pass through with anonymous identity.
			h(w, r)
			return
		}

		// Extract candidate token from Authorization header or query parameter.
		var candidate string
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				candidate = parts[1]
			}
		}
		if candidate == "" {
			candidate = r.URL.Query().Get("token")
		}

		if _, ok := tr.Lookup(candidate); ok {
			h(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", `Bearer realm="jig-mcp"`)
		http.Error(w, "Unauthorized: missing or invalid token", http.StatusUnauthorized)
	}
}

const dashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Jig-MCP Dashboard</title>
    <style>
        :root {
            --bg-color: #0f172a;
            --card-bg: rgba(30, 41, 59, 0.7);
            --text-color: #f8fafc;
            --accent-color: #38bdf8;
            --success-color: #4ade80;
            --error-color: #f87171;
            --border-color: rgba(255, 255, 255, 0.1);
        }

        body {
            font-family: 'Inter', system-ui, -apple-system, sans-serif;
            background-color: var(--bg-color);
            color: var(--text-color);
            margin: 0;
            padding: 2rem;
            line-height: 1.5;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 2rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid var(--border-color);
        }

        h1 {
            font-size: 1.875rem;
            font-weight: 700;
            background: linear-gradient(to right, #38bdf8, #818cf8);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .dashboard-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }

        .card {
            background: var(--card-bg);
            backdrop-filter: blur(12px);
            border: 1px solid var(--border-color);
            border-radius: 1rem;
            padding: 1.5rem;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
        }

        .card h2 {
            margin-top: 0;
            font-size: 1.25rem;
            color: var(--accent-color);
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 0.5rem;
        }

        table {
            width: 100%;
            border-collapse: collapse;
            font-size: 0.875rem;
        }

        th, td {
            text-align: left;
            padding: 0.75rem;
            border-bottom: 1px solid var(--border-color);
        }

        th {
            color: var(--accent-color);
            font-weight: 600;
        }

        .status-success { color: var(--success-color); }
        .status-error { color: var(--error-color); }

        .tool-list {
            list-style: none;
            padding: 0;
        }

        .tool-item {
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid var(--border-color);
        }

        .platform-tag {
            display: inline-block;
            background: rgba(56, 189, 248, 0.2);
            padding: 0.125rem 0.5rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            margin-right: 0.25rem;
        }

        .refresh-btn {
            background: var(--accent-color);
            color: var(--bg-color);
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 0.5rem;
            font-weight: 600;
            cursor: pointer;
            transition: opacity 0.2s;
        }

        .refresh-btn:hover { opacity: 0.9; }

        .metric-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--success-color);
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Jig-MCP Dashboard</h1>
            <button class="refresh-btn" onclick="refreshData()">Refresh Data</button>
        </header>

        <div class="dashboard-grid">
            <div class="card">
                <h2>Active Tools</h2>
                <div id="tools-container">Loading...</div>
            </div>
            <div class="card">
                <h2>Performance Metrics</h2>
                <div id="metrics-container">Loading...</div>
            </div>
        </div>

        <div class="card">
            <h2>Recent Audit Log (Last 100)</h2>
            <div style="overflow-x: auto;">
                <table id="logs-table">
                    <thead>
                        <tr>
                            <th>Timestamp</th>
                            <th>Tool</th>
                            <th>Duration (ms)</th>
                            <th>Status</th>
                            <th>Error</th>
                        </tr>
                    </thead>
                    <tbody id="logs-body">
                        <tr><td colspan="5">Loading...</td></tr>
                    </tbody>
                </table>
            </div>
        </div>
    </div>

    <script>
        async function fetchData(endpoint) {
            const resp = await fetch('/api/' + endpoint);
            return resp.json();
        }

        async function refreshData() {
            try {
                const tools = await fetchData('tools');
                const logs = await fetchData('logs');
                const metrics = await fetchData('metrics');

                renderTools(tools);
                renderLogs(logs);
                renderMetrics(metrics);
            } catch (err) {
                console.error('Failed to fetch data:', err);
            }
        }

        function renderTools(tools) {
            const container = document.getElementById('tools-container');
            container.textContent = '';
            const list = document.createElement('ul');
            list.className = 'tool-list';

            tools.forEach(tool => {
                const li = document.createElement('li');
                li.className = 'tool-item';

                const nameDiv = document.createElement('div');
                nameDiv.style.fontWeight = '600';
                nameDiv.textContent = tool.name;
                li.appendChild(nameDiv);

                const descDiv = document.createElement('div');
                descDiv.style.fontSize = '0.8rem';
                descDiv.style.color = '#94a3b8';
                descDiv.style.margin = '0.25rem 0';
                descDiv.textContent = tool.description;
                li.appendChild(descDiv);

                const platDiv = document.createElement('div');
                const platforms = tool.platforms || {};
                Object.keys(platforms).forEach(p => {
                    const span = document.createElement('span');
                    span.className = 'platform-tag';
                    span.textContent = p;
                    platDiv.appendChild(span);
                });
                li.appendChild(platDiv);

                list.appendChild(li);
            });
            container.appendChild(list);
        }

        function renderMetrics(data) {
            const container = document.getElementById('metrics-container');
            container.textContent = '';

            // Show concurrency info
            if (data.concurrency) {
                const c = data.concurrency;
                const concDiv = document.createElement('div');
                concDiv.style.marginBottom = '1.5rem';
                concDiv.style.fontSize = '0.85rem';
                concDiv.style.color = '#94a3b8';
                concDiv.textContent = 'Slots: ' + c.in_use + '/' + c.max_concurrent_tools + ' in use';
                container.appendChild(concDiv);
            }

            const metrics = data.tools || {};
            for (const [tool, m] of Object.entries(metrics)) {
                const div = document.createElement('div');
                div.style.marginBottom = '1rem';

                const title = document.createElement('div');
                title.style.fontSize = '0.9rem';
                title.style.fontWeight = '600';
                title.textContent = tool;
                div.appendChild(title);

                const row = document.createElement('div');
                row.style.display = 'flex';
                row.style.gap = '1rem';
                row.style.alignItems = 'baseline';

                const countDiv = document.createElement('div');
                countDiv.textContent = 'Calls: ';
                const countSpan = document.createElement('span');
                countSpan.className = 'metric-value';
                countSpan.textContent = m.count;
                countDiv.appendChild(countSpan);
                row.appendChild(countDiv);

                const avgDiv = document.createElement('div');
                avgDiv.textContent = 'Avg: ';
                const avgSpan = document.createElement('span');
                avgSpan.className = 'metric-value';
                avgSpan.style.color = 'var(--accent-color)';
                avgSpan.textContent = m.avg_duration_ms.toFixed(1) + 'ms';
                avgDiv.appendChild(avgSpan);
                row.appendChild(avgDiv);

                div.appendChild(row);
                container.appendChild(div);
            }
            if (Object.keys(data.tools || {}).length === 0) {
                container.textContent = 'No data available yet.';
            }
        }

        function renderLogs(logs) {
            const body = document.getElementById('logs-body');
            body.textContent = '';
            logs.forEach(log => {
                const tr = document.createElement('tr');

                const tdTs = document.createElement('td');
                tdTs.textContent = new Date(log.timestamp).toLocaleString();
                tr.appendChild(tdTs);

                const tdTool = document.createElement('td');
                tdTool.style.fontWeight = '600';
                tdTool.textContent = log.tool;
                tr.appendChild(tdTool);

                const tdDur = document.createElement('td');
                tdDur.textContent = log.duration_ms;
                tr.appendChild(tdDur);

                const tdStatus = document.createElement('td');
                tdStatus.className = log.success ? 'status-success' : 'status-error';
                tdStatus.textContent = log.success ? 'SUCCESS' : 'FAILURE';
                tr.appendChild(tdStatus);

                const tdErr = document.createElement('td');
                tdErr.style.fontSize = '0.75rem';
                tdErr.style.maxWidth = '300px';
                tdErr.style.overflow = 'hidden';
                tdErr.style.textOverflow = 'ellipsis';
                tdErr.style.whiteSpace = 'nowrap';
                tdErr.title = log.error || '';
                tdErr.textContent = log.error || '-';
                tr.appendChild(tdErr);

                body.appendChild(tr);
            });
            if (logs.length === 0) {
                const tr = document.createElement('tr');
                const td = document.createElement('td');
                td.colSpan = 5;
                td.textContent = 'No log entries found.';
                tr.appendChild(td);
                body.appendChild(tr);
            }
        }

        refreshData();
        setInterval(refreshData, 10000); // Auto refresh every 10s
    </script>
</body>
</html>
`

type auditLine struct {
	Timestamp  string         `json:"timestamp"`
	Tool       string         `json:"tool"`
	Arguments  map[string]any `json:"arguments"`
	DurationMS int64          `json:"duration_ms"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
}

func getAuditLogs(limit int) ([]auditLine, error) {
	logDir := audit.GetLogDir()
	if strings.Contains(logDir, "..") {
		return nil, os.ErrInvalid
	}
	root, err := os.OpenRoot(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []auditLine{}, nil
		}
		return nil, err
	}
	defer root.Close()

	f, err := root.Open("audit.jsonl")
	if err != nil {
		if os.IsNotExist(err) {
			return []auditLine{}, nil
		}
		return nil, err
	}
	defer f.Close()

	// Stat the file to get its size
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	// Seek to tail: read at most 64KB from the end of the file.
	// This bounds memory usage regardless of file size.
	const tailSize int64 = 64 * 1024 // 64KB
	seekPos := fileSize - tailSize
	if seekPos < 0 {
		seekPos = 0
	}

	_, err = f.Seek(seekPos, 0)
	if err != nil {
		return nil, err
	}

	var lines []auditLine
	scanner := bufio.NewScanner(f)
	firstLine := true

	for scanner.Scan() {
		// If we seeked to the middle of the file, skip the first partial line
		if seekPos > 0 && firstLine {
			firstLine = false
			continue
		}
		firstLine = false

		var line auditLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err == nil {
			lines = append(lines, line)
		}
	}

	// Reverse to get recent first
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].Timestamp > lines[j].Timestamp
	})

	if len(lines) > limit {
		lines = lines[:limit]
	}
	return lines, nil
}

// StartDashboard starts an HTTP dashboard server.
func StartDashboard(port int, registry *tools.Registry, semaphore chan struct{}, semaphoreSize int) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", authenticate(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(dashboardHTML)); err != nil {
			slog.Error("failed to write dashboard HTML", "err", err)
		}
	}))

	mux.HandleFunc("/api/tools", authenticate(func(w http.ResponseWriter, r *http.Request) {
		allTools := registry.GetTools()
		names := make([]string, 0, len(allTools))
		for name := range allTools {
			names = append(names, name)
		}
		sort.Strings(names)
		var apiTools []any
		for _, name := range names {
			tool := allTools[name]
			cfg, ok := tools.GetConfig(tool)
			if !ok {
				continue
			}
			apiTools = append(apiTools, cfg)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(apiTools); err != nil {
			slog.Error("failed to encode response", "err", err)
		}
	}))

	mux.HandleFunc("/api/logs", authenticate(func(w http.ResponseWriter, r *http.Request) {
		logs, err := getAuditLogs(100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(logs); err != nil {
			slog.Error("failed to encode response", "err", err)
		}
	}))

	mux.HandleFunc("/api/metrics", authenticate(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		metrics := getMetrics()
		// Include concurrency info in a top-level wrapper.
		inUse := 0
		if semaphore != nil {
			inUse = len(semaphore)
		}
		wrapper := map[string]any{
			"tools": metrics,
			"concurrency": map[string]any{
				"max_concurrent_tools": semaphoreSize,
				"in_use":               inUse,
				"available":            semaphoreSize - inUse,
			},
		}
		if err := json.NewEncoder(w).Encode(wrapper); err != nil {
			slog.Error("failed to encode response", "err", err)
		}
	}))

	slog.Info("dashboard starting", "addr", fmt.Sprintf(":%d", port))
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("dashboard server failed", "err", err)
		}
	}()

	return httpServer
}
