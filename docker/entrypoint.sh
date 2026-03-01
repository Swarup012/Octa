#!/bin/sh
set -e

export HOME=/root
OCTA_DIR="${HOME}/.octa"

# First-run: config doesn't exist yet
if [ ! -f "${OCTA_DIR}/config.json" ]; then
    octa onboard
    echo ""
    echo "First-run setup complete."
    echo "Edit docker/data/config.json (add your API key, bot tokens, etc.) then restart the container."
    exit 0
fi

exec octa gateway "$@"
