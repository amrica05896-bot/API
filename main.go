package main

import (
	"context"
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
	// إعداد كاش عملاق لاستغلال الـ 30 جيجا رام وتقليل عمليات الاستخراج المتكررة
	cacheConfig := bigcache.DefaultConfig(20 * time.Minute)
	cacheConfig.Shards = 2048 // تقسيم عالي جداً لضمان عدم حدوث بطء مع الـ 10 كور
	cacheConfig.MaxEntriesInWindow = 1000 * 60 * 20
	cacheConfig.MaxEntrySize = 1024
	cacheConfig.HardMaxCacheSize = 2048 // حجز 2 جيجا رام للكاش فقط
	cache, _ := bigcache.New(context.Background(), cacheConfig)

	app := fiber.New(fiber.Config{
		Prefork:           true, // تشغيل سيرفر لكل كور (10 سيرفرات متوازية)
		ReduceMemoryUsage: false, // تعطيل تقليل الميموري لأن عندك 30 جيجا فمحتاجين سرعة مش توفير
		ServerHeader:      "Annie-HighPerformance-2026",
		BodyLimit:         10 * 1024 * 1024,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	})

	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	// المحرك الأساسي للاستخراج باستخدام الكوكيز الثابتة في الريبو
	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"status": "fail", "error": "url is required"})
		}

		mediaFormat := "bestaudio/best"
		if c.Query("type") == "video" {
			mediaFormat = "best[ext=mp4]/best"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		// تنفيذ yt-dlp باستخدام ملف الكوكيز الموجود في مسار /app/cookies.txt
		cmd := exec.CommandContext(ctx, "yt-dlp",
			"--cookies", "/app/cookies.txt",
			"--quiet",
			"--no-warnings",
			"--no-check-certificates",
			"--extractor-args", "youtube:player_client=web",
			"-g", "-f", mediaFormat, videoURL,
		)

		out, err := cmd.Output()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "check cookies validity", "details": err.Error()})
		}

		rawURL := strings.TrimSpace(string(out))
		if rawURL == "" {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "extraction returned empty content"})
		}

		token := "AN_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:20]
		_ = cache.Set(token, []byte(rawURL))

		return c.JSON(fiber.Map{
			"status":         "success",
			"video_id":       videoURL,
			"download_token": token,
			"timestamp":      time.Now().Unix(),
		})
	})

	app.Get("/stream/:vid", func(c *fiber.Ctx) error {
		token := c.Query("token")
		directURL, err := cache.Get(token)
		if err != nil {
			return c.Status(403).SendString("expired or invalid session token")
		}
		
		c.Set("X-Server-Core", "10-Cores-Power")
		return c.Redirect(string(directURL), 302)
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "online", 
			"cores": runtime.NumCPU(), 
			"ram_usage": "optimized",
			"version": "2026.4.18",
		})
	})

	app.Listen(":7860")
}
