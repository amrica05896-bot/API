package main

import (
	"context"
	"log/slog" // المعيار الحديث للسجلات في Go
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const maxConcurrentExtractions = 30

var rdb *redis.Client

func init() {
	// في 2026، الاتصال بـ Redis ضروري في السحابة لمنع فقدان البيانات
	// Fly.io توفر رابط Redis في المتغير البيئي REDIS_URL
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0" // للتطوير المحلي
	}
	
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("🔴 Failed to parse Redis URL", "error", err)
		os.Exit(1)
	}
	rdb = redis.NewClient(opt)
}

func main() {
	// إعداد سجلات النظام الحديثة (JSON format للسيرفرات السحابية)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	extractionSemaphore := make(chan struct{}, maxConcurrentExtractions)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ServerHeader:          "Cloud-Native-API-2026",
		BodyLimit:             10 * 1024 * 1024,
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          0, // مفتوح للسماح بالبث المباشر المستمر
		IdleTimeout:           60 * time.Second,
	})

	app.Use(recover.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" || !isValidYouTubeURL(videoURL) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "fail", "error": "valid youtube url is required"})
		}

		mediaType := c.Query("type", "audio")
		mediaFormat := "bestaudio/best"
		if mediaType == "video" {
			mediaFormat = "best[ext=mp4]/best"
		}

		ctxBg := context.Background()
		cacheKey := "vid:" + mediaType + ":" + videoURL

		// 1. الفحص من Redis (Distributed Cache)
		if cachedRawURL, err := rdb.Get(ctxBg, cacheKey).Result(); err == nil {
			token := generateToken()
			// تخزين التوكن لمدة 6 ساعات
			rdb.Set(ctxBg, "tok:"+token, cachedRawURL, 6*time.Hour)
			
			slog.Info("Served from cache", "url", videoURL, "token", token)
			return c.JSON(fiber.Map{
				"status":         "success",
				"video_id":       videoURL,
				"download_token": token,
				"cached":         true,
			})
		}

		// 2. التحكم في التزامن
		extractionSemaphore <- struct{}{}
		defer func() { <-extractionSemaphore }()

		ctxCmd, cancel := context.WithTimeout(ctxBg, 25*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctxCmd, "yt-dlp",
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
			slog.Error("Extraction failed", "url", videoURL, "error", string(out))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "extraction failed or timed out",
			})
		}

		rawURL := strings.TrimSpace(string(out))
		if rawURL == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "empty response"})
		}

		// 3. تخزين النتيجة في Redis
		token := generateToken()
		rdb.Set(ctxBg, "tok:"+token, rawURL, 6*time.Hour)
		rdb.Set(ctxBg, cacheKey, rawURL, 20*time.Minute) // تخزين الرابط الأصلي لمنع التكرار

		return c.JSON(fiber.Map{
			"status":         "success",
			"video_id":       videoURL,
			"download_token": token,
			"cached":         false,
		})
	})

	// 🚀 التحديث الأهم: عمل Proxy بدلاً من Redirect لتخطي حظر IP
	app.Get("/stream/:token", func(c *fiber.Ctx) error {
		token := c.Params("token")
		rawURL, err := rdb.Get(context.Background(), "tok:"+token).Result()
		
		if err != nil {
			slog.Warn("Invalid or expired token accessed", "token", token)
			return c.Status(fiber.StatusNotFound).SendString("Token expired or invalid")
		}

		// نقوم بجلب الفيديو من سيرفرنا وإرساله للمستخدم (Proxying)
		req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, rawURL, nil)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to create stream request")
		}
		
		// تقليد متصفح حديث لتجنب حظر يوتيوب
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return c.Status(fiber.StatusBadGateway).SendString("Upstream video provider failed")
		}

		// نقل ترويسات الاستجابة (مثل حجم الملف ونوع المحتوى)
		c.Set("Content-Type", resp.Header.Get("Content-Type"))
		c.Set("Content-Length", resp.Header.Get("Content-Length"))
		c.Set("Accept-Ranges", "bytes")

		// البث المباشر (Streaming) بكفاءة عالية عبر Fiber
		return c.SendStream(resp.Body)
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":         "online",
			"cores":          runtime.NumCPU(),
			"active_workers": len(extractionSemaphore),
			"redis_ping":     rdb.Ping(context.Background()).Err() == nil,
		})
	})

	go func() {
		if err := app.Listen(":7860"); err != nil {
			slog.Error("Server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("🛑 Gracefully shutting down...")
	_ = app.Shutdown()
	_ = rdb.Close()
	slog.Info("Shutdown complete.")
}

func generateToken() string {
	return "TX_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:15]
}

func isValidYouTubeURL(u string) bool {
	parsed, err := url.ParseRequestURI(u)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Host)
	return strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be")
}
