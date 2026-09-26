package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/snonux/totalrecall/internal/bgtutor/mcpserver"
)

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP server over the episode library (Streamable HTTP at /mcp)",
		Long: `Run the MCP server. Set BGTUTOR_TOKEN to require a bearer token
("Authorization: Bearer <token>" or "?token=<token>" on the URL). Without a
token the server refuses to listen on anything but localhost.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dataDir, _ := cmd.Flags().GetString("data-dir")
			return serve(cmd.Context(), dataDir, addr, os.Getenv("BGTUTOR_TOKEN"))
		},
	}
	cmd.Flags().StringVar(&addr, "addr", envOr("BGTUTOR_ADDR", "127.0.0.1:8080"), "listen address (env BGTUTOR_ADDR)")
	return cmd
}

func serve(ctx context.Context, dataDir, addr, token string) error {
	if token == "" && !isLoopback(addr) {
		return fmt.Errorf("refusing to listen on %s without BGTUTOR_TOKEN; set a token or use 127.0.0.1", addr)
	}
	if st, err := os.Stat(dataDir); err != nil || !st.IsDir() {
		return fmt.Errorf("data dir %q does not exist", dataDir)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv := &http.Server{
		Addr:              addr,
		Handler:           mcpserver.Handler(mcpserver.New(dataDir), mcpserver.HTTPOptions{Token: token, Logger: logger}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	// ListenAndServe returns as soon as Shutdown starts, so wait for
	// Shutdown to finish draining in-flight requests before returning.
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	auth := "none (localhost only)"
	if token != "" {
		auth = "bearer token"
	}
	fmt.Fprintf(os.Stderr, "bgtutor: serving %s at http://%s/mcp (auth: %s)\n", dataDir, addr, auth)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-shutdownDone
	return nil
}

// isLoopback reports whether addr only binds to the local machine.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
