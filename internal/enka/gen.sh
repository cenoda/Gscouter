#!/bin/sh
# Writes gcsim's game data (Enka ID mappings and artifact main stat
# tables) from the gcsim module version pinned in go.mod, and the names
# from Enka.Network's store data, into the directory given, to diff
# against data/. data/ is legacy data (data/README.md) and is no longer
# overwritten from these sources: the directory must be another one.
#
#   ./gen.sh /tmp/enka-legacy && diff -r /tmp/enka-legacy data
set -eu
out="${1:?usage: gen.sh <output directory other than data>}"
mkdir -p "$out"
if [ "$(cd "$out" && pwd -P)" = "$(cd data && pwd -P)" ]; then
	echo "gen.sh: data/ is legacy data and is not overwritten (data/README.md)" >&2
	exit 1
fi
src="$(go list -m -f '{{.Dir}}' github.com/genshinsim/gcsim)/ui/packages/ui/src/data"
for f in character.dm.json weapon.dm.json artifact.dm.json artifact_main_gen.json; do
	cp "$src/$f" "$out/$f"
	chmod 644 "$out/$f"
done
# In-game names in every game language, from Enka.Network's store data.
go run ./gennames -dir "$out"
