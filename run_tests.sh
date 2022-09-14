#!/bin/bash

set -e

# run tests on all modules
for i in $(find . -name go.mod -type f -print); do
  module=$(dirname ${i})
  echo "running tests and lint on ${module}"
  (cd ${module} && \
    go test && \
    golangci-lint run --deadline=10m \
      --disable-all \
      --enable govet \
      --enable staticcheck \
      --enable gosimple \
      --enable unconvert \
      --enable ineffassign \
      --enable goimports \
      --enable misspell \
  )
done
