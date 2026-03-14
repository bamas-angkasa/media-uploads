-- +goose Up
CREATE TABLE reports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_id    UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    reporter_id UUID NOT NULL REFERENCES users(id),
    reason      TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reports_media_id ON reports(media_id);
CREATE INDEX idx_reports_status   ON reports(status);
CREATE INDEX idx_reports_reporter ON reports(reporter_id);

-- +goose Down
DROP TABLE IF EXISTS reports;
