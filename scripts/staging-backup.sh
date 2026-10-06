#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
: "${S3_URI:?Set S3_URI to the dedicated staging backup prefix}"
: "${AGE_RECIPIENT:?Set AGE_RECIPIENT to the off-host age recipient}"
: "${AWS_PROFILE:?Set AWS_PROFILE for the staging backup bucket}"
[[ -f .env.staging ]] || { printf 'Missing .env.staging\n' >&2; exit 1; }
compose=(docker compose --env-file .env.staging -f compose.staging.yaml)
container="$("${compose[@]}" ps -q api)"
if [[ -n "$container" ]]; then
	image="$(docker inspect --format '{{.Config.Image}}' "$container")"
	image_id="$(docker inspect --format '{{.Image}}' "$container")"
	revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$image_id")"
else
	: "${STAGING_IMAGE:?Set STAGING_IMAGE for the initial staging backup}"
	docker image inspect "$STAGING_IMAGE" >/dev/null
	image="$STAGING_IMAGE"
	image_id="$(docker image inspect --format '{{.Id}}' "$image")"
	revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$image")"
fi
schema_version="$("${compose[@]}" exec -T db sh -ec 'exec mysql --defaults-extra-file=/run/secrets/backup.cnf "$MYSQL_DATABASE" -Nse "SELECT COALESCE(MAX(version), 0) FROM schema_migrations"')"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
name="ticket-online-staging-$timestamp"
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT
blob="$temporary/$name.sql.gz.age"
# Deliberately excludes routines, events, and triggers; add matching grants and round-trip coverage if migrations introduce them.
"${compose[@]}" exec -T db sh -ec 'exec mysqldump --defaults-extra-file=/run/secrets/backup.cnf --single-transaction --skip-routines --skip-events --skip-triggers --hex-blob --no-tablespaces "$MYSQL_DATABASE"' | gzip -c | age -r "$AGE_RECIPIENT" > "$blob"
checksum="$(sha256sum "$blob" | cut -d ' ' -f 1)"
printf '%s  %s\n' "$checksum" "$name.sql.gz.age" > "$temporary/$name.sha256"
printf '{"createdAt":"%s","schemaVersion":%s,"revision":"%s","image":"%s","imageId":"%s","sha256":"%s"}\n' \
	"$timestamp" "$schema_version" "$revision" "$image" "$image_id" "$checksum" > "$temporary/$name.json"
aws_args=()
if [[ -n "${S3_ENDPOINT_URL:-}" ]]; then aws_args+=(--endpoint-url "$S3_ENDPOINT_URL"); fi
aws s3 cp "${aws_args[@]}" "$blob" "$S3_URI/$name.sql.gz.age" --sse AES256
aws s3 cp "${aws_args[@]}" "$temporary/$name.sha256" "$S3_URI/$name.sha256" --sse AES256
aws s3 cp "${aws_args[@]}" "$temporary/$name.json" "$S3_URI/$name.json" --sse AES256
printf 'Encrypted backup uploaded: %s/%s.sql.gz.age\n' "$S3_URI" "$name"
