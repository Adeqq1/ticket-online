#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
: "${RESTORE_PROJECT_NAME:?Set a fresh RESTORE_PROJECT_NAME beginning ticket-online-staging-restore-}"
: "${S3_URI:?Set S3_URI to the staging backup prefix}"
: "${AGE_IDENTITY_FILE:?Set AGE_IDENTITY_FILE to the operator-only age identity}"
: "${AWS_PROFILE:?Set AWS_PROFILE for the staging backup bucket}"
[[ "$RESTORE_PROJECT_NAME" =~ ^ticket-online-staging-restore-[a-z0-9-]+$ ]] || { printf 'Invalid isolated restore project name\n' >&2; exit 1; }
[[ -f .env.staging.restore && -f "$AGE_IDENTITY_FILE" ]] || { printf 'Missing restore env or age identity file\n' >&2; exit 1; }
backup_name="${1:?Usage: staging-restore.sh ticket-online-staging-YYYYMMDDTHHMMSSZ}"
[[ "$backup_name" =~ ^ticket-online-staging-[0-9]{8}T[0-9]{6}Z$ ]] || { printf 'Invalid backup name\n' >&2; exit 1; }
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT
aws_args=()
if [[ -n "${S3_ENDPOINT_URL:-}" ]]; then aws_args+=(--endpoint-url "$S3_ENDPOINT_URL"); fi
aws s3 cp "${aws_args[@]}" "$S3_URI/$backup_name.sql.gz.age" "$temporary/backup.age"
aws s3 cp "${aws_args[@]}" "$S3_URI/$backup_name.sha256" "$temporary/backup.sha256"
expected="$(cut -d ' ' -f 1 < "$temporary/backup.sha256")"
[[ "$expected" =~ ^[0-9a-f]{64}$ ]] || { printf 'Invalid checksum manifest\n' >&2; exit 1; }
actual="$(sha256sum "$temporary/backup.age" | cut -d ' ' -f 1)"
[[ "$actual" == "$expected" ]] || { printf 'Backup checksum mismatch\n' >&2; exit 1; }
compose=(docker compose --env-file .env.staging.restore -f compose.restore.yaml --project-name "$RESTORE_PROJECT_NAME")
volume="${RESTORE_PROJECT_NAME}_restore_data"
if docker volume inspect "$volume" >/dev/null 2>&1; then printf 'Restore volume already exists; choose a fresh project name\n' >&2; exit 1; fi
"${compose[@]}" up -d --wait db
age --decrypt -i "$AGE_IDENTITY_FILE" "$temporary/backup.age" | gzip -dc | "${compose[@]}" exec -T db sh -ec 'exec mysql --defaults-extra-file=/run/secrets/restore.cnf ticket_online_staging_restore'
printf 'Restored into isolated project %s. Verify schema, row counts, relations, and sample tickets before recording the restore duration.\n' "$RESTORE_PROJECT_NAME"
