-- +goose Up
-- Test table for types where sqlc generates native Go types instead of pgtype wrappers
CREATE TABLE articles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL,
    tags text[] NOT NULL,
    scores integer[] NOT NULL,
    settings jsonb NOT NULL,
    metadata json,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE articles;
