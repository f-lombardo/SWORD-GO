#!/bin/bash

DIR="$(dirname "${BASH_SOURCE[0]}")"
DIR="$(readlink -f "${DIR}")"

# You should have created the /run/secrets folder with
# sudo mkdir -p /run/secrets
sudo mount --bind "$DIR"/../secrets /run/secrets