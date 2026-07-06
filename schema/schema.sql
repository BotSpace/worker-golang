-- Django (backend) boshqaradigan jadvallarning sqlc uchun MINIMAL snapshot'i.
-- Bu migratsiya EMAS — schema egasi Django (backend repo). Ustun qo'shilsa shu
-- faylni yangilab `make sqlc` qilinadi.

CREATE TABLE users (
    id uuid PRIMARY KEY
);

CREATE TABLE projects (
    id uuid PRIMARY KEY,
    uuid uuid NOT NULL,
    name text NOT NULL,
    username text,
    avatar text,
    avatar_file_id text,
    owner_id uuid NOT NULL REFERENCES users (id),
    flow jsonb NOT NULL DEFAULT '[]'::jsonb,
    is_template boolean NOT NULL DEFAULT false,
    legacy_id integer,
    deleted_at timestamptz
);

CREATE TABLE debug (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    flow_hash text,
    service_id text,
    status text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE relase (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    version integer NOT NULL,
    flow jsonb NOT NULL DEFAULT '[]'::jsonb,
    service_id text,
    status text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE enviroment (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    user_id uuid NOT NULL REFERENCES users (id),
    key text NOT NULL,
    value text NOT NULL,
    secret boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE file (
    id uuid PRIMARY KEY,
    uuid uuid NOT NULL,
    project_id uuid NOT NULL REFERENCES projects (id),
    user_id uuid NOT NULL REFERENCES users (id),
    file_path text NOT NULL,
    original_name text NOT NULL DEFAULT '',
    filesize bigint NOT NULL DEFAULT 0,
    filetype text NOT NULL DEFAULT 'file',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    expires_at timestamptz
);

CREATE TABLE message (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    caption text NOT NULL DEFAULT '',
    message_type text NOT NULL DEFAULT 'text',
    file_id text,
    button_type text NOT NULL DEFAULT 'reply',
    buttons jsonb NOT NULL,
    total bigint NOT NULL DEFAULT 0,
    done bigint NOT NULL DEFAULT 0,
    fail bigint NOT NULL DEFAULT 0,
    status text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE collection (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    name text NOT NULL,
    slug text NOT NULL,
    system boolean NOT NULL DEFAULT false,
    deleted_at timestamptz
);

CREATE TABLE project_resource_snapshot (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL REFERENCES projects (id),
    disk_bytes bigint NOT NULL DEFAULT 0,
    disk_files_bytes bigint NOT NULL DEFAULT 0,
    disk_cms_bytes bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- ===================== backend-golang (UUID-PK) jadvallari =====================
-- Yuqoridagi jadvallar Django'ning eski bigint-PK sxemasi (schema owner:
-- Django/backend repo). Quyidagilar esa faqat backend-golang'da mavjud —
-- Django'da hech qachon bo'lmagan (net-new Go-side auth/plan janitorlari,
-- backend-golang/internal/server/scheduler.go'dan worker'ga ko'chirildi).
-- FK REFERENCES atayin qo'shilmagan (users/plan/authbot_bot_users jadvallari
-- bu snapshot'da yo'q) — sqlc'ga faqat ustun turlari kerak.

CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);

CREATE TABLE user_subscription (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL,
    plan_id    uuid NOT NULL,
    status     text NOT NULL,
    expires_at timestamptz,
    updated_at timestamptz NOT NULL
);

CREATE TABLE authbot_codes (
    id          uuid PRIMARY KEY,
    bot_user_id uuid NOT NULL,
    expires_at  timestamptz NOT NULL
);
