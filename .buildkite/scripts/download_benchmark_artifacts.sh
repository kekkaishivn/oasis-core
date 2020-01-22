#! /bin/bash

####################################################
# Download artifacts needed to perform benchmarks. #
####################################################

# Helpful tips on writing build scripts:
# https://buildkite.com/docs/pipelines/writing-build-scripts
set -euxo pipefail

source .buildkite/scripts/common.sh

# Extra tools
download_artifact ba go/extra/ba 755
