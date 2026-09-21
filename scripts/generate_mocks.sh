#!/usr/bin/env bash
# generate_mocks.sh — regenerate all gomock-based mocks for astro-boilerplate.
#
# Conventions:
#   - Mocks live next to the consumer in a mocks/ sub-package named
#     "<consumer>mock" so tests in package <consumer>_test can import a
#     single namespace without alias collisions (matches astro-helpdesk-be).
#   - Source mode for first-party Go interfaces in this repo. Reflect mode is
#     reserved for proto-generated stubs whose interfaces live in external
#     packages (e.g., astro-proto). Reflect mode requires the package to
#     resolve via go mod; when astro-proto is unavailable on the build host
#     the reflect-mode generation is skipped with a warning rather than
#     failing the script.
#   - Uses `go run go.uber.org/mock/mockgen` so the mockgen version is
#     pinned by go.mod; a globally installed binary is never used.

set -euo pipefail

cd "$(dirname "$0")/.."

MOCKGEN="go run go.uber.org/mock/mockgen"

# gen_source <source-file> <dest-file> <package>
gen_source() {
  local src="$1"
  local dst="$2"
  local pkg="$3"
  mkdir -p "$(dirname "$dst")"
  $MOCKGEN -source="$src" -destination="$dst" -package="$pkg"
  echo "OK source-mode mock: $dst"
}

# gen_reflect <import-path> <interface-name> <dest-file> <package>
gen_reflect() {
  local pkgpath="$1"
  local iface="$2"
  local dst="$3"
  local pkg="$4"
  mkdir -p "$(dirname "$dst")"
  if ! go list "$pkgpath" >/dev/null 2>&1; then
    echo "SKIP reflect-mode mock: $pkgpath unavailable (private dep not resolvable on this host)"
    return 0
  fi
  $MOCKGEN -destination="$dst" -package="$pkg" "$pkgpath" "$iface"
  echo "OK reflect-mode mock: $dst"
}

echo "=== Generating vendor application mocks ==="
gen_source internal/application/vendors/repository.go   internal/application/vendors/mocks/mock_repository.go   vendorsmock
gen_source internal/application/vendors/transaction.go  internal/application/vendors/mocks/mock_transaction.go  vendorsmock
gen_source internal/application/vendors/lock.go         internal/application/vendors/mocks/mock_lock.go         vendorsmock
gen_source internal/application/vendors/notification.go internal/application/vendors/mocks/mock_notification.go vendorsmock
gen_source internal/application/vendors/audit.go        internal/application/vendors/mocks/mock_audit.go        vendorsmock
gen_source internal/application/vendors/reader.go       internal/application/vendors/mocks/mock_reader.go       vendorsmock

echo "All mocks generated successfully."
