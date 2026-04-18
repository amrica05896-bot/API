package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
)

// تحديد سعة العمليات المتزامنة القصوى لحماية المعالج (10 أنوية)
const maxConcurrentExtractions = 30

func main() {
	// إعداد الكاش بخصائص محسنة للتعامل مع المفاتيح المزدوجة (Tokens & VideoIDs)
	cacheConfig := bigcache.DefaultConfig(20 * time.Minute)
	cacheConfig.Shards = 2048
	cacheConfig.MaxEntriesInWindow = 1000 * 60 * 20
	cache, err := bigcache.New(context.Background(), cacheConfig)
	if err != nil {
		log.Fatalf("🔴 Critical Error: Failed to initialize BigCache: %v", err)
	}

	// Semaphore للتحكم في التزامن
	extractionSemaphore := make(chan struct{}, maxConcurrentExtractions)

	// إعدادات Fiber مع إضافة Timeouts لمنع استنزاف الموارد
	app := fiber.New(fiber.Config{
		Prefork:           false,
		ReduceMemoryUsage: false,
		ServerHeader:      "HighPerformance-API-2026",
		BodyLimit:         10 * 1024 * 1024,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	})

	// وسطاء (Middlewares) لحماية الخادم وضغط البيانات
	app.Use(recover.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	app.Get("/download", func(c *fiber.Ctx) error {
		videoURL := c.Query("url")
		if videoURL == "" || !isValidYouTubeURL(videoURL) {
			return c.Status(400).JSON(fiber.Map{"status": "fail", "error": "valid youtube url is required"})
		}

		mediaType := c.Query("type", "audio") // افتراضي: audio
		mediaFormat := "bestaudio/best"
		if mediaType == "video" {
			mediaFormat = "best[ext=mp4]/best"
		}

		// 1. فحص الكاش لمنع التكرار (Deduplication)
		cacheKey := "vid:" + mediaType + ":" + videoURL
		if cachedRawURL, err := cache.Get(cacheKey); err == nil {
			// إذا كان الرابط مستخرجاً مسبقاً، نولد توكن جديد ونربطه بالرابط الجاهز
			token := generateToken()
			_ = cache.Set("tok:"+token, cachedRawURL)
			return c.JSON(fiber.Map{
				"status":         "success",
				"video_id":       videoURL,
				"download_token": token,
				"cached":         true,
			})
		}

		// 2. التحكم في التزامن: الدخول إلى الـ Semaphore (ينتظر إذا تجاوزنا maxConcurrentExtractions)
		extractionSemaphore <- struct{}{}
		defer func() { <-extractionSemaphore }() // تحرير الخانة عند انتهاء الدالة

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

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
			log.Printf("🔴 Extraction Error for %s: %s\n", videoURL, string(out))
			return c.Status(500).JSON(fiber.Map{
				"status":  "error",
				"message": "extraction failed or timed out",
			})
		}

		rawURL := strings.TrimSpace(string(out))
		if rawURL == "" {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "empty response from extractor"})
		}

		// 3. تخزين النتيجة في الكاش المزدوج
		token := generateToken()
		rawURLBytes := []byte(rawURL)
		
		// تخزين التوكن لاستخدامه في البث
		if err := cache.Set("tok:"+token, rawURLBytes); err != nil {
			return c.Status(500).JSON(fiber.Map{"status": "error", "message": "internal cache error"})
		}
		
		// تخزين الرابط لمنع التكرار مستقبلاً (أفضل جهد Best-effort، نتجاهل الخطأ إن حدث)
		_ = cache.Set(cacheKey, rawURLBytes)

		return c.JSON(fiber.Map{
			"status":         "success",
			"video_id":       videoURL,
			"download_token": token,
			"cached":         false,
		})
	})

	app.Get("/stream/:token", func(c *fiber.Ctx) error {
		token := c.Params("token")
		rawURL, err := cache.Get("tok:" + token)
		if err != nil {
			return c.Status(404).SendString("Token expired or invalid")
		}
		return c.Redirect(string(rawURL), 302)
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":         "online",
			"cores":          runtime.NumCPU(),
			"active_workers": len(extractionSemaphore),
			"version":        "2026.4.18-max-optimized",
		})
	})

	// 4. آلية الإغلاق الآمن (Graceful Shutdown)
	go func() {
		if err := app.Listen(":7860"); err != nil {
			log.Panic(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c // انتظار إشارة الإغلاق

	log.Println("🛑 Gracefully shutting down...")
	_ = app.Shutdown()
	log.Println("Fiber was successful shutdown.")
}

// دوال مساعدة
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
