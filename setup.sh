#!/bin/bash

set -euo pipefail

sedi() {
  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

toplevel=$(git rev-parse --show-toplevel)
repo=$(basename "$toplevel")
username=$(basename "$(dirname "$toplevel")")
module="github.com/${username}/${repo}"

sedi "s|github.com/pasataleo/go-template|${module}|g" go.mod main.go
sedi "s|go-template|${repo}|" main.go
sedi "s|# go-template|# ${repo}|" README.md

read -rp "Is this an executable or a library? [exe/lib] " project_type

mv .github/workflow-templates .github/workflows

if [[ "${project_type}" == "lib" ]]; then
  rm -f main.go
  rm -rf version/
  sedi 's|go build -o bin/go-template main.go|go build ./...|' Makefile
  rm .github/workflows/release-exe.yml
  mv .github/workflows/release-lib.yml .github/workflows/release.yml
else
  sedi "s|bin/go-template|bin/${repo}|" Makefile
  rm .github/workflows/release-lib.yml
  mv .github/workflows/release-exe.yml .github/workflows/release.yml
fi

cat > .git/hooks/pre-commit << 'HOOK'
#!/bin/bash
make all || exit 1

if ! git diff --quiet; then
  echo "pre-commit: files were modified by make all, please stage the changes and commit again"
  exit 1
fi
HOOK
chmod +x .git/hooks/pre-commit

rm -- "$0"
