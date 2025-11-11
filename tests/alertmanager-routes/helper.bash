#!/usr/bin/env bash

# Define amtool arguments
AMTOOL_ARGS=""
if [[ -n ${VERBOSE+x} ]]; then
  AMTOOL_ARGS+=--tree
fi

# test alertmanager routes using amtool
# ALERTMANAGER_CONFIG_FILE must be set to the alertmanager config file path
# AMTOOL_BIN must be set to the amtool binary path
amtool() {
  "$AMTOOL_BIN" config routes test --config.file "$ALERTMANAGER_CONFIG_FILE" $AMTOOL_ARGS "$@"
}

# setup performed prior to running each test
setup() {
    # Load Bats helpers
    load '../../bats/support/load'
    load '../../bats/assert/load'
}
