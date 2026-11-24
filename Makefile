.PHONY: help dev build test test-backend-sqlite test-backend-postgres test-backend-postgres-full test-backend e2e check specs contracts contracts-check fresh-clone version-check

help:
	python3 scripts/workspace.py --help

dev:
	python3 scripts/workspace.py dev

build:
	python3 scripts/workspace.py build

test:
	python3 scripts/workspace.py test

test-backend-sqlite:
	DRIVEBRIDGE_TEST_DATABASE=sqlite python3 scripts/check.py --timeout 2100 -- go -C server test ./internal/service -count=1 -timeout=30m

test-backend-postgres:
	DRIVEBRIDGE_TEST_DATABASE=postgres python3 scripts/check.py --timeout 2100 -- python3 scripts/backend-postgres-test.py --run '.' --test-timeout=30m

test-backend-postgres-full:
	DRIVEBRIDGE_TEST_DATABASE=postgres python3 scripts/check.py --timeout 2100 -- python3 scripts/backend-postgres-test.py --run '.' --test-timeout=30m

test-backend: test-backend-sqlite test-backend-postgres

e2e:
	python3 scripts/workspace.py e2e

check:
	python3 scripts/workspace.py check

specs:
	openspec list

contracts:
	node web/scripts/generate-contract-types.mjs --profile all

contracts-check:
	python3 scripts/check.py -- node web/scripts/generate-contract-types.mjs --check --profile all
	python3 scripts/check.py -- node web/scripts/check-contract-schemas.mjs
	python3 scripts/check.py -- node web/scripts/check-desktop-contract.mjs

fresh-clone:
	python3 scripts/ci/fresh_clone.py

version-check:
	python3 scripts/check-version.py
