#!/bin/sh
set -e

# First-run: neither config nor workspace exists.
# If config.json is already mounted but workspace is missing we skip onboard to
# avoid the interactive "Overwrite? (y/n)" prompt hanging in a non-TTY container.
if [ ! -d "${HOME}/.octa/workspace" ] && [ ! -f "${HOME}/.octa/config.json" ]; then
    octa onboard
    echo ""
    echo "First-run setup complete."
    echo "Edit ${HOME}/.octa/config.json (add your API key, bot tokens, etc.) then restart the container."
    exit 0
fi

exec octa gateway "$@"
