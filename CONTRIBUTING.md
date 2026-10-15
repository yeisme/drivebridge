# Contributing to DriveBridge

DriveBridge is maintained as one repository with CLI/MCP, HTTP/GORM Server,
Web and Electron modules. Start from a clean clone and keep unrelated dirty
worktree changes out of a patch.

## Local checks

```bash
pnpm --dir web install --frozen-lockfile
make check
make contracts-check
python3 scripts/check-version.py
```

Do not run race detection unless the project owner explicitly requests it.
Do not use real cloud accounts, production credentials, paid providers or
production services in tests. Fixture evidence must be written under
`temp/integration-test-runs/<run-id>/` and must not contain secrets, tokens,
temporary URLs, raw prompts or private request bodies.

## Pull requests

Describe the user path, contract impact, test commands and known limits. Keep
experimental, fixture-only and planned capabilities labelled honestly. Changes
to authentication, authorization, storage keys, cryptography, generated
contracts or release workflows require focused regression tests.
