-- +goose Up
-- +goose StatementBegin
CREATE TABLE river_migration(
  id bigserial PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  version bigint NOT NULL,
  CONSTRAINT version CHECK (version >= 1)
);

CREATE UNIQUE INDEX ON river_migration USING btree(version);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE river_migration;
-- +goose StatementEnd
