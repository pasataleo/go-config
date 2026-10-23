#!/bin/bash

# Prints the release notes for the given version from CHANGELOG.md: the lines
# under its "# <version>" heading, up to the next version heading, with HTML
# comments removed. Fails if the section is missing or has no notes.

set -euo pipefail

version=$1

notes=$(awk -v heading="# ${version}" '
  $0 == heading { found = 1; next }
  found && /^# / { exit }
  !found { next }
  /<!--/ { comment = 1 }
  !comment { print }
  /-->/ { comment = 0 }
' CHANGELOG.md)

if ! grep -q '[^[:space:]]' <<< "${notes}"; then
  echo "CHANGELOG.md has no notes for ${version}, expected an uncommented section under \"# ${version}\"" >&2
  exit 1
fi

printf '%s\n' "${notes}"
