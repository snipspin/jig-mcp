package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"log/slog"

	"github.com/joho/godotenv"
	"github.com/snipspin/jig-mcp/common"
	"github.com/snipspin/jig-mcp/internal/audit"
	"github.com/snipspin/jig-mcp/internal/auth"
	"github.com/snipspin/jig-mcp/internal/config"
	"github.com/snipspin/jig-mcp/internal/dashboard"
	"github.com/snipspin/jig-mcp/internal/logging"
	"github.com/snipspin/jig-mcp/internal/server"
	"github.com/snipspin/jig-mcp/internal/sse"
	"github.com/snipspin/jig-mcp/internal/tools"
)

// globalTimeout is the server-wide default timeout, overridable via JIG_TOOL_TIMEOUT.
var globalTimeout = defaultGlobalTimeout

// toolSemaphore limits the number of concurrent tool executions.
var toolSemaphore chan struct{}

// toolSemaphoreSize stores the configured capacity for metrics reporting.
var toolSemaphoreSize int

// Version is set at build time via -ldflags.
var Version = "dev"

// defaultGlobalTimeout is used when no per-tool or env-level timeout is set.
const defaultGlobalTimeout = 30 * time.Second

// shutdownGracePeriod is the maximum time to wait for in-flight work to
// finish after receiving SIGTERM/SIGINT.
const shutdownGracePeriod = 10 * time.Second

// inflightTools tracks in-flight tool call executions for graceful shutdown.
var inflightTools sync.WaitGroup

// initToolSemaphore creates the buffered-channel semaphore.
func initToolSemaphore() {
	maxTools := 8
	if v := os.Getenv("JIG_MAX_CONCURRENT_TOOLS"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &maxTools); err != nil || maxTools < 1 {
			slog.Warn("invalid JIG_MAX_CONCURRENT_TOOLS, using default", logging.Sanitize("value", v), slog.Int("default", maxTools))
			maxTools = 8
		}
	}
	toolSemaphoreSize = maxTools
	toolSemaphore = make(chan struct{}, maxTools)
	slog.Info("tool concurrency limit configured", "max_concurrent_tools", maxTools)
}

// initGlobalTimeout reads JIG_TOOL_TIMEOUT and sets the global timeout.
func initGlobalTimeout() {
	if v := os.Getenv("JIG_TOOL_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			globalTimeout = d
			slog.Info("global tool timeout set via JIG_TOOL_TIMEOUT", "timeout", logging.Untaint(d))
		} else {
			slog.Warn("invalid JIG_TOOL_TIMEOUT, using default", logging.Sanitize("value", v), slog.Duration("default", defaultGlobalTimeout))
		}
	}
}

// loadEnvFiles loads .env files from the specified directory.
// It silently ignores missing files (returns nil).
func loadEnvFiles(dir string) error {
	envPath := filepath.Join(dir, ".env")
	// godotenv.Load returns an error if file doesn't exist - we ignore it
	_ = godotenv.Load(envPath)
	return nil
}

// applyEnvOverrides applies JIG_* environment variable overrides to the
// provided flag pointers.
func applyEnvOverrides(transport *string, ssePort *int, configDir *string) {
	if t := os.Getenv("JIG_TRANSPORT"); t != "" {
		*transport = t
	}
	if p := os.Getenv("JIG_SSE_PORT"); p != "" {
		if _, err := fmt.Sscanf(p, "%d", ssePort); err != nil || *ssePort < 1 || *ssePort > 65535 {
			slog.Warn("invalid JIG_SSE_PORT, using default", logging.Sanitize("value", p), slog.Int("default", 3001))
			*ssePort = 3001
		}
	}
	if c := os.Getenv("JIG_CONFIG_DIR"); strings.TrimSpace(c) != "" {
		*configDir = c
	}
}

// detectConfigDir determines the config directory based on:
// 1. Explicit override (if provided, use it)
// 2. Auto-detect from binary location (install-root or legacy layout)
// 3. Empty string means "use current working directory"
// Returns the detected directory path (may be empty) and any error.
func detectConfigDir(explicitDir, binaryDir string) (string, error) {
	if strings.TrimSpace(explicitDir) != "" {
		return explicitDir, nil
	}

	// Check if we're in an install-root layout (bin/ subdirectory)
	if filepath.Base(binaryDir) == "bin" {
		installRoot := filepath.Dir(binaryDir)
		if _, err := os.Stat(filepath.Join(installRoot, "tools")); err == nil {
			return installRoot, nil
		}
	}

	// Fallback: check if tools/ exists next to binary (legacy layout)
	if _, err := os.Stat(filepath.Join(binaryDir, "tools")); err == nil {
		return binaryDir, nil
	}

	// No tools/ found - return empty to signal "use current directory"
	return "", nil
}

