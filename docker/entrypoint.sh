#!/bin/sh
set -e

export HOME=/root
OCTA_DIR="${HOME}/.octa"

# Ensure directories exist
mkdir -p "${OCTA_DIR}/tokens" "${OCTA_DIR}/data" "${OCTA_DIR}/workspace"

# First-run: config doesn't exist yet
if [ ! -f "${OCTA_DIR}/config.json" ]; then
    echo "No config found. Creating minimal config..."
    cat > "${OCTA_DIR}/config.json" << 'EOF'
{
  "agents": {
    "defaults": {
      "model": "gemini-2.0-flash",
      "max_tokens": 8192,
      "max_tool_iterations": 20
    }
  },
  "model_list": [
    {
      "model_name": "gemini-2.0-flash",
      "api_base": "https://generativelanguage.googleapis.com/v1beta/openai/",
      "api_key": "YOUR_GEMINI_API_KEY",
      "model": "gemini-2.0-flash"
    }
  ]
}
EOF
    echo ""
    echo "Config created at: ${OCTA_DIR}/config.json"
    echo "Edit it to add your API key, then restart the container."
    echo ""
    echo "Example:"
    echo "  docker compose --profile gateway up -d"
    exit 0
fi

exec octa gateway "$@"
