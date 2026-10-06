#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
: "${STAGING_IMAGE:?Set STAGING_IMAGE to the loaded commit-tagged image}"
: "${STAGING_DOMAIN:?Set STAGING_DOMAIN to the public HTTPS staging domain}"
[[ -f .env.staging ]] || { printf 'Missing .env.staging\n' >&2; exit 1; }
compose=(docker compose --env-file .env.staging -f compose.staging.yaml)
docker compose --env-file .env.staging -f compose.staging.yaml config -q
docker image inspect "$STAGING_IMAGE" >/dev/null
"${compose[@]}" up -d --wait db
if [[ -n "$("${compose[@]}" ps --status running -q api)" ]]; then
	./scripts/staging-backup.sh
fi
"${compose[@]}" stop https api
"${compose[@]}" run --rm --no-deps --entrypoint /app/ticket-migrate api
"${compose[@]}" up -d api https
for attempt in {1..20}; do
	if curl --fail --silent --show-error --output /dev/null "https://$STAGING_DOMAIN/api/v1/ready"; then
		printf 'Staging is ready at https://%s\n' "$STAGING_DOMAIN"
		exit 0
	fi
	sleep 3
done
printf 'Staging readiness check failed; inspect docker compose logs.\n' >&2
exit 1
