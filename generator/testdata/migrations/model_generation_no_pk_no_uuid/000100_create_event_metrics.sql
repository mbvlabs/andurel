-- +goose Up
CREATE TABLE event_metrics (
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    event_count INTEGER NOT NULL,
    successful BOOLEAN NOT NULL,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE event_metrics;
