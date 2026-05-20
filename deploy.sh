#!/usr/bin/env bash
set -e

read -p "Commit message: " MSG
if [ -z "$MSG" ]; then
    echo "empty message, aborting"
    exit 1
fi

go vet ./...

git add -A
git status

git commit -m "$MSG"
git push

git fetch --tags

LATEST=$(git tag -l 'v*' | sort -V | tail -n 1)
if [ -z "$LATEST" ]; then
    NEW="v0.1.0"
else
    MAJOR=$(echo "$LATEST" | sed -E 's/^v([0-9]+)\.([0-9]+)\.([0-9]+)$/\1/')
    MINOR=$(echo "$LATEST" | sed -E 's/^v([0-9]+)\.([0-9]+)\.([0-9]+)$/\2/')
    PATCH=$(echo "$LATEST" | sed -E 's/^v([0-9]+)\.([0-9]+)\.([0-9]+)$/\3/')
    NEW="v${MAJOR}.${MINOR}.$((PATCH + 1))"
fi

if git rev-parse "$NEW" >/dev/null 2>&1; then
    echo "tag $NEW already exists, aborting"
    exit 1
fi

git tag "$NEW"
git push origin "$NEW"
echo "tagged $NEW"