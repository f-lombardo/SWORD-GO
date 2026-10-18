#!/bin/bash

########################################################################################################################
# This script needs ngrok, that is used to expose local port to the internet,
# so that you can expose the URL for downloading the provisioning script
# If you defined a ngrok domain, you can define the NGROK_URL in .env
########################################################################################################################

DIR="$(dirname "${BASH_SOURCE[0]}")"
DIR="$(readlink -f "${DIR}")"

source "${DIR}"/../.env

if [ -z "$NGROK_URL" ]; then
  URL=
else
  URL="--url $NGROK_URL"
fi

echo "TOKEN: $NGROK_AUTHTOKEN"

NGROK_AUTHTOKEN=$NGROK_AUTHTOKEN ngrok http $SWORD_GO_HTTP_PORT $URL