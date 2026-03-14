-- +goose Up
CREATE TABLE media (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    short_code     TEXT UNIQUE NOT NULL,
    type           TEXT NOT NULL,
    title          TEXT,
    description    TEXT,
    tags           TEXT[],
    status         TEXT NOT NULL DEFAULT 'processing',
    original_key   TEXT NOT NULL,
    file_size      BIGINT NOT NULL DEFAULT 0,
    mime_type      TEXT NOT NULL,
    width          INT,
    height         INT,
    duration_sec   INT,
    view_count     BIGINT NOT NULL DEFAULT 0,
    download_count BIGINT NOT NULL DEFAULT 0,
    watermarked    BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE media_files (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id  UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    variant   TEXT NOT NULL,
    s3_key    TEXT NOT NULL,
    width     INT,
    height    INT,
    file_size BIGINT,
    format    TEXT
);

CREATE INDEX idx_media_user_id    ON media(user_id);
CREATE INDEX idx_media_short_code ON media(short_code);
CREATE INDEX idx_media_status     ON media(status);
CREATE INDEX idx_media_created_at ON media(created_at DESC);
CREATE INDEX idx_media_tags       ON media USING GIN(tags);
CREATE INDEX idx_media_fts        ON media USING GIN(
    to_tsvector('english', coalesce(title,'') || ' ' || coalesce(description,''))
);
CREATE INDEX idx_media_files_media_id ON media_files(media_id);

-- +goose Down
DROP TABLE IF EXISTS media_files;
DROP TABLE IF EXISTS media;
