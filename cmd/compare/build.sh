#!/bin/sh
# Builds cmd/validate once per gcsim version for the regression run
# (design §5.4), into <dir>:
#   validate-fork      go.mod as it is (the gcsim fork)
#   validate-upstream  go.mod without the gcsim replace directive
#   validate-<name>    the fork at <commit>, for each <name>=<commit>
#                      argument (e.g. previous=91c5de4)
# go.mod and go.sum are not changed: the other builds use copies in
# <dir>. The cache (validate -cache) keys results by the gcsim each
# binary is built with, so the binaries can share it.
set -eu
dir=${1:?usage: cmd/compare/build.sh <dir> [name=commit ...]}
shift
cd "$(dirname "$0")/../.."
mkdir -p "$dir"
dir=$(cd "$dir" && pwd)
gcsim=github.com/genshinsim/gcsim
fork=$(go list -m -f '{{.Replace.Path}}' "$gcsim")

go build -o "$dir/validate-fork" ./cmd/validate

# variant <name> <go mod edit flag>: a build with go.mod edited.
variant() {
	cp go.mod "$dir/$1.mod"
	cp go.sum "$dir/$1.sum"
	go mod edit -modfile="$dir/$1.mod" "$2"
	go build -mod=mod -modfile="$dir/$1.mod" -o "$dir/validate-$1" ./cmd/validate
}
variant upstream -dropreplace=$gcsim
for arg in "$@"; do
	variant "${arg%%=*}" -replace=$gcsim=$fork@"${arg#*=}"
done