// loadTools creates a new tool registry and loads tools from manifests and scripts.
// Returns the populated registry or an error.
func loadTools(configDir string) (*tools.Registry, error) {
	registry := tools.NewRegistry()

	// Load tools from manifests
	manifestTools := config.LoadManifests(configDir)
	existingNames := make(map[string]bool)
	for _, lt := range manifestTools {
		existingNames[lt.Config.Name] = true
		var tool common.Tool
		switch lt.Type {
		case "http":
			tool = tools.HTTPTool{BaseTool: tools.BaseTool{Config: lt.Config}}
		case "terminal":
			tool = tools.TerminalTool{BaseTool: tools.BaseTool{Config: lt.Config}}
		default:
			tool = tools.ExternalTool{BaseTool: tools.BaseTool{Config: lt.Config}}
		}
		registry.RegisterTool(lt.Config.Name, tool)
	}

	// Probe scripts for additional tools
	scriptTools := config.ProbeScripts(existingNames, configDir)
	for _, lt := range scriptTools {
		tool := tools.ExternalTool{BaseTool: tools.BaseTool{Config: lt.Config}}
		registry.RegisterTool(lt.Config.Name, tool)
	}

	return registry, nil
}

// createServer creates a new server.Server with the given configuration.
func createServer(registry *tools.Registry, globalTimeout time.Duration, semaphore chan struct{}, semaphoreSize int, version string, onToolCall func(string, int64)) *server.Server {
	return &server.Server{
		Registry:      registry,
		GlobalTimeout: globalTimeout,
		Semaphore:     semaphore,
		SemaphoreSize: semaphoreSize,
		Version:       version,
		OnToolCall:    onToolCall,
	}
}

// runStdioServer reads JSON-RPC requests from stdin and writes responses to stdout.
// It processes each request in a separate goroutine and respects the notification
// semantics of JSON-RPC (no response for notifications).
func runStdioServer(
	stdin *os.File,
	stdout *os.File,
	stdoutMu *sync.Mutex,
	srv *server.Server,
	ctx context.Context,
	requestWg *sync.WaitGroup,
) {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20) // 1 MB — matches SSE body limit

	for scanner.Scan() {
		// Copy scanner bytes — the underlying buffer is reused on next Scan.
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())

		requestWg.Add(1)
		go func() {
			defer requestWg.Done()
			var req server.Request
			if err := json.Unmarshal(line, &req); err != nil {
				return
			}

			// JSON-RPC notifications have no "id" field. Per spec, servers
			// must not send a response for notifications.
			if req.ID == nil || string(req.ID) == "null" {
				slog.Debug("notification received, no response sent", "method", req.Method)
				return
			}

			inflightTools.Add(1)
			defer inflightTools.Done()

			resp := srv.ProcessRequest(ctx, req)
			out, err := json.Marshal(resp)
			if err != nil {
				slog.Error("failed to marshal JSON-RPC response", "error", err, "method", req.Method)
				return
			}
			stdoutMu.Lock()
			fmt.Fprintf(stdout, "%s\n", out)
			stdoutMu.Unlock()
		}()
	}
}

