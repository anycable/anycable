#!/usr/bin/env bash
set -euo pipefail

# Conformance test dependencies.
bundle check || bundle install

# Pre-download Go modules so the first build is fast.
go mod download
