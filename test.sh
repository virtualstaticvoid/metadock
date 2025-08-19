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

topic "aws s3 ls"
aws s3 ls | indent

topic "aws ec2 describe-instances"
aws ec2 describe-instances --region us-east-1 | indent

# TODO: add others...?
