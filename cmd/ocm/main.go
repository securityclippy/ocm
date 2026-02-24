package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openclaw/ocm/internal/api"
	"github.com/openclaw/ocm/internal/store"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	// Flags
	agentAddr := flag.String("agent-addr", ":9999", "Agent API listen address")
	adminAddr := flag.String("admin-addr", ":8080", "Admin API/UI listen address")
	dbPath := flag.String("db", "ocm.db", "Database path")
	masterKeyFile := flag.String("master-key-file", "", "Path to master key file (or set OCM_MASTER_KEY)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ocm %s (%s)\n", version, commit)
		os.Exit(0)
	}

	// Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Master key
	masterKey, err := loadMasterKey(*masterKeyFile)
	if err != nil {
		slog.Error("failed to load master key", "error", err)
		os.Exit(1)
	}

	// Initialize store
	db, err := store.New(*dbPath, masterKey)
	if err != nil {
		slog.Error("failed to initialize store", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create routers
	agentRouter := api.NewAgentRouter(db, logger)
	adminRouter := api.NewAdminRouter(db, logger)

	// Start servers
	agentServer := &http.Server{
		Addr:         *agentAddr,
		Handler:      agentRouter,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	adminServer := &http.Server{
		Addr:         *adminAddr,
		Handler:      adminRouter,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		agentServer.Shutdown(shutdownCtx)
		adminServer.Shutdown(shutdownCtx)
	}()

	// Start agent API
	go func() {
		slog.Info("starting agent API", "addr", *agentAddr)
		if err := agentServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("agent server error", "error", err)
			cancel()
		}
	}()

	// Start admin API/UI
	go func() {
		slog.Info("starting admin API/UI", "addr", *adminAddr)
		if err := adminServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("admin server error", "error", err)
			cancel()
		}
	}()

	slog.Info("ocm started", "version", version, "agent", *agentAddr, "admin", *adminAddr)
	<-ctx.Done()
	slog.Info("ocm stopped")
}

func loadMasterKey(keyFile string) ([]byte, error) {
	// Try environment variable first
	if key := os.Getenv("OCM_MASTER_KEY"); key != "" {
		if len(key) != 64 {
			return nil, fmt.Errorf("OCM_MASTER_KEY must be 64 hex characters (32 bytes)")
		}
		return hexDecode(key)
	}

	// Try key file
	if keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		// Key file should be raw 32 bytes or 64 hex chars
		if len(data) == 32 {
			return data, nil
		}
		if len(data) == 64 || len(data) == 65 { // 65 for trailing newline
			return hexDecode(string(data[:64]))
		}
		return nil, fmt.Errorf("key file must be 32 bytes or 64 hex characters")
	}

	return nil, fmt.Errorf("no master key provided (set OCM_MASTER_KEY or use --master-key-file)")
}

func hexDecode(s string) ([]byte, error) {
	if len(s) != 64 {
		return nil, fmt.Errorf("expected 64 hex characters, got %d", len(s))
	}
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		_, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &b[i])
		if err != nil {
			return nil, fmt.Errorf("invalid hex at position %d: %w", i*2, err)
		}
	}
	return b, nil
}
