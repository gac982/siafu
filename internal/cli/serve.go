// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Siafu contributors

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gac982/siafu/internal/orchestrator"
	"github.com/gac982/siafu/internal/results"
	"github.com/gac982/siafu/internal/web"
)

func runServe(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", "127.0.0.1:8080", "dirección de escucha")
	resultsDir := flags.String("results-dir", "", "directorio de resultados")
	webDir := flags.String("web-dir", "", "directorio de assets para desarrollo")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprint(stderr, "uso: siafu serve [--addr DIR] [--results-dir DIR] [--web-dir DIR]\n")
		return 2
	}

	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		fmt.Fprintf(stderr, "dirección inválida: %v\n", err)
		return 2
	}
	if !isLoopbackHost(host) {
		fmt.Fprintf(stderr, "aviso: escuchando en %s; la UI no tiene autenticación y expone los resultados. Usa 127.0.0.1 o localhost.\n", host)
	}

	store, err := results.OpenStore(orchestrator.ResolveResultsDir(orchestrator.Options{ResultsDir: *resultsDir}))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	store.Warn = func(msg string) { fmt.Fprintf(stderr, "aviso: %s\n", msg) }
	var assets fs.FS = web.AssetsFS()
	if *webDir != "" {
		info, err := os.Stat(*webDir)
		if err != nil || !info.IsDir() {
			fmt.Fprintf(stderr, "web-dir inválido: %s\n", *webDir)
			return 2
		}
		assets = os.DirFS(*webDir)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := serveListener(ctx, listener, web.NewServer(store, assets).Handler(), stdout, store.Dir); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func serveListener(ctx context.Context, listener net.Listener, handler http.Handler, stdout io.Writer, resultsDir string) error {
	return serveListenerWithReady(ctx, listener, handler, stdout, resultsDir, nil)
}

func serveListenerWithReady(ctx context.Context, listener net.Listener, handler http.Handler, stdout io.Writer, resultsDir string, onReady func(string)) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	url := "http://" + listener.Addr().String()
	fmt.Fprintf(stdout, "siafu serve escuchando en %s (resultados: %s)\n", url, resultsDir)
	if onReady != nil {
		onReady(url)
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
