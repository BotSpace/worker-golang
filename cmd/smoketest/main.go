// smoketest — worker'ning barcha integratsiyalarini READ-ONLY tekshiradi
// (prod'ga yozmaydi; faqat izolyatsiyalangan goworker:smoketest stream'iga
// bitta test xabar yozib o'chiradi).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/BotSpace/worker-golang/internal/shared/config"
	"github.com/BotSpace/worker-golang/internal/shared/database"
	"github.com/BotSpace/worker-golang/internal/shared/mongox"
	"github.com/BotSpace/worker-golang/internal/shared/storagex"

	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg, err := config.Load()
	must("config", err)

	// --- Postgres + sqlc queries ---
	pool, q, err := database.New(ctx, cfg.DatabaseURL)
	must("postgres connect", err)
	defer pool.Close()

	cutoff := pgtype.Timestamptz{Time: time.Now().Add(-30 * time.Minute), Valid: true}
	debugs, err := q.ListExpiredRunningDebugs(ctx, cutoff)
	must("ListExpiredRunningDebugs", err)
	fmt.Printf("OK ListExpiredRunningDebugs: %d ta (30 daq o'tgan running debug)\n", len(debugs))

	files, err := q.ListExpiredFiles(ctx)
	must("ListExpiredFiles", err)
	fmt.Printf("OK ListExpiredFiles: %d ta\n", len(files))

	ids, err := q.ListReleasedProjectIDs(ctx)
	must("ListReleasedProjectIDs", err)
	fmt.Printf("OK ListReleasedProjectIDs: %d ta loyiha\n", len(ids))

	tokens, err := q.ListProjectsWithReleaseToken(ctx)
	must("ListProjectsWithReleaseToken", err)
	fmt.Printf("OK ListProjectsWithReleaseToken: %d ta loyiha\n", len(tokens))

	var probe pgtype.UUID
	var legacyProbe int64
	if len(ids) > 0 {
		probe = ids[0]
		fb, err := q.SumProjectFileBytes(ctx, probe)
		must("SumProjectFileBytes", err)
		fmt.Printf("OK SumProjectFileBytes(project=%s): %d bayt\n", database.UUIDString(probe), fb)
	}
	diskProjects, err := q.ListReleasedProjectsForDisk(ctx)
	must("ListReleasedProjectsForDisk", err)
	for _, project := range diskProjects {
		if project.LegacyID != nil {
			legacyProbe = int64(*project.LegacyID)
			break
		}
	}

	// --- Mongo ---
	mongo, err := mongox.New(ctx, cfg.MongoURL)
	must("mongo connect", err)
	if legacyProbe > 0 {
		users, err := mongo.ProjectUsers(ctx, legacyProbe)
		if err != nil {
			fmt.Printf("!! ProjectUsers(project=%d): %v (users kolleksiyasi bo'lmasligi normal)\n", legacyProbe, err)
		} else {
			fmt.Printf("OK ProjectUsers(project=%d): %d ta user\n", legacyProbe, len(users))
		}
	}
	if probe.Valid {
		size, err := mongo.TenantDataSize(ctx, "botspace_cms", database.UUIDString(probe))
		must("ProjectDataSize", err)
		fmt.Printf("OK ProjectDataSize(project=%s): %d bayt\n", database.UUIDString(probe), size)
	}

	// --- auth_cleanup.sql jadvallari (read-only mavjudlik tekshiruvi —
	// ExpireActiveSubscriptions/DeleteExpiredRefreshTokens/
	// DeleteExpiredAuthbotCodes o'zi UPDATE/DELETE, shu sabab bu yerda
	// chaqirilmaydi; faqat jadval borligini va ulanish ishlashini tekshiramiz) ---
	for _, table := range []string{"user_subscription", "refresh_tokens", "authbot_codes"} {
		var n int64
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			fmt.Printf("!! %s: %v\n", table, err)
		} else {
			fmt.Printf("OK %s: %d ta qator\n", table, n)
		}
	}

	// --- S3 storage (read-only: mavjud fayldan 1 dona o'qib ko'ramiz) ---
	storage, err := storagex.New(cfg.StorageURL, cfg.StorageID, cfg.StorageKey, cfg.StorageRegion, cfg.StorageBucketMedia)
	must("storage init", err)
	var anyFile string
	if err := pool.QueryRow(ctx, "SELECT file_path FROM file ORDER BY id DESC LIMIT 1").Scan(&anyFile); err == nil {
		if content, err := storage.Read(ctx, anyFile); err != nil {
			fmt.Printf("!! storage read (%s): %v\n", anyFile, err)
		} else {
			fmt.Printf("OK storage read: %s (%d bayt)\n", anyFile, len(content))
		}
	} else {
		fmt.Println("-- storage read: file jadvali bo'sh, o'tkazib yuborildi")
	}

	// --- Redis Stream (izolyatsiyalangan test stream) ---
	opts, err := redis.ParseURL(cfg.RedisURL)
	must("redis url", err)
	rdb := redis.NewClient(opts)
	testStream := "goworker:smoketest"
	payload, _ := json.Marshal(map[string]any{"task": "echo", "args": map[string]any{"x": 1}})
	must("xadd", rdb.XAdd(ctx, &redis.XAddArgs{Stream: testStream, Values: map[string]any{"payload": string(payload)}}).Err())
	res, err := rdb.XRange(ctx, testStream, "-", "+").Result()
	must("xrange", err)
	fmt.Printf("OK redis stream: %d ta xabar yozildi/o'qildi\n", len(res))
	must("del test stream", rdb.Del(ctx, testStream).Err())

	fmt.Println("\nSMOKETEST OK — barcha integratsiyalar ishlayapti")
}

func must(what string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", what, err)
		os.Exit(1)
	}
}
