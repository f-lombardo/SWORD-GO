#!/bin/bash

DIR="$(dirname "${BASH_SOURCE[0]}")"
DIR="$(readlink -f "${DIR}")"

sudo mkdir -p /run/secrets

sudo mount --bind "$DIR"/../secrets /run/secrets