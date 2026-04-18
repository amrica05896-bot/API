package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func main() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// 1. Download
	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"error": "يجب إرسال رابط يوتيوب"})
		}

		mediaType := c.Query("type", "audio")
		mediaFormat := "bestaudio/best"
		if mediaType == "video" {
			mediaFormat = "best[ext=mp4]/best"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "yt-dlp",
			"--quiet", "--no-warnings",
			"--extractor-args", "youtube:player_client=android",
			"-g", "-f", mediaFormat, videoURL)
		
		// استخدام CombinedOutput لكشف أي خطأ
		out, err := cmd.CombinedOutput()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل استخراج الرابط", "details": string(out)})
		}

		return c.JSON(fiber.Map{"status": "success", "direct_url": strings.TrimSpace(string(out))})
	})

	// 2. Formats
	app.Get("/formats", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"error": "url مطلوب"})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "yt-dlp", "--quiet", "-J", "--extractor-args", "youtube:player_client=android", videoURL)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل جلب الجودات", "details": string(out)})
		}

		return c.Type("json").Send(out)
	})

	// 3. 🚀 الماسورة (WebSocket)
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		log.Println("✅ تم فتح الماسورة مع العميل")
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("❌ انقطعت الماسورة:", err)
				break
			}

			parts := strings.SplitN(string(msg), "|", 2)
			videoURL := parts[0]
			
			mediaFormat := "bestaudio/best"
			if len(parts) > 1 && parts[1] == "video" {
				mediaFormat = "best[ext=mp4]/best"
			}

			startTime := time.Now()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			cmd := exec.CommandContext(ctx, "yt-dlp", "-f", mediaFormat, "-g", "--extractor-args", "youtube:player_client=android", videoURL)
			out, err := cmd.CombinedOutput()
			cancel()

			response := fiber.Map{
				"direct_url": strings.TrimSpace(string(out)),
				"time_taken": time.Since(startTime).String(),
			}
			if err != nil {
				response["error"] = "فشل سريع"
				response["details"] = string(out)
			}

			c.WriteJSON(response)
		}
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "7860"
	}
	log.Fatal(app.Listen(":" + port))
}
