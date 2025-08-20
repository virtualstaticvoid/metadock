#!/bin/bash

set -euo pipefail
# set -x # debug

heading() {
  echo
  echo "==> $*"
}

topic() {
  echo
  echo "--> $*"
}

indent() {
  sed -u 's/^/    /'
}

heading "Running tests..."

topic "aws sts get-caller-identity"
aws sts get-caller-identity --no-cli-pager | indent
