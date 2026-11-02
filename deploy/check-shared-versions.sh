#!/usr/bin/env bash
# Lists every module whose packages are compiled into both Bifrost and the
# plugin at different versions, and fails when there is one. A Go plugin loads
# only when each package it shares with the host binary was built from the same
# version, so a mismatch would otherwise surface as a plugin load error when
# Bifrost starts. Modules that only appear in a module graph, without packages
# in both binaries, cannot cause a load error and are not compared.
#
# Usage: check-shared-versions.sh <bifrost main package dir> <plugin main package dir>
set -euo pipefail

if [ "$#" -ne 2 ]; then
	echo "usage: $0 <bifrost main package dir> <plugin main package dir>" >&2
	exit 2
fi

linked_modules() {
	(cd "$1" && GOWORK=off CGO_ENABLED=1 go list -deps -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' .) | LC_ALL=C sort -u -k1,1
}

# Plain assignments, so that set -e stops the script when either listing fails.
bifrost=$(linked_modules "$1")
plugin=$(linked_modules "$2")
mismatches=$(LC_ALL=C join <(echo "$bifrost") <(echo "$plugin") | awk 'NF == 3 && $2 != $3 { print $1 ": Bifrost " $2 ", plugin " $3 }')

if [ -n "$mismatches" ]; then
	echo "The plugin and Bifrost compile these modules at different versions:" >&2
	echo "$mismatches" >&2
	echo "Align them in the plugin's go.mod with: go get <module>@<Bifrost's version>" >&2
	exit 1
fi
echo "The plugin and Bifrost compile every shared module at the same version."
