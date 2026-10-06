#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT
mkdir -p "$temporary/bin" "$temporary/repo/scripts"
cp "$ROOT/scripts/staging-deploy.sh" "$temporary/repo/scripts/"
touch "$temporary/repo/.env.staging" "$temporary/repo/compose.staging.yaml"
cat > "$temporary/repo/scripts/staging-backup.sh" <<'SH'
#!/usr/bin/env bash
printf 'backup\n' >> "$CALL_LOG"
[[ "${MOCK_BACKUP_FAIL:-0}" != 1 ]]
SH
cat > "$temporary/bin/docker" <<'SH'
#!/usr/bin/env bash
printf 'docker %s\n' "$*" >> "$CALL_LOG"
case " $* " in
  *" ps --status running -q api "*)
    [[ "$MOCK_API_STATE" != running ]] || printf 'api-container\n'
    exit 0
    ;;
  *" exec -T db sh -ec "*)
    [[ "${MOCK_DB_CHECK_FAIL:-0}" != 1 ]] || exit 42
    printf '%s' "$MOCK_SCHEMA_OBJECTS"
    ;;
esac
exit 0
SH
cat > "$temporary/bin/curl" <<'SH'
#!/usr/bin/env bash
exit 0
SH
chmod +x "$temporary/repo/scripts/staging-deploy.sh" "$temporary/repo/scripts/staging-backup.sh" "$temporary/bin/docker" "$temporary/bin/curl"

run_deploy() {
	local name="$1" tables="$2" api_state="$3" db_failure="${4:-0}" backup_failure="${5:-0}"
	CALL_LOG="$temporary/$name.log" PATH="$temporary/bin:$PATH" STAGING_IMAGE=test-image STAGING_DOMAIN=staging.example \
		MOCK_SCHEMA_OBJECTS="$tables" MOCK_API_STATE="$api_state" MOCK_DB_CHECK_FAIL="$db_failure" MOCK_BACKUP_FAIL="$backup_failure" \
		bash "$temporary/repo/scripts/staging-deploy.sh" >/dev/null 2>&1
}

assert_backup_before_migration() {
	local log="$1" backup_line migration_line
	backup_line="$(grep -n '^backup$' "$log" | cut -d: -f1)"
	migration_line="$(grep -n 'ticket-migrate' "$log" | cut -d: -f1)"
	[[ -n "$backup_line" && -n "$migration_line" && "$backup_line" -lt "$migration_line" ]]
}

for api_state in running stopped absent; do
	run_deploy "existing-$api_state" 12 "$api_state"
	assert_backup_before_migration "$temporary/existing-$api_state.log"
done

run_deploy initial-install 0 absent
! grep -q '^backup$' "$temporary/initial-install.log"
grep -q 'ticket-migrate' "$temporary/initial-install.log"

if run_deploy database-check-fails 0 absent 1; then exit 1; fi
! grep -q '^backup$\|ticket-migrate' "$temporary/database-check-fails.log"

if run_deploy backup-fails 12 stopped 0 1; then exit 1; fi
grep -q '^backup$' "$temporary/backup-fails.log"
! grep -q 'ticket-migrate' "$temporary/backup-fails.log"

printf 'staging deploy backup ordering checks passed\n'
