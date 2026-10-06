#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT
mkdir -p "$temporary/bin" "$temporary/repo/scripts"
cp "$ROOT/scripts/staging-backup.sh" "$temporary/repo/scripts/"
touch "$temporary/repo/.env.staging" "$temporary/repo/compose.staging.yaml"
cat > "$temporary/bin/docker" <<'SH'
#!/usr/bin/env bash
printf 'docker %s\n' "$*" >> "$CALL_LOG"
case " $* " in
  *" ps -q api "*) exit 0 ;;
  *"SELECT COALESCE(MAX(version), 0) FROM schema_migrations"*) printf '14\n'; exit 0 ;;
  *mysqldump*)
    if [[ "${MOCK_DUMP_FAIL:-0}" == 1 ]]; then printf 'dump failed\n' >&2; exit 2; fi
    printf 'sql dump'
    exit 0
    ;;
  *"image inspect --format {{.Id}}"*) printf 'sha256:test-image\n'; exit 0 ;;
  *"image inspect --format {{index .Config.Labels"*) printf 'test-revision\n'; exit 0 ;;
esac
exit 0
SH
cat > "$temporary/bin/gzip" <<'SH'
#!/usr/bin/env bash
cat
SH
cat > "$temporary/bin/age" <<'SH'
#!/usr/bin/env bash
cat >/dev/null
printf 'encrypted dump'
SH
cat > "$temporary/bin/aws" <<'SH'
#!/usr/bin/env bash
printf 'aws %s\n' "$*" >> "$CALL_LOG"
SH
chmod +x "$temporary/repo/scripts/staging-backup.sh" "$temporary/bin/"*

if CALL_LOG="$temporary/failure.log" PATH="$temporary/bin:$PATH" STAGING_IMAGE=test-image S3_URI=s3://test/backups \
	AGE_RECIPIENT=age1test AWS_PROFILE=test MOCK_DUMP_FAIL=1 bash "$temporary/repo/scripts/staging-backup.sh" >/dev/null 2>&1; then
	exit 1
fi
! grep -q '^aws ' "$temporary/failure.log"

CALL_LOG="$temporary/success.log" PATH="$temporary/bin:$PATH" STAGING_IMAGE=test-image S3_URI=s3://test/backups \
	AGE_RECIPIENT=age1test AWS_PROFILE=test MOCK_DUMP_FAIL=0 bash "$temporary/repo/scripts/staging-backup.sh" >/dev/null
grep -q -- '--single-transaction --skip-routines --skip-events --skip-triggers --hex-blob --no-tablespaces' "$temporary/success.log"
[[ "$(grep -c '^aws ' "$temporary/success.log")" == 3 ]]
printf 'staging backup dump flags and failure checks passed\n'
