#! /bin/bash

set -ux

# Script invoked from .buildkite/benchmarks.pipeline.yml

./go/extra/ba/ba cmp --metrics.address $METRICS_QUERY_ADDR --metrics.target.git_branch $METRICS_TARGET_GIT_BRANCH --metrics.source.git_branch $METRICS_SOURCE_GIT_BRANCH -t $TESTS 2>error.txt 1>out.txt
BA_RETURN_CODE=$?

# Escape double quotes for JSON.
BA_STDOUT=`cat out.txt | sed "s/\"/\\\\\\\\\"/g"`
BA_STDERR=`cat error.txt | sed "s/\"/\\\\\\\\\"/g"`

if [ $BA_RETURN_CODE != 0 ]; then
  # Post error to slack channel.
  curl -H "Content-Type: application/json" \
       -X POST \
        --data "{\"text\": \"Daily oasis-core benchmarks failed.\", \"attachments\":[{\"title\":\"stdout\",\"text\":\"$BA_STDOUT\"}, {\"title\":\"stderr\",\"text\":\"$BA_STDERR\"}]}" \
        "$SLACK_WEBHOOK_URL"

    # Exit with non-zero exit code, so that the buildkite build will be
    # marked as failed.
    exit 1
fi
