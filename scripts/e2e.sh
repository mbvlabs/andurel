#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

step() {
	printf '\n==> %s\n' "$*"
}

require() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "missing required command: $1" >&2
		exit 1
	fi
}

require go
require just
require jq
require curl

started_at="${SECONDS}"
tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/andurel-e2e-XXXXXX")"
app_name="e2eapp"
app_dir="${tmp_dir}/${app_name}"
cli="${tmp_dir}/bin/andurel"
app_pid=""
pg_container=""

cleanup() {
	local status=$?
	if [[ -n "${app_pid}" ]] && kill -0 "${app_pid}" 2>/dev/null; then
		kill "${app_pid}" 2>/dev/null || true
		wait "${app_pid}" 2>/dev/null || true
	fi
	if [[ -n "${pg_container}" ]]; then
		docker rm -f "${pg_container}" >/dev/null 2>&1 || true
	fi
	if [[ ${status} -eq 0 ]]; then
		rm -rf "${tmp_dir}"
	else
		echo "e2e artifacts kept in ${tmp_dir}" >&2
	fi
}
trap cleanup EXIT INT TERM

parse_database_url() {
	local url="$1"
	url="${url#postgres://}"
	url="${url#postgresql://}"
	local authority="${url%%/*}"
	local creds=""
	if [[ "${authority}" == *@* ]]; then
		creds="${authority%@*}"
		authority="${authority##*@}"
	fi
	if [[ -n "${creds}" ]]; then
		DB_USER="${creds%%:*}"
		if [[ "${creds}" == *:* ]]; then
			DB_PASSWORD="${creds#*:}"
		fi
	fi
	DB_HOST="${authority%%:*}"
	if [[ "${authority}" == *:* ]]; then
		DB_PORT="${authority##*:}"
	fi
}

assert_manifest_files() {
	local manifest="$1"
	if ! jq -e '
		.data.manifest.files as $files
		| ($files | map(.path) | any(. == "domains/invoice.sql"))
		and ($files | map(.path) | any(. == "domains/invoice.go"))
		and ($files | map(.path) | any(. == "domains/invoice_test.go"))
		and ($files | any(.path == "internal/runtime/runtime.go" and .action == "edit"))
		and ($files | map(.path) | any(endswith("create_invoices_table.sql")))
	' "${manifest}" >/dev/null; then
		echo "domain manifest is missing a required Invoice file" >&2
		jq '.data.manifest.files' "${manifest}" >&2 || true
		exit 1
	fi
}

step "building the CLI from this checkout"
mkdir -p "${tmp_dir}/bin"
GOWORK=off go build -o "${cli}" .

step "scaffolding ${app_name}"
(
	cd "${tmp_dir}"
	"${cli}" new "${app_name}"
)

step "pointing ${app_name} at the local pkg modules"
just local-replace "${app_dir}"

step "syncing project tools"
(
	cd "${app_dir}"
	"${cli}" tool sync
)

DB_KIND="${DB_KIND:-postgres}"
DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-${app_name}}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_SSL_MODE="${DB_SSL_MODE:-disable}"

if [[ -n "${DATABASE_URL:-}" ]]; then
	step "using DATABASE_URL for Postgres"
	parse_database_url "${DATABASE_URL}"
	DB_KIND=postgres
	DB_NAME="${app_name}"
	DB_SSL_MODE=disable
else
	step "starting a disposable Postgres container"
	require docker
	pg_container="andurel-e2e-pg-$$"
	docker run --rm -d --name "${pg_container}" \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=postgres \
		-p 127.0.0.1::5432 \
		postgres:17 >/dev/null
	DB_HOST=127.0.0.1
	DB_PORT="$(docker port "${pg_container}" 5432/tcp | awk -F: 'NR == 1 {print $NF}')"
	DB_NAME="${app_name}"
	DB_USER=postgres
	DB_PASSWORD=postgres
	DB_SSL_MODE=disable
	if [[ -z "${DB_PORT}" ]]; then
		echo "could not determine the Postgres host port" >&2
		exit 1
	fi
	for _ in $(seq 1 60); do
		if docker exec "${pg_container}" pg_isready -U postgres >/dev/null 2>&1; then
			break
		fi
		sleep 1
	done
	if ! docker exec "${pg_container}" pg_isready -U postgres >/dev/null 2>&1; then
		docker logs "${pg_container}" >&2 || true
		echo "Postgres did not become ready" >&2
		exit 1
	fi
fi

PORT="${PORT:-$((20000 + RANDOM % 20000))}"
HOST=127.0.0.1
HOST_PRIMARY="localhost:${PORT}"
PROTOCOL=http
ENVIRONMENT=development

step "writing ${app_dir}/.env"
grep -vE '^(DB_|HOST=|PORT=|HOST_PRIMARY=|PROTOCOL=|ENVIRONMENT=)' \
	"${app_dir}/.env.example" >"${app_dir}/.env"
{
	printf 'ENVIRONMENT=%s\n' "${ENVIRONMENT}"
	printf 'HOST=%s\n' "${HOST}"
	printf 'PORT=%s\n' "${PORT}"
	printf 'HOST_PRIMARY=%s\n' "${HOST_PRIMARY}"
	printf 'PROTOCOL=%s\n' "${PROTOCOL}"
	printf 'DB_KIND=%s\n' "${DB_KIND}"
	printf 'DB_HOST=%s\n' "${DB_HOST}"
	printf 'DB_PORT=%s\n' "${DB_PORT}"
	printf 'DB_NAME=%s\n' "${DB_NAME}"
	printf 'DB_USER=%s\n' "${DB_USER}"
	printf 'DB_PASSWORD=%s\n' "${DB_PASSWORD}"
	printf 'DB_SSL_MODE=%s\n' "${DB_SSL_MODE}"
} >>"${app_dir}/.env"

