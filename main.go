package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/google/uuid"
)

func main() {
	cacheConfig := bigcache.DefaultConfig(10 * time.Minute)
	cacheConfig.Shards = 1024
	cacheConfig.MaxEntriesInWindow = 1000 * 10 * 60
	cacheConfig.MaxEntrySize = 512
	cacheConfig.HardMaxCacheSize = 1024 
	cache, _ := bigcache.New(context.Background(), cacheConfig)

	app := fiber.New(fiber.Config{
		Prefork:           true,
		DisableKeepalive:  false,
		ReduceMemoryUsage: true,
		ServerHeader:      "AnnieEngine",
		BodyLimit:         5 * 1024 * 1024,
	})

	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	app.Get("/download", func(c *fiber.Ctx) error {
		videoID := c.Query("url")
		if videoID == "" {
			return c.Status(400).JSON(fiber.Map{"status": "fail", "error": "missing url"})
		}

		format := "bestaudio/best"
		if c.Query("type") == "video" {
			format = "best[ext=mp4]/best"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "yt-dlp",
			"--username", "oauth2",
			"--password", "''",
			"--quiet",
			"--no-warnings",
			"--extractor-args", "youtube:player_client=tv,web",
			"-g", "-f", format, videoID,
		)

		out, err := cmd.Output()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "error": "extraction failed"})
		}

		directURL := strings.TrimSpace(string(out))
		if directURL == "" {
			return c.Status(500).JSON(fiber.Map{"status": "error", "error": "empty url returned"})
		}

		token := "AN_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
		_ = cache.Set(token, []byte(directURL))

		return c.JSON(fiber.Map{
			"status":         "success",
			"video_id":       videoID,
			"download_token": token,
		})
	})

	app.Get("/stream/:vid", func(c *fiber.Ctx) error {
		token := c.Query("token")
		directURL, err := cache.Get(token)
		if err != nil {
			return c.Status(403).SendString("forbidden: invalid token")
		}
		
		c.Set("X-Powered-By", "AnnieCore")
		return c.Redirect(string(directURL), 302)
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "cores": runtime.NumCPU()})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "7860"
	}
	app.Listen(":" + port)
}
