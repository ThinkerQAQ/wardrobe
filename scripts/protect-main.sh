#!/bin/sh
set -eu
# Enforce the same server-side protection as the actual public repository.
# Must use a PR and pass both CI jobs on the latest main, squash-only configured at repo level.
# Admins must also comply; no force-push/deletion; conversations resolved.
gh api --method PUT repos/ThinkerQAQ/wardrobe/branches/main/protection --input - <<'JSON'
{"required_status_checks":{"strict":true,"contexts":["test","windows-installer"]},"enforce_admins":true,"required_pull_request_reviews":{"dismiss_stale_reviews":true,"required_approving_review_count":0},"restrictions":null,"required_linear_history":true,"allow_force_pushes":false,"allow_deletions":false,"required_conversation_resolution":true}
JSON
# Only squash merges are allowed; successful PR heads are automatically deleted.
gh repo edit ThinkerQAQ/wardrobe --enable-squash-merge --disable-merge-commit --disable-rebase-merge --delete-branch-on-merge