export ENVIRONMENT HOST PORT HOST_PRIMARY PROTOCOL
export DB_KIND DB_HOST DB_PORT DB_NAME DB_USER DB_PASSWORD DB_SSL_MODE

step "creating and migrating the database"
(
	cd "${app_dir}"
	"${cli}" db create
	"${cli}" db migrate up
)

step "verifying the scaffold"
verify_json="$(
	cd "${app_dir}"
	"${cli}" verify --json
)"
printf '%s\n' "${verify_json}" >"${tmp_dir}/verify.json"
if ! printf '%s\n' "${verify_json}" | jq -e '.ok == true' >/dev/null; then
	echo "andurel verify did not report ok: true" >&2
	printf '%s\n' "${verify_json}" >&2
	exit 1
fi

step "running the scaffold tests"
(
	cd "${app_dir}"
	GOWORK=off go test ./...
)

step "reading the domain manifest for Invoice"
manifest_before="$(
	cd "${app_dir}"
	"${cli}" patterns show domain --entity Invoice --table invoices --json
)"
printf '%s\n' "${manifest_before}" >"${tmp_dir}/manifest-before.json"
assert_manifest_files "${tmp_dir}/manifest-before.json"

step "creating the invoices migration"
(
	cd "${app_dir}"
	"${cli}" generate migration create_invoices_table
)
migration_file="$(ls "${app_dir}"/migrations/*_create_invoices_table.sql | head -n1)"
cat >"${migration_file}" <<'SQL'
-- +goose Up
create table invoices (
	id uuid primary key default gen_random_uuid(),
	number text not null unique,
	amount_cents bigint not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

-- +goose Down
drop table if exists invoices;
SQL

step "adding invoices queries and regenerating narsilc code"
cat >"${app_dir}/domains/invoice.sql" <<'SQL'
-- name: GetInvoice :one
SELECT *
FROM invoices
WHERE id = $1
LIMIT 1;
SQL
(
	cd "${app_dir}"
	"${cli}" db migrate up
	"${cli}" sync queries
)

step "rendering the Invoice domain from its narsilc row"
manifest_after="$(
	cd "${app_dir}"
	"${cli}" patterns show domain --entity Invoice --table invoices --json
)"
printf '%s\n' "${manifest_after}" >"${tmp_dir}/manifest-after.json"
if ! jq -e '.data.manifest.row_found == true' "${tmp_dir}/manifest-after.json" >/dev/null; then
	echo "Invoice row was not found after sync queries" >&2
	jq '.data.manifest' "${tmp_dir}/manifest-after.json" >&2 || true
	exit 1
fi
if ! jq -e '
	.data.manifest.files
	| any(.path == "domains/invoice.go" and (.content | length) > 0)
' "${tmp_dir}/manifest-after.json" >/dev/null; then
	echo "manifest did not render domains/invoice.go" >&2
	jq '.data.manifest.files' "${tmp_dir}/manifest-after.json" >&2 || true
	exit 1
fi

write_manifest_file() {
	local path="$1"
	local destination="${app_dir}/${path}"
	mkdir -p "$(dirname "${destination}")"
	jq -j --arg path "${path}" '
		.data.manifest.files[] | select(.path == $path) | .content
	' "${tmp_dir}/manifest-after.json" >"${destination}"
}
write_manifest_file "domains/invoice.sql"
write_manifest_file "domains/invoice.go"

step "regenerating narsilc code from the rendered domain SQL"
(
	cd "${app_dir}"
	"${cli}" sync queries
)

step "verifying the generated Invoice domain"
(
	cd "${app_dir}"
	"${cli}" verify --json
) >"${tmp_dir}/verify-after.json"
if ! jq -e '.ok == true' "${tmp_dir}/verify-after.json" >/dev/null; then
	echo "andurel verify failed after writing the Invoice domain" >&2
	cat "${tmp_dir}/verify-after.json" >&2
	exit 1
fi
(
	cd "${app_dir}"
	"${cli}" inspect domains --json
) >"${tmp_dir}/domains.json"
if ! jq -e '
	.data
	| map(.entities // []) | flatten
	| any(.name == "Invoice")
' "${tmp_dir}/domains.json" >/dev/null; then
	echo "andurel inspect domains did not report the Invoice domain" >&2
	cat "${tmp_dir}/domains.json" >&2
	exit 1
fi

step "installing frontend dependencies"
(
	cd "${app_dir}"
	pnpm install
)

step "building the application"
(
	cd "${app_dir}"
	"${cli}" build
)

step "starting the application and polling /health"
(
	cd "${app_dir}"
	exec "./${app_name}"
) >"${tmp_dir}/app.log" 2>&1 &
app_pid=$!

health_url="http://${HOST}:${PORT}/health"
healthy=false
for _ in $(seq 1 60); do
	if curl -fsS "${health_url}" >/dev/null 2>&1; then
		healthy=true
		break
	fi
	if ! kill -0 "${app_pid}" 2>/dev/null; then
		break
	fi
	sleep 1
done
if [[ "${healthy}" != true ]]; then
	echo "application did not answer ${health_url}" >&2
	cat "${tmp_dir}/app.log" >&2 || true
	exit 1
fi
kill "${app_pid}" 2>/dev/null || true
wait "${app_pid}" 2>/dev/null || true
app_pid=""

elapsed=$((SECONDS - started_at))
printf '\n✓ e2e passed in %ds\n' "${elapsed}"
