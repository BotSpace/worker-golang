// worker — backend celery worker+beat o'rnini bosuvchi Go servis.
//
// Periodik (celery beat bilan bir xil jadval):
//
//	sync_debug               */5 daqiqa
//	cleanup_expired_files    */5 daqiqa
//	cleanup_failed_releases  */10 daqiqa
//	sync_bot                 har soat :00
//	snapshot_project_disk    har soat :00
//
// On-demand (Redis Stream goworker:tasks, Django web XADD qiladi):
//
//	send_message           — broadcast
//	send_telegram_message  — CRM chat javobi
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/modules/botsync"
	"github.com/BotSpace/worker-golang/internal/modules/broadcast"
	"github.com/BotSpace/worker-golang/internal/modules/chatsend"
	"github.com/BotSpace/worker-golang/internal/modules/debugsync"
	"github.com/BotSpace/worker-golang/internal/modules/disksnap"
	"github.com/BotSpace/worker-golang/internal/modules/files"
	"github.com/BotSpace/worker-golang/internal/modules/releases"
	"github.com/BotSpace/worker-golang/internal/shared/config"
	"github.com/BotSpace/worker-golang/internal/shared/database"
	"github.com/BotSpace/worker-golang/internal/shared/k8sx"
	"github.com/BotSpace/worker-golang/internal/shared/logger"
	"github.com/BotSpace/worker-golang/internal/shared/mongox"
	"github.com/BotSpace/worker-golang/internal/shared/redisq"
	"github.com/BotSpace/worker-golang/internal/shared/storagex"
)

func main() {
	log, err := logger.New()
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, q, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("postgres", zap.Error(err))
	}
	defer pool.Close()

	mongo, err := mongox.New(ctx, cfg.MongoURL)
	if err != nil {
		log.Fatal("mongo", zap.Error(err))
	}

	k8s, err := k8sx.New(cfg.K8sNamespace, log)
	if err != nil {
		log.Fatal("k8s", zap.Error(err))
	}

	storage, err := storagex.New(cfg.StorageURL, cfg.StorageID, cfg.StorageKey, cfg.StorageRegion, cfg.StorageBucketMedia)
	if err != nil {
		log.Fatal("storage", zap.Error(err))
	}

	// --- Usecase'lar ---
	debugSync := &debugsync.Usecase{Q: q, K8s: k8s, Log: log}
	fileCleanup := &files.Usecase{Q: q, Storage: storage, Log: log}
	releaseCleanup := &releases.Usecase{Q: q, K8s: k8s, Log: log}
	botSync := &botsync.Usecase{Q: q, Storage: storage, Log: log}
	diskSnap := &disksnap.Usecase{Q: q, Mongo: mongo, Log: log}
	bcast := &broadcast.Usecase{Q: q, Mongo: mongo, Storage: storage, Log: log}
	chatSend := &chatsend.Usecase{Q: q, Storage: storage, Log: log}

	// --- Cron (celery beat jadvali) ---
	c := cron.New()
	mustAdd := func(spec string, name string, fn func(context.Context)) {
		_, err := c.AddFunc(spec, func() { fn(ctx) })
		if err != nil {
			log.Fatal("cron add", zap.String("job", name), zap.Error(err))
		}
	}
	mustAdd("*/5 * * * *", "sync_debug", debugSync.Run)
	mustAdd("*/5 * * * *", "cleanup_expired_files", fileCleanup.Run)
	mustAdd("*/10 * * * *", "cleanup_failed_releases", releaseCleanup.Run)
	mustAdd("0 * * * *", "sync_bot", botSync.Run)
	mustAdd("0 * * * *", "snapshot_project_disk", diskSnap.Run)
	c.Start()
	defer c.Stop()

	// --- Redis Stream consumer (on-demand tasklar) ---
	consumer, err := redisq.NewConsumer(cfg.RedisURL, cfg.TaskStream, cfg.TaskConsumerGroup, log)
	if err != nil {
		log.Fatal("redis", zap.Error(err))
	}
	consumer.Register("send_message", bcast.Handle)
	consumer.Register("send_telegram_message", chatSend.Handle)
	// RELEASE_BOT_TOKEN o'zgarganda backend signal shu taskni enqueue qiladi.
	consumer.Register("sync_bot", func(ctx context.Context, _ json.RawMessage) error {
		botSync.Run(ctx)
		return nil
	})

	log.Info("worker started",
		zap.String("stream", cfg.TaskStream),
		zap.String("namespace", cfg.K8sNamespace))

	if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal("consumer", zap.Error(err))
	}
	log.Info("worker stopped")
}
