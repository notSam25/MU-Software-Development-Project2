#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
LOGIN_ENDPOINT="${BASE_URL%/}/api/login"

EMAIL="${EMAIL:-jane.doe@example.com}"
PASSWORD="${PASSWORD:-example-password-123}"
ACCOUNT_TYPE="${ACCOUNT_TYPE:-regulated_entity}"

echo "POST ${LOGIN_ENDPOINT}"

RESPONSE=$(curl -sS -X POST "${LOGIN_ENDPOINT}" \
  -H "Content-Type: application/json" \
  --data "$(cat <<JSON
{
  "account_type": "${ACCOUNT_TYPE}",
  "email": "${EMAIL}",
  "password": "${PASSWORD}"
}
JSON
)")

echo "${RESPONSE}" | jq .

TOKEN=$(echo "${RESPONSE}" | jq -r '.token // empty')

if [ -z "${TOKEN}" ]; then
  echo "Error: Failed to extract token from login response"
  exit 1
fi

echo ""
echo "Login successful!"
echo "Token: ${TOKEN}"
echo ""
echo "Export token for use in subsequent requests:"
echo "export JWT_TOKEN=\"${TOKEN}\""
