#!/bin/bash -e

ekiden_image=${EKIDEN_DOCKER_IMAGE:-ekiden/development:0.1.0-alpha.4}
base_dir=$( cd "$( dirname "${BASH_SOURCE[0]}" )/../.." && pwd )
build_sgx_mode=${BUILD_IMAGES_SGX_MODE:-SIM}
if [ "$build_sgx_mode" = SIM ]; then
    build_skip_avr_verify_flag="-e EKIDEN_SKIP_AVR_VERIFY=1"
else
    build_skip_avr_verify_flag=""
fi

cd ${base_dir}

if [ -n "$BUILD_IMAGES_NO_ENTER" ]; then
    ./docker/deployment/build-images-inner.sh
elif [ -z "$BUILD_IMAGES_CONTAINER" ]; then
    # Build in a fresh container.
    docker run --rm \
        -v "$PWD:/code" \
        -e "SGX_MODE=$build_sgx_mode" \
        -e INTEL_SGX_SDK=/opt/sgxsdk \
        $build_skip_avr_verify_flag \
        -w /code \
        "$ekiden_image" \
        /code/docker/deployment/build-images-inner.sh
else
    # Build in a specified container.
    docker exec "$BUILD_IMAGES_CONTAINER" \
        /code/docker/deployment/build-images-inner.sh
fi

# Build the deployable image from the output.
docker build --rm --force-rm -t oasislabs/testnet - <target/docker-deployment/context.tar.gz