.PHONY: help dev build test e2e check specs contracts contracts-check

help:
	python3 scripts/workspace.py --help

dev:
	python3 scripts/workspace.py dev

build:
	python3 scripts/workspace.py build

test:
	python3 scripts/workspace.py test

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
