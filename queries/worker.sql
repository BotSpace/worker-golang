-- name: ListExpiredRunningDebugs :many
SELECT d.id, d.service_id
FROM debug d
JOIN project p ON p.id = d.project_id
WHERE d.status = 'running'
  AND d.updated_at <= $1
  AND p.is_template = false;

-- name: MarkDebugStopping :exec
UPDATE debug SET status = 'stopping', updated_at = now() WHERE id = $1;

-- name: ListExpiredFiles :many
SELECT id, file FROM file WHERE expires_at IS NOT NULL AND expires_at < now();

-- name: DeleteFile :exec
DELETE FROM file WHERE id = $1;

-- name: MarkReleaseFailedByService :execrows
UPDATE relase SET status = 'failed' WHERE service_id = $1;

-- name: MarkDebugFailedByService :execrows
UPDATE debug SET status = 'failed' WHERE service_id = $1;

-- name: ListProjectsWithReleaseToken :many
SELECT p.id, p.username, p.avatar, p.avatar_file_id, e.value AS bot_token
FROM project p
JOIN enviroment e ON e.project_id = p.id AND e.key = 'RELEASE_BOT_TOKEN'
WHERE p.deleted_at IS NULL;

-- name: UpdateProjectBotInfo :exec
UPDATE project SET username = $2, avatar = $3, avatar_file_id = $4 WHERE id = $1;

-- name: ListReleasedProjectIDs :many
-- Reliz QILINGAN yoki debug ishlatilgan loyihalar — tokensiz (debug-only)
-- botlarning ham disk charti chiqishi kerak.
SELECT DISTINCT p.id
FROM project p
LEFT JOIN relase r ON r.project_id = p.id
LEFT JOIN debug d ON d.project_id = p.id
WHERE p.deleted_at IS NULL
  AND (r.id IS NOT NULL OR d.id IS NOT NULL);

-- name: SumProjectFileBytes :one
SELECT COALESCE(SUM(filesize), 0)::bigint FROM file WHERE project_id = $1;

-- name: InsertDiskSnapshot :exec
INSERT INTO api_projectresourcesnapshot (project_id, disk_bytes, disk_files_bytes, disk_cms_bytes, created_at)
VALUES ($1, $2, $3, $4, now());

-- name: GetMessage :one
SELECT id, project_id, caption, message_type, file, file_id, button_type, buttons, total, done, fail, status
FROM message WHERE id = $1;

-- name: UpdateMessageTotal :exec
UPDATE message SET total = $2 WHERE id = $1;

-- name: UpdateMessageFileID :exec
UPDATE message SET file_id = $2 WHERE id = $1;

-- name: UpdateMessageResult :exec
UPDATE message SET done = $2, fail = $3, status = $4 WHERE id = $1;

-- name: MarkMessageFailed :exec
UPDATE message SET status = 'failed' WHERE id = $1;

-- name: HasUsersCollection :one
SELECT EXISTS (SELECT 1 FROM collection WHERE project_id = $1 AND name = 'users');

-- name: GetProjectEnv :one
SELECT value FROM enviroment WHERE project_id = $1 AND key = $2;

-- name: GetFilePath :one
SELECT file FROM file WHERE id = $1;
