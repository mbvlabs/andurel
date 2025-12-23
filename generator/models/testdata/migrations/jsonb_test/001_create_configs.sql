-- +goose Up
CREATE TABLE configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    settings jsonb NOT NULL,
    metadata json,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE configs;
