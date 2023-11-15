#!/bin/bash

set -e

# run tests on all modules
for i in $(find . -name go.mod -type f -print); do
  module=$(dirname ${i})
  echo "running tests and lint on ${module}"
  (cd ${module} && \
    go test && \
    golangci-lint run
  )
done
