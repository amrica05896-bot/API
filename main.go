package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2" // مكتبة الماسورة
)

func main() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// 1. نقطة النهاية العادية (Download) مع دعم type (صوت أو فيديو)
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

		out, err := exec.CommandContext(ctx, "yt-dlp",
			"--quiet", "--no-warnings",
			"--extractor-args", "youtube:player_client=android",
			"--cookies", "cookies.txt",
			"-g", "-f", mediaFormat, videoURL).Output()

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل استخراج الرابط", "details": string(out)})
		}

		return c.JSON(fiber.Map{"status": "success", "direct_url": strings.TrimSpace(string(out))})
	})

	// 2. نقطة النهاية لجلب جميع الجودات (Formats)
	app.Get("/formats", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"error": "url مطلوب"})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// جلب البيانات بصيغة JSON كاملة
		out, err := exec.CommandContext(ctx, "yt-dlp", "--quiet", "-J", "--cookies", "cookies.txt", videoURL).Output()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل جلب الجودات"})
		}

		return c.Type("json").Send(out)
	})

	// 3. 🚀 الماسورة (WebSocket) - الاتصال المتواصل
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		log.Println("✅ تم فتح الماسورة مع العميل")
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("❌ انقطعت الماسورة:", err)
				break
			}

			// استخراج الرابط والنوع من رسالة العميل. نفترض أن الرسالة بالشكل: url|type
			// مثال: https://youtube.com/watch?v=...|video
			parts := strings.SplitN(string(msg), "|", 2)
			videoURL := parts[0]
			
			mediaFormat := "bestaudio/best" // افتراضي: صوت
			if len(parts) > 1 && parts[1] == "video" {
				mediaFormat = "best[ext=mp4]/best"
			}

			startTime := time.Now()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			out, err := exec.CommandContext(ctx, "yt-dlp", "-f", mediaFormat, "-g", "--cookies", "cookies.txt", videoURL).Output()
			cancel()

			response := fiber.Map{
				"direct_url": strings.TrimSpace(string(out)),
				"time_taken": time.Since(startTime).String(),
			}
			if err != nil {
				response["error"] = "فشل سريع"
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
