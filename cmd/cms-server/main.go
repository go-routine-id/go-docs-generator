// cms-server — authoring backend for docs-generator.
//
// Reads the same YAML spec directory that docs-generator serves, lets a
// signed-in admin edit guides through an HTML form, and writes the changes
// back to the same files. docs-generator (with -dev) picks up the new YAML
// via its file watcher; in production a redeploy reloads the spec.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"docs-generator/pkg/cms"

	"github.com/gin-gonic/gin"
)

func main() {
	os.Exit(run())
}

// run is the actual entry point as a function returning an exit code so a
// single deferred-cleanup path covers BOTH the graceful-shutdown branch and
// the fatal-listen-error branch — main's prior direct os.Exit(1) skipped
// `defer store.Close()` AND the gcCancel/Wait synchronisation entirely.
func run() int {
	var (
		specDir = flag.String("spec-dir", "./spec", "Path to the YAML spec directory the CMS authors against.")
		dbPath  = flag.String("db", "./cms.db", "Path to the SQLite database file.")
		addr    = flag.String("addr", ":8090", "HTTP listen address.")
		dev     = flag.Bool("dev", false, "Enable verbose logging and gin debug mode.")
	)
	flag.Parse()

	setupLogging(*dev)

	password := os.Getenv("CMS_ADMIN_PASSWORD")
	if password == "" {
		slog.Error("CMS_ADMIN_PASSWORD env var is not set — refusing to start")
		return 2
	}

	info, err := os.Stat(*specDir)
	if err != nil || !info.IsDir() {
		slog.Error("spec-dir must be an existing directory", "path", *specDir, "err", err)
		return 2
	}

	store, err := cms.OpenStore(*dbPath)
	if err != nil {
		slog.Error("open store", "path", *dbPath, "err", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	auth, err := cms.NewAuthenticator(store, password)
	if err != nil {
		slog.Error("init auth", "err", err)
		return 1
	}

	srv, err := cms.NewServer(store, auth, *specDir)
	if err != nil {
		slog.Error("init server", "err", err)
		return 1
	}

	if !*dev {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	if *dev {
		r.Use(gin.Logger())
	}
	srv.RegisterRoutes(r)

	// gcLoop runs against store, which is closed via defer above. The
	// cancel/Wait pair below joins the goroutine BEFORE this function
	// returns and the deferred store.Close() runs — otherwise the next
	// tick would Exec on a closed DB and log a noisy
	// "sql: database is closed" warning at every shutdown.
	gcCtx, gcCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		gcLoop(gcCtx, store)
	}()
	defer func() {
		gcCancel()
		// Bound the wait so a single in-flight GarbageCollect on a huge
		// sessions table can't block shutdown indefinitely; the deferred
		// store.Close() will then interrupt the Exec via the driver.
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			slog.Warn("gcLoop did not exit within 3s; continuing shutdown")
		}
	}()

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("cms-server starting",
		"addr", *addr,
		"spec_dir", srv.SpecDir(),
		"db", *dbPath,
	)

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	exitCode := 0
	select {
	case sig := <-stop:
		slog.Info("shutdown signal", "signal", sig.String())
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			exitCode = 1
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	return exitCode
}

// gcLoop runs the session-table garbage collector on a slow tick so expired
// rows don't pile up. Exits when ctx is cancelled.
func gcLoop(ctx context.Context, store *cms.Store) {
	t := time.NewTicker(1 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := store.GarbageCollect(); err != nil {
				slog.Warn("session gc", "err", err)
			}
		}
	}
}

func setupLogging(dev bool) {
	level := slog.LevelInfo
	if dev {
		level = slog.LevelDebug
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}
