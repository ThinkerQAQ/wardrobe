# Contributing

[简体中文](CONTRIBUTING_ZH.md) · [Home](README.md)

Contributions are welcome when they respect the local-first, privacy-preserving scope of Wardrobe. Please open a feature or bugfix branch and submit a pull request. Keep main releasable; **never force-push to or delete main**.

## Development checklist

- Run **go test ./...**, **go vet ./...**, and **node --check web/app.js** plus **node --check web/lookbook.js**.
- Check the Windows cross-compile and both Linux/Windows installation smoke tests when changing setup scripts.
- Keep GitHub Actions passing before merging. Prefer squash merges and delete merged feature branches only.
- Keep application settings in TOML, business records in SQLite, and photos on local storage referenced by SQLite.
- Never submit Data/, real orders, database files, personal photos, API keys, Tunnel IDs, browser cookies, source snapshots, or unredacted logs.
- Examples must be wholly synthetic — do not disguise real personal purchase data as an example.
- Skills define reusable operations, not actual user profile details.

## Branch protection

Use scripts/protect-main.sh to configure public repository pull-request rules and prevent destructive changes to main. Check current CI before making a check required. Git hooks may prevent accidental direct pushes but are not a substitute for server-side rules.

## Documentation conventions

**English is the default.** README.md is English, README_ZH.md is Simplified Chinese; docs use the default English file name and a **.zh-CN.md** counterpart. Keep both updated for functional changes. GitHub repository summary is a concise English description with a separate Chinese line where space permits.

All external contributors must protect private local information. Security issues should be reported privately as described in [Security](SECURITY.md).
Security changes: run the CI Go vulnerability scan and preserve all required GitHub branch protections. Do not bypass checks for dependency updates.
