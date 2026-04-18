// Docs Generator - Standalone API Documentation Service
// Generates dynamic HTML documentation from YAML specification files.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"docs-generator/pkg/docs"

	"github.com/gin-gonic/gin"
)

func main() {
	var (
		specPath  = flag.String("spec", "./spec/index.yaml", "Path to spec file or directory")
		port      = flag.String("port", "8080", "Server port")
		prefix    = flag.String("prefix", "/docs", "URL prefix for documentation routes")
		devMode   = flag.Bool("dev", false, "Development mode (hot-reload)")
		logFormat = flag.String("log-format", defaultLogFormat(), "Log format: text or json")
	)
	flag.Parse()

	setupLogging(*logFormat)

	// Normalize prefix (ensure leading /, no trailing /)
	*prefix = "/" + strings.Trim(*prefix, "/")

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	printBanner(*specPath, *devMode, *prefix, *port)

	handler, err := docs.NewHandler(*specPath, *devMode)
	if err != nil {
		slog.Error("failed to initialize docs handler", "err", err)
		os.Exit(1)
	}

	router := gin.Default()

	// Enable CORS - needed for API tester to work cross-origin
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	handler.RegisterRoutes(router, *prefix)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"service":   "docs-generator",
			"dev_mode":  *devMode,
			"spec_path": *specPath,
		})
	})

	slog.Info("server starting", "port", *port, "prefix", *prefix, "dev", *devMode)
	if err := router.Run(":" + *port); err != nil {
		slog.Error("server stopped with error", "err", err)
		os.Exit(1)
	}
}

// setupLogging configures slog with the requested handler.
func setupLogging(format string) {
	level := slog.LevelInfo
	if lv := os.Getenv("LOG_LEVEL"); lv != "" {
		switch strings.ToLower(lv) {
		case "debug":
			level = slog.LevelDebug
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if strings.EqualFold(format, "json") {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

// defaultLogFormat picks JSON for non-dev environments, text for humans otherwise.
func defaultLogFormat() string {
	if os.Getenv("GIN_MODE") == "release" || os.Getenv("LOG_FORMAT") == "json" {
		return "json"
	}
	return "text"
}

// printBanner is kept as plain stderr output — it is presentation, not a log event.
func printBanner(specPath string, devMode bool, prefix, port string) {
	fmt.Fprintln(os.Stderr, "Docs Generator")
	fmt.Fprintln(os.Stderr, "===========================================")
	fmt.Fprintf(os.Stderr, "Spec:   %s\n", specPath)
	fmt.Fprintf(os.Stderr, "Dev:    %v\n", devMode)
	fmt.Fprintf(os.Stderr, "Prefix: %s\n", prefix)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Endpoints:")
	fmt.Fprintf(os.Stderr, "  http://localhost:%s%s\n", port, prefix)
	fmt.Fprintf(os.Stderr, "  http://localhost:%s%s/spec\n", port, prefix)
	fmt.Fprintf(os.Stderr, "  http://localhost:%s%s/specs\n", port, prefix)
	fmt.Fprintf(os.Stderr, "  http://localhost:%s%s/yaml\n", port, prefix)
	fmt.Fprintf(os.Stderr, "  http://localhost:%s/health\n", port)
	fmt.Fprintln(os.Stderr, "")
}
