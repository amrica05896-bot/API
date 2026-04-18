package main

import (
	"context"
	"fmt"
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
	// إعداد الكاش لتخزين الروابط المستخرجة لمدة 20 دقيقة لتوفير الموارد
	cacheConfig := bigcache.DefaultConfig(20 * time.Minute)
	cacheConfig.Shards = 2048
	cacheConfig.MaxEntriesInWindow = 1000 * 60 * 20
	cache, _ := bigcache.New(context.Background(), cacheConfig)

	app := fiber.New(fiber.Config{
		Prefork:           true,
		ReduceMemoryUsage: false,
		ServerHeader:      "HighPerformance-API-2026",
		BodyLimit:         10 * 1024 * 1024,
	})

	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	// نقطة النهاية لاستخراج الروابط المباشرة
	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"status": "fail", "error": "url is required"})
		}

		mediaFormat := "bestaudio/best"
		if c.Query("type") == "video" {
			mediaFormat = "best[ext=mp4]/best"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		// تنفيذ yt-dlp باستخدام خلطة الأندرويد التي أثبتت نجاحها في الاختبار اليدوي
		// ملاحظة: تم إزالة خيار الكوكيز هنا لأن عميل الأندرويد نجح بدونها في الكونسول
		cmd := exec.CommandContext(ctx, "yt-dlp",
			"--quiet",
			"--no-warnings",
			"--no-check-certificates",
			"--extractor-args", "youtube:player_client=android",
			"-g", 
			"-f", mediaFormat, 
			videoURL,
		)

		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("🔴 Extraction Error: %s\n", string(out))
			return c.Status(500).JSON(fiber.Map{
				"status":  "error",
				"message": "extraction failed",
				"details": string(out),
			})
		}

		rawURL := strings.TrimSpace(string(out))
		if rawURL == "" {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "empty response from extractor"})
		}

		// توليد توكن فريد للرابط المستخرج
		token := "TX_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:15]
		_ = cache.Set(token, []byte(rawURL))

		return c.JSON(fiber.Map{
			"status":         "success",
			"video_id":       videoURL,
			"download_token": token,
		})
	})

	// نقطة النهاية لتوجيه البث المباشر باستخدام التوكن
	app.Get("/stream/:token", func(c *fiber.Ctx) error {
		token := c.Params("token")
		rawURL, err := cache.Get(token)
		if err != nil {
			return c.Status(404).SendString("Token expired or invalid")
		}
		return c.Redirect(string(rawURL), 302)
	})

	// فحص حالة السيرفر والمواصفات
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "online",
			"cores":   runtime.NumCPU(),
			"version": "2026.4.18-android-optimized",
		})
	})

	app.Listen(":7860")
}
