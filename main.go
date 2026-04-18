package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// نقطة النهاية للحصول على الرابط المباشر
	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"error": "يجب إرسال رابط يوتيوب"})
		}

		// تحديد الجودة (فيديو أو صوت)
		mediaType := c.Query("type", "audio")
		mediaFormat := "bestaudio/best"
		if mediaType == "video" {
			mediaFormat = "best[ext=mp4]/best"
		}

		// تحديد وقت أقصى للعملية (20 ثانية) حتى لا يعلق السيرفر
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// استخراج الرابط المباشر باستخدام yt-dlp
		cmd := exec.CommandContext(ctx, "yt-dlp",
			"--quiet",
			"--no-warnings",
			"--extractor-args", "youtube:player_client=android",
			"-g", // هذه الإضافة (-g) هي ما يجلب الرابط المباشر فقط
			"-f", mediaFormat,
			videoURL,
		)

		out, err := cmd.CombinedOutput()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":   "فشل استخراج الرابط",
				"details": string(out),
			})
		}

		// تنظيف الرابط المستخرج من المسافات
		rawURL := strings.TrimSpace(string(out))

		// إرجاع الرابط المباشر فوراً بدون توكن أو لف ودوران
		return c.JSON(fiber.Map{
			"status":     "success",
			"direct_url": rawURL,
		})
	})

	// نقطة نهاية إضافية (اختيارية): لو أردت أن يقوم السيرفر بتوجيهك فوراً للتحميل/التشغيل
	app.Get("/play", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		mediaType := c.Query("type", "audio")
		mediaFormat := "bestaudio/best"
		if mediaType == "video" {
			mediaFormat = "best[ext=mp4]/best"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "yt-dlp", "--quiet", "--no-warnings", "--extractor-args", "youtube:player_client=android", "-g", "-f", mediaFormat, videoURL).Output()
		if err != nil {
			return c.Status(500).SendString("فشل استخراج الرابط")
		}

		// إعادة توجيه المستخدم فوراً للرابط المباشر
		return c.Redirect(strings.TrimSpace(string(out)), 302)
	})

	// تشغيل الخادم
	port := os.Getenv("PORT")
	if port == "" {
		port = "7860"
	}
	log.Printf("🚀 Server running on port %s...", port)
	log.Fatal(app.Listen(":" + port))
}
