#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
REVISION="$(git rev-parse HEAD)"
[[ -z "$(git status --porcelain)" ]] || { printf 'Build from a clean committed worktree so the image revision is reproducible.\n' >&2; exit 1; }
TAG="ticket-online-api:$REVISION"
docker build --build-arg "VCS_REF=$REVISION" -f backend/Dockerfile -t "$TAG" .
mkdir -p out
docker image save "$TAG" -o "out/ticket-online-api-$REVISION.tar"
docker image inspect --format '{{.Id}}' "$TAG" > "out/ticket-online-api-$REVISION.image-id"
printf 'Built %s\nArtifact: %s/out/ticket-online-api-%s.tar\n' "$TAG" "$ROOT" "$REVISION"
