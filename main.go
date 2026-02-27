package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

const msgWidth = 35

func formatMsgFixed(i interface{}) string {
	s, _ := i.(string)
	if len(s) > msgWidth {
		return s[:msgWidth]
	}
	if len(s) < msgWidth {
		return s + strings.Repeat(" ", msgWidth-len(s))
	}
	return s
}

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:            os.Stdout,
		FormatMessage:  formatMsgFixed,
	}).With().Timestamp().Logger()

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.json"
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		logger.Fatal().Err(err).Str("path", configPath).Msg("failed to load config")
	}

	privateKey := cfg.PrivateKey
	if privateKey == "" {
		privateKey = os.Getenv("BROADCASTER_PRIVATE_KEY")
	}
	if privateKey == "" {
		logger.Fatal().Msg("private_key in config or BROADCASTER_PRIVATE_KEY environment variable is required")
	}
	signer, err := NewSigner(privateKey)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize signer")
	}

	httpClient := newSharedHTTPClient()
	manager := NewRelayManager(cfg, signer, httpClient, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	manager.WarmConnections(ctx)
	cancel()

	wsServer := NewWSServer(manager, logger)
	wsServer.Start()

	addr := cfg.Server.Addr()
	var auth *authGuard
	if cfg.Auth.PasswordHash != "" {
		auth = newAuthGuard(cfg.Auth.PasswordHash, cfg.Auth.MaxAttempts, cfg.Auth.LockoutMinutes)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsServer.HandleWS)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/metrics/view", metricsHandler(auth))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/metrics/view", http.StatusFound)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", addr).Msg("server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := wsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("WebSocket server shutdown error")
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("HTTP server shutdown error")
	}

	logger.Info().Msg("shutdown complete")
}
