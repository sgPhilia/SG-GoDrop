package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"godrop/internal/config"
	"godrop/internal/logger"
	"godrop/internal/server"
	"godrop/internal/transfer"
)

func main() {
	var (
		host string
		port int
		dir  string
	)
	flag.StringVar(&host, "host", "0.0.0.0", "host to bind to")
	flag.IntVar(&port, "port", 8080, "port to listen on")
	flag.StringVar(&dir, "dir", "", "temporary storage directory (default: OS temp)")
	flag.Parse()

	log := logger.New(os.Stdout, slog.LevelInfo)

	cfg := config.Default()
	if host != "0.0.0.0" {
		cfg.Host = host
	}
	if port != 8080 {
		cfg.Port = port
	}
	if dir != "" {
		cfg.TempDir = dir
	}

	mgr, err := transfer.NewManager(cfg.TempDir, log)
	if err != nil {
		log.Error("failed to create transfer manager", "error", err)
		os.Exit(1)
	}
	defer mgr.Cleanup()

	srv := server.New(cfg, mgr, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Error("server error", "error", err)
			cancel()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Info("shutting down gracefully...")
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}

	mgr.Cleanup()
	log.Info("goodbye")
}
