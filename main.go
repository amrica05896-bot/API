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

// دالة مركزية لاستخراج الرابط بأقصى سرعة ممكنة
func getFastURL(ctx context.Context, videoURL, format string) (string, error) {
	args := []string{
		"--quiet", "--no-warnings", "--no-playlist",
		"--js-runtimes", "node",                     // الأسرع في فك التشفير
		"--remote-components", "ejs:github",         // محملة مسبقاً في الدوكر
		"--cookies", "cookies.txt",                  // لتخطي حظر البوتات
		"-f", format,
		"-g", videoURL,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

func main() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// 1. Download Endpoint
	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"error": "يجب إرسال رابط يوتيوب"})
		}

		// 🚀 السحر هنا: استخدام 140 لملفات m4a المتوافقة فورياً مع FFmpeg في البوت
		mediaFormat := "140"
		if c.Query("type") == "video" {
			mediaFormat = "best[ext=mp4]/best" // MP4 ممتاز أيضاً للبث المرئي
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		res, err := getFastURL(ctx, videoURL, mediaFormat)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل الاستخراج", "details": res})
		}
		return c.JSON(fiber.Map{"status": "success", "direct_url": res})
	})

	// 2. Formats Endpoint
	app.Get("/formats", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" {
			return c.Status(400).JSON(fiber.Map{"error": "url مطلوب"})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "yt-dlp",
			"--quiet", "--no-warnings", "--no-playlist",
			"--js-runtimes", "node",
			"--remote-components", "ejs:github",
			"--cookies", "cookies.txt",
			"-J", videoURL)
		
		out, err := cmd.CombinedOutput()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل جلب الجودات", "details": string(out)})
		}

		return c.Type("json").Send(out)
	})

	// 3. 🚀 WebSocket Endpoint (الماسورة)
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
			
			// تطبيق نفس الجودة الصاروخية هنا
			format := "140"
			if len(parts) > 1 && parts[1] == "video" {
				format = "best[ext=mp4]/best"
			}

			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			
			res, err := getFastURL(ctx, videoURL, format)
			cancel()

			response := fiber.Map{
				"direct_url": res,
				"time_taken": time.Since(startTime).String(),
			}
			if err != nil {
				response["error"] = "فشل سريع"
				response["details"] = res
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
