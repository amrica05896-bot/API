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
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" { return c.Status(400).JSON(fiber.Map{"error": "URL مطلوب"}) }

		format := "bestaudio/best"
		if c.Query("type") == "video" { format = "best[ext=mp4]/best" }

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// 🚀 الأوامر دي بتجبره يستخدم الكاش المحمل مسبقاً
		cmd := exec.CommandContext(ctx, "yt-dlp",
			"--quiet", "--no-warnings", "--no-playlist",
			"--js-runtimes", "node",
			"--remote-components", "ejs:github",
			"--cookies", "cookies.txt",
			"-f", format,
			"-g", videoURL)
		
		out, err := cmd.CombinedOutput()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "فشل الاستخراج", "details": string(out)})
		}

		return c.JSON(fiber.Map{"status": "success", "direct_url": strings.TrimSpace(string(out))})
	})

    // (بقية الـ Endpoints بنفس منطق الـ args اللي فوق)
    
	port := os.Getenv("PORT")
	if port == "" { port = "7860" }
	log.Fatal(app.Listen(":" + port))
}
