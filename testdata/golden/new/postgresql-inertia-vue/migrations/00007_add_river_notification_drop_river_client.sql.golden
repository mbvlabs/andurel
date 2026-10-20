-- +goose Up
-- +goose StatementBegin
--
-- Notification outbox.
--

CREATE TABLE river_notification (
    id bigserial PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    payload text NOT NULL,
    topic text NOT NULL,
    CONSTRAINT topic_length CHECK (length(topic) > 0 AND length(topic) < 128)
);

CREATE INDEX river_notification_created_at_idx ON river_notification (created_at);
CREATE INDEX river_notification_topic_id_idx ON river_notification (topic, id);

--
-- SQLite JSONB conversion.
--
-- No-op. PostgreSQL already stores River JSON columns as jsonb.

--
-- SQL cleanup.
--

--
-- Drop unused tables `river_client` and `river_client_queue`.
--

DROP TABLE river_client_queue;
DROP TABLE river_client;

--
-- Adds `DEFAULT 25` to `river_job.max_attempts`.
--

ALTER TABLE river_job
    ALTER COLUMN max_attempts SET DEFAULT 25;

--
-- Changes `river_queue.updated_at` to have a default of `CURRENT_TIMESTAMP`.
--

ALTER TABLE river_queue
    ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
--
-- SQL cleanup rollback.
--

--
-- Add back unused tables `river_client` and `river_client_queue`.
--

CREATE UNLOGGED TABLE river_client (
    id text PRIMARY KEY NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}',
    paused_at timestamptz,
    updated_at timestamptz NOT NULL,
    CONSTRAINT name_length CHECK (char_length(id) > 0 AND char_length(id) < 128)
);

CREATE UNLOGGED TABLE river_client_queue (
    river_client_id text NOT NULL REFERENCES river_client (id) ON DELETE CASCADE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    max_workers bigint NOT NULL DEFAULT 0,
    metadata jsonb NOT NULL DEFAULT '{}',
    num_jobs_completed bigint NOT NULL DEFAULT 0,
    num_jobs_running bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (river_client_id, name),
    CONSTRAINT name_length CHECK (char_length(name) > 0 AND char_length(name) < 128),
    CONSTRAINT num_jobs_completed_zero_or_positive CHECK (num_jobs_completed >= 0),
    CONSTRAINT num_jobs_running_zero_or_positive CHECK (num_jobs_running >= 0)
);

--
-- Revert addition of `DEFAULT 25` to `river_job.max_attempts`.
--

ALTER TABLE river_job
    ALTER COLUMN max_attempts DROP DEFAULT;

--
-- Changes `river_queue.updated_at` to revert the default of `CURRENT_TIMESTAMP`.
--

ALTER TABLE river_queue
    ALTER COLUMN updated_at DROP DEFAULT;

--
-- SQLite JSONB conversion rollback.
--
-- No-op. PostgreSQL already stores River JSON columns as jsonb.

--
-- Notification outbox rollback.
--

DROP TABLE river_notification;
-- +goose StatementEnd