// gracefulShutdown coordinates an orderly shutdown:
// 1. Close all SSE sessions (send close events)
// 2. Shut down HTTP servers (stop accepting new connections)
// 3. Wait for in-flight tool calls to finish (with timeout)
func gracefulShutdown(servers []*http.Server) {
	// 1. Notify SSE clients before tearing down HTTP listeners.
	sse.CloseAllSessions()

	// 2. Shut down HTTP servers with the grace period as deadline.
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("HTTP server shutdown error", "err", err)
		}
	}

	// 3. Wait for in-flight tool executions to complete (or timeout).
	done := make(chan struct{})
	go func() {
		inflightTools.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all in-flight tool calls completed")
	case <-ctx.Done():
		slog.Warn("shutdown grace period expired, some tool calls may have been interrupted")
	}

	audit.Close()
	slog.Info("shutdown complete")
}

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	dashPort := flag.Int("dashboard-port", 0, "Optional HTTP port for a status dashboard (default 0, disabled)")
	transport := flag.String("transport", "stdio", "Transport mode: stdio or sse")
	ssePort := flag.Int("port", 3001, "Port for SSE transport")
	configDir := flag.String("config-dir", "", "Config directory containing tools/ and scripts/ (default: auto-detect from binary location)")
	flag.Parse()

	if *showVersion {
		fmt.Println("jig-mcp " + Version)
		return
	}

	// Load .env files from binary location
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		if filepath.Base(exeDir) == "bin" {
			installRoot := filepath.Dir(exeDir)
			_ = loadEnvFiles(installRoot)
		} else {
			_ = loadEnvFiles(exeDir)
		}
	}

	logging.InitLogging()
	if err := audit.Init(); err != nil {
		slog.Error("failed to initialize audit log", "err", err)
		os.Exit(1)
	}

	// Apply environment variable overrides
	applyEnvOverrides(transport, ssePort, configDir)

	auth.InitTokenRegistry()
	initGlobalTimeout()
	initToolSemaphore()

	// Detect config directory
	var detectedDir string
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		var err error
		detectedDir, err = detectConfigDir(*configDir, exeDir)
		if err != nil {
			slog.Error("failed to detect config directory", "err", err)
			os.Exit(1)
		}
		if detectedDir != "" && detectedDir != *configDir {
			*configDir = detectedDir
			slog.Info("config directory auto-detected", "dir", detectedDir)
		} else if detectedDir == "" {
			slog.Info("using current working directory for config")
		}
	}

	// Load tools into registry
	registry, err := loadTools(*configDir)
	if err != nil {
		slog.Error("failed to load tools", "err", err)
		os.Exit(1)
	}

	dashboard.SeedMetricsFromLog()

	// Collect servers that need graceful shutdown.
	var servers []*http.Server

	if *dashPort > 0 {
		servers = append(servers, dashboard.StartDashboard(*dashPort, registry, toolSemaphore, toolSemaphoreSize))
	}

	if *transport == "sse" {
		sseServer, sseErrCh := sse.StartSSEServer(*ssePort, registry, globalTimeout, toolSemaphore, toolSemaphoreSize, Version, dashboard.RecordMetric)
		servers = append(servers, sseServer)

		// Block until a termination signal or server error arrives.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

		select {
		case s := <-sig:
			slog.Info("received signal, starting graceful shutdown", "signal", s, "grace_period", shutdownGracePeriod)
		case err := <-sseErrCh:
			slog.Error("SSE server failed, starting graceful shutdown", "err", err)
		}

		gracefulShutdown(servers)
		return
	}

	// stdio transport — signal handling for graceful exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	var stdoutMu sync.Mutex

	// Create server for processing requests
	srv := &server.Server{
		Registry:      registry,
		GlobalTimeout: globalTimeout,
		Semaphore:     toolSemaphore,
		SemaphoreSize: toolSemaphoreSize,
		Version:       Version,
		OnToolCall:    dashboard.RecordMetric,
	}

	// Stdio callers inherit OS-level permissions — identity is always "local".
	stdioCtx := auth.WithCaller(context.Background(), auth.CallerIdentity{
		Name:      "local",
		Transport: "stdio",
	})

	// requestWg waits for all request processing goroutines to complete.
	var requestWg sync.WaitGroup

	stdioDone := make(chan struct{})
	go func() {
		defer close(stdioDone)
		runStdioServer(os.Stdin, os.Stdout, &stdoutMu, srv, stdioCtx, &requestWg)
	}()

	select {
	case <-stdioDone:
		// stdin closed — wait for all in-flight requests to complete (with timeout).
		requestDone := make(chan struct{})
		go func() {
			requestWg.Wait()
			close(requestDone)
		}()
		timer := time.NewTimer(shutdownGracePeriod)
		select {
		case <-requestDone:
			slog.Debug("all pending requests completed")
		case <-timer.C:
			slog.Warn("shutdown grace period expired, some requests may have been interrupted")
		}
		timer.Stop()
	case s := <-sig:
		slog.Info("received signal, starting graceful shutdown", "signal", s, "grace_period", shutdownGracePeriod)
	}

	// Shut down any running servers (e.g. dashboard in stdio mode).
	if len(servers) > 0 {
		gracefulShutdown(servers)
	} else {
		// Wait for in-flight tool calls to finish before exiting.
		done := make(chan struct{})
		go func() {
			inflightTools.Wait()
			close(done)
		}()
		timer := time.NewTimer(shutdownGracePeriod)
		select {
		case <-done:
			slog.Info("all in-flight tool calls completed")
		case <-timer.C:
			slog.Warn("shutdown grace period expired, some tool calls may have been interrupted")
		}
		timer.Stop()
		audit.Close()
	}
}
