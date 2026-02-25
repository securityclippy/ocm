package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/openclaw/ocm/internal/api"
	"github.com/openclaw/ocm/internal/elevation"
	"github.com/openclaw/ocm/internal/gateway"
	"github.com/openclaw/ocm/internal/store"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ocm",
	Short: "OpenClaw Credential Manager",
	Long: `OCM (OpenClaw Credential Manager) is a secure credential management sidecar.

It stores credentials outside the agent's process and requires human approval
for sensitive operations, injecting credentials via environment variables.`,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
}

// --- serve command ---

var serveFlags struct {
	agentAddr     string
	adminAddr     string
	dbPath        string
	masterKeyFile string
	gatewayURL    string
	envFile       string
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the OCM server",
	Long:  `Start the OCM server with both agent API and admin API/UI.`,
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveFlags.agentAddr, "agent-addr", ":9999", "Agent API listen address")
	serveCmd.Flags().StringVar(&serveFlags.adminAddr, "admin-addr", ":8080", "Admin API/UI listen address")
	serveCmd.Flags().StringVar(&serveFlags.dbPath, "db", "ocm.db", "Database path")
	serveCmd.Flags().StringVar(&serveFlags.masterKeyFile, "master-key-file", "", "Path to master key file (or set OCM_MASTER_KEY env)")
	serveCmd.Flags().StringVar(&serveFlags.gatewayURL, "gateway-url", "http://localhost:18789", "OpenClaw Gateway RPC URL")
	serveCmd.Flags().StringVar(&serveFlags.envFile, "env-file", "", "Path to .env file for credential injection (default: ~/.openclaw/.env)")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Master key
	masterKey, err := loadMasterKey(serveFlags.masterKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load master key: %w", err)
	}

	// Initialize store
	db, err := store.New(serveFlags.dbPath, masterKey)
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer db.Close()

	// Initialize gateway client
	gwClient := gateway.NewClient(serveFlags.gatewayURL, serveFlags.envFile)
	slog.Info("gateway client configured", "url", serveFlags.gatewayURL, "envFile", gwClient.EnvFilePath)

	// Initialize elevation service
	elevSvc := elevation.NewService(db, gwClient, logger)

	// Create routers
	agentRouter := api.NewAgentRouter(db, logger)
	adminRouter := api.NewAdminRouter(db, elevSvc, logger)

	// Start servers
	agentServer := &http.Server{
		Addr:         serveFlags.agentAddr,
		Handler:      agentRouter,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	adminServer := &http.Server{
		Addr:         serveFlags.adminAddr,
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
		slog.Info("starting agent API", "addr", serveFlags.agentAddr)
		if err := agentServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("agent server error", "error", err)
			cancel()
		}
	}()

	// Start admin API/UI
	go func() {
		slog.Info("starting admin API/UI", "addr", serveFlags.adminAddr)
		if err := adminServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("admin server error", "error", err)
			cancel()
		}
	}()

	slog.Info("ocm started", "version", version, "agent", serveFlags.agentAddr, "admin", serveFlags.adminAddr)
	<-ctx.Done()
	slog.Info("ocm stopped")
	return nil
}

// --- version command ---

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ocm %s (%s)\n", version, commit)
	},
}

// --- helpers ---

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
