// Comando talaia arranca el dashboard.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/izakdev/talaia/internal/api"
	"github.com/izakdev/talaia/internal/collector"
	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/integrations/adguard"
	"github.com/izakdev/talaia/internal/integrations/homeassistant"
	"github.com/izakdev/talaia/internal/integrations/proxmox"
	"github.com/izakdev/talaia/internal/secrets"
	"github.com/izakdev/talaia/internal/store"
	"github.com/izakdev/talaia/web"
)

var version = "dev"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("no se pudo arrancar", "error", err)
		os.Exit(1)
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func run(log *slog.Logger) error {
	addr := flag.String("listen", env("TALAIA_LISTEN", ":8080"), "dirección de escucha")
	dataDir := flag.String("data", env("TALAIA_DATA_DIR", "data"), "directorio de datos")
	keyFile := flag.String("key-file", env("TALAIA_KEY_FILE", ""), "archivo de la clave de cifrado (por defecto, dentro del directorio de datos)")
	showVersion := flag.Bool("version", false, "muestra la versión")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return nil
	}

	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		return fmt.Errorf("crear directorio de datos: %w", err)
	}
	if *keyFile == "" {
		*keyFile = filepath.Join(*dataDir, "secret.key")
	}
	key, err := secrets.LoadOrCreateKey(*keyFile)
	if err != nil {
		return err
	}
	box, err := secrets.New(key)
	if err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(*dataDir, "talaia.db"), box)
	if err != nil {
		return err
	}
	defer st.Close()

	registry := integrations.Registry{"adguard": adguard.New, "homeassistant": homeassistant.New, "proxmox": proxmox.New}
	manager := collector.New(st, registry, log)
	server, err := api.New(st, manager, registry, log, version, web.Dist())
	if err != nil {
		return err
	}
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := manager.Start(ctx); err != nil {
		return err
	}
	// Se espera a los recolectores antes de cerrar la base de datos.
	defer manager.Wait()
	defer stop()
	httpServer.BaseContext = func(net.Listener) context.Context { return ctx }
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			if err := st.DeleteExpiredSessions(ctx); err != nil && ctx.Err() == nil {
				log.Warn("no se pudieron limpiar las sesiones", "error", err)
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()
	log.Info("Talaia en marcha", "version", version, "listen", *addr, "data", *dataDir)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdown)
}
