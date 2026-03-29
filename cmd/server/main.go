// Museum Docs Generator - Standalone Documentation Service
// This service generates dynamic HTML documentation from YAML spec
package main

import (
	"flag"
	"log"
	"os"

	"docs-generator/pkg/docs"

	"github.com/gin-gonic/gin"
)

func main() {
	var (
		specPath = flag.String("spec", "./spec/index.yaml", "Path to spec file or directory")
		port     = flag.String("port", "8080", "Server port")
		devMode  = flag.Bool("dev", false, "Development mode (hot-reload)")
	)
	flag.Parse()

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	log.Println("🏛️  Museum Docs Generator")
	log.Println("═══════════════════════════════════════")

	// Create docs handler
	handler, err := docs.NewHandler(*specPath, *devMode)
	if err != nil {
		log.Fatalf("❌ Failed to initialize docs handler: %v", err)
	}

	// Create router
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

	// Register routes
	handler.RegisterRoutes(router)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"service":   "docs-generator",
			"dev_mode":  *devMode,
			"spec_path": *specPath,
		})
	})

	// Print info
	log.Printf("📄 Spec path: %s", *specPath)
	log.Printf("🔄 Dev mode: %v", *devMode)
	log.Println("")
	log.Println("📚 Endpoints:")
	log.Printf("   - http://localhost:%s/docs              - HTML Documentation", *port)
	log.Printf("   - http://localhost:%s/docs?p=<project>   - Project docs", *port)
	log.Printf("   - http://localhost:%s/api/docs/spec      - AI Spec (JSON)", *port)
	log.Printf("   - http://localhost:%s/api/docs/specs      - List projects", *port)
	log.Printf("   - http://localhost:%s/api/docs/yaml       - Download YAML", *port)
	log.Printf("   - http://localhost:%s/health              - Health check", *port)
	log.Println("")

	// Start server
	if err := router.Run(":" + *port); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
