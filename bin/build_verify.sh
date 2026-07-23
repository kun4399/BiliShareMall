#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_NAME="${APP_NAME:-BiliShareMall}"
BUILD_TAGS="${BUILD_TAGS:-fts5}"

SKIP_INSTALL=0
SKIP_TESTS=0
SKIP_BUILD=0
FORCE_NSIS=0

log() {
  printf '[build-verify] %s\n' "$*"
}

die() {
  printf '[build-verify] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: ./bin/build_verify.sh [options]

Options:
  --skip-install   Skip frontend dependency install
  --skip-tests     Skip go test verification
  --skip-build     Skip wails build
  --nsis           Force adding -nsis when running wails build
  -h, --help       Show this help

Environment variables:
  APP_NAME         App name for output checks (default: BiliShareMall)
  BUILD_TAGS       Wails build tags (default: fts5)
  GOCACHE          Go build cache path (default: <repo>/.cache/go-build)
EOF
}

while (($# > 0)); do
  case "$1" in
    --skip-install) SKIP_INSTALL=1 ;;
    --skip-tests) SKIP_TESTS=1 ;;
    --skip-build) SKIP_BUILD=1 ;;
    --nsis) FORCE_NSIS=1 ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown option: $1"
      ;;
  esac
  shift
done

cd "$ROOT_DIR"

for cmd in go pnpm; do
  command -v "$cmd" >/dev/null 2>&1 || die "Required command not found: $cmd"
done
if ((SKIP_BUILD == 0)); then
  command -v wails >/dev/null 2>&1 || die "Required command not found: wails"
fi

export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
mkdir -p "$GOCACHE"

if ((SKIP_INSTALL == 0)); then
  log "Installing frontend dependencies"
  pnpm --dir frontend install --frozen-lockfile
else
  log "Skipping frontend dependency install"
fi

if ((SKIP_TESTS == 0)); then
  log "Running backend verification tests: go test ./..."
  go test ./...
  log "Running frontend type checks and unit tests"
  pnpm --dir frontend run typecheck
  pnpm --dir frontend run test:unit
else
  log "Skipping tests"
fi

log "Building production web assets"
pnpm --dir frontend run build

entry_asset="$(sed -n 's/.*src="\/assets\/\([^"]*\.js\)".*/\1/p' frontend/dist/index.html | head -n 1)"
[[ -n "$entry_asset" ]] || die "Unable to resolve frontend entry asset"
entry_gzip_bytes="$(gzip -c "frontend/dist/assets/$entry_asset" | wc -c | tr -d ' ')"
entry_gzip_budget="${ENTRY_GZIP_BUDGET_BYTES:-350000}"
if ((entry_gzip_bytes > entry_gzip_budget)); then
  die "Frontend entry gzip size ${entry_gzip_bytes}B exceeds budget ${entry_gzip_budget}B"
fi
log "Frontend entry gzip size: ${entry_gzip_bytes}B / ${entry_gzip_budget}B"

if ((SKIP_BUILD == 0)); then
  log "Building app with Wails"
  wails_args=(build -tags "$BUILD_TAGS")

  if ((FORCE_NSIS == 1)); then
    wails_args+=(-nsis)
  elif [[ "${OS:-}" == "Windows_NT" ]]; then
    wails_args+=(-nsis)
  fi

  wails "${wails_args[@]}"
else
  log "Skipping Wails build"
fi

if ((SKIP_BUILD == 0)); then
  artifact_path=""
  if [[ "${OS:-}" == "Windows_NT" ]]; then
    artifact_path="$ROOT_DIR/build/bin/${APP_NAME}.exe"
  else
    case "$(uname -s)" in
      Darwin) artifact_path="$ROOT_DIR/build/bin/${APP_NAME}.app" ;;
      Linux) artifact_path="$ROOT_DIR/build/bin/${APP_NAME}" ;;
      *) die "Unsupported OS for artifact verification: $(uname -s)" ;;
    esac
  fi

  [[ -e "$artifact_path" ]] || die "Build artifact not found: $artifact_path"
  log "Artifact: $artifact_path"
fi

log "Build and verification completed successfully"
