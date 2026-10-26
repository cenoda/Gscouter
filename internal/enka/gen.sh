#!/bin/sh
# Copies gcsim's game data (Enka ID mappings and artifact main stat tables)
# from the gcsim module version pinned in go.mod into data/.
set -eu
src="$(go list -m -f '{{.Dir}}' github.com/genshinsim/gcsim)/ui/packages/ui/src/data"
for f in character.dm.json weapon.dm.json artifact.dm.json artifact_main_gen.json; do
	cp "$src/$f" "data/$f"
	chmod 644 "data/$f"
done
