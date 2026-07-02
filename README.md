# worker-golang

Backend'dagi **celery worker + beat** o'rnini bosuvchi Go servis (jst-go
konvensiyalari: pgx + sqlc + viper + zap, feature-modular tuzilma).

## Nima qiladi

**Periodik (celery beat jadvali bilan bir xil):**

| Job | Interval | Celery ekvivalenti |
|---|---|---|
| `debugsync` | */5 daq | `core.apps.api.tasks.debug.sync_debug` |
| `files` | */5 daq | `core.apps.api.tasks.file.cleanup_expired_files` |
| `releases` | */10 daq | `core.apps.api.tasks.release.cleanup_failed_releases` |
| `botsync` | soat :00 | `core.apps.api.tasks.bot.sync_bot` |
| `disksnap` | soat :00 | `core.apps.api.tasks.monitoring.snapshot_project_disk` |

**On-demand (Redis Stream `goworker:tasks`, Django `core/services/go_worker.py` XADD qiladi):**

| Task | Celery ekvivalenti |
|---|---|
| `send_message` | `core.apps.api.tasks.message.send_message` (broadcast) |
| `send_telegram_message` | `core.apps.api.tasks.chat.send_telegram_message` (CRM javob) |

Stream kontrakt: `XADD goworker:tasks * payload='{"task":"send_message","args":{"message_id":1}}'`.
Consumer group bilan o'qiladi — worker restart bo'lsa ack qilinmagan xabarlar qayta ishlanadi.

**Celery'da qolganlar (2-bosqichga):** `enable_default_modules`, `SendConfirm` (SMS),
`export_collection_to_excel` (legacy Excel), `clear_old_codes`.

## Ishga tushirish

Env (backend .env bilan bir xil nomlar): `DATABASE_URL`, `REDIS_URL`, `MONGO_URL`,
`K8S_NAMESPACE`, `STORAGE_URL`, `STORAGE_ID`, `STORAGE_KEY`, `STORAGE_REGION`,
`STORAGE_BUCKET_MEDIA`.

```bash
make build   # kompilyatsiya
make run     # lokal (kubeconfig fallback ishlaydi)
make sqlc    # queries/*.sql o'zgarganda
```

## Migratsiya rejasi

1. Worker deploy qilinadi (celery bilan YONMA-YON — periodik joblar hozircha
   ikkalasida ham yurmasin deb, avval celery beat'dagi mos yozuvlar kommentga olinadi).
2. Django'da `GO_WORKER_TASKS=true` yoqiladi — broadcast/chat tasklar stream'ga boradi.
3. Kuzatuv OK bo'lgach celery beat'dan 5 periodik job olib tashlanadi,
   worker+beat deployment'lari qisqartiriladi.

## Diqqat — schema egasi Django

`schema/schema.sql` — Django jadvallarining minimal snapshot'i (sqlc uchun).
Backend migratsiyasi ustun qo'shsa/o'zgartirsa shu faylni ham yangilang.

Farqlar (ataylab):
- `disksnap` CMS hajmini backend'dagi JSON-taxmin o'rniga Mongo `dbStats.dataSize`
  bilan hisoblaydi (aniqroq, bitta chaqiruv).
- `sync_bot` faqat RELEASE_BOT_TOKEN'i BOR loyihalarni oladi (backend hamma
  loyihani olib token yo'qlarini exception bilan tashlaydi — natija bir xil, so'rov kam).
