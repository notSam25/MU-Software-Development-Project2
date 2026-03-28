#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
ENDPOINT="${BASE_URL%/}/api/register"

CONTACT_PERSON_NAME="${CONTACT_PERSON_NAME:-Jane Doe}"
PASSWORD="${PASSWORD:-example-password-123}"
EMAIL="${EMAIL:-jane.doe@example.com}"
ORGANIZATION_NAME="${ORGANIZATION_NAME:-Example Org LLC}"
ORGANIZATION_ADDRESS="${ORGANIZATION_ADDRESS:-123 Main St, Columbia, MO}"

echo "POST ${ENDPOINT}"

curl -sS -i -X POST "${ENDPOINT}" \
  -H "Content-Type: application/json" \
  --data "$(cat <<JSON
{
  "contact_person_name": "${CONTACT_PERSON_NAME}",
  "password": "${PASSWORD}",
  "email": "${EMAIL}",
  "organization_name": "${ORGANIZATION_NAME}",
  "organization_address": "${ORGANIZATION_ADDRESS}"
}
JSON
)"
