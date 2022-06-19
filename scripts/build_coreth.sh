#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Directory above this script
AVALANCHE_PATH=$( cd "$( dirname "${BASH_SOURCE[0]}" )"; cd .. && pwd )

# Load the versions
source "$AVALANCHE_PATH"/scripts/versions.sh

# Load the constants
source "$AVALANCHE_PATH"/scripts/constants.sh

# check if there's args defining different dijethh source and build paths
if [[ $# -eq 2 ]]; then
    dijethh_path=$1
    evm_path=$2
elif [[ $# -eq 0 ]]; then
    if [[ ! -d "$dijethh_path" ]]; then
        go get "github.com/lasthyphen/dijethh@$dijethh_version"
    fi
else
    echo "Invalid arguments to build dijethh. Requires either no arguments (default) or two arguments to specify dijethh directory and location to add binary."
    exit 1
fi

# Build Dijethh
echo "Building Dijethh @ ${dijethh_version} ..."
cd "$dijethh_path"
go build -ldflags "-X github.com/lasthyphen/dijethh/plugin/evm.Version=$dijethh_version $static_ld_flags" -o "$evm_path" "plugin/"*.go
cd "$AVALANCHE_PATH"

# Building dijethh + using go get can mess with the go.mod file.
go mod tidy
