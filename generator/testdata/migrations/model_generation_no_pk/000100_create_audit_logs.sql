-- +goose Up
CREATE TABLE audit_logs (
    event_id UUID NOT NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID NOT NULL,
    payload JSONB,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE audit_logs;
