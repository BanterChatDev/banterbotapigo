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