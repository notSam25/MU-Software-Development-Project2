#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
REQUEST_PERMIT_ENDPOINT="${BASE_URL%/}/api/request-permit"

JWT_TOKEN="${JWT_TOKEN:-}"
REQUEST_NUMBER="${REQUEST_NUMBER:-REQ-$(date +%s)}"
ACTIVITY_DESCRIPTION="${ACTIVITY_DESCRIPTION:-Routine maintenance operation}"
ACTIVITY_START_DATE="${ACTIVITY_START_DATE:-2026-03-28T20:00:00Z}"
ACTIVITY_DURATION="${ACTIVITY_DURATION:-3600000000000}"
ENVIRONMENTAL_PERMIT_ID="${ENVIRONMENTAL_PERMIT_ID:-1}"

if [ -z "${JWT_TOKEN}" ]; then
  echo "Error: JWT_TOKEN environment variable not set"
  echo "Run login_regulated_entity.sh first and export the token:"
  echo "export JWT_TOKEN=\"<token>\""
  exit 1
fi

echo "POST ${REQUEST_PERMIT_ENDPOINT}"
echo "Authorization: Bearer ${JWT_TOKEN:0:20}..."

curl -sS -i -X POST "${REQUEST_PERMIT_ENDPOINT}" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${JWT_TOKEN}" \
  --data "$(cat <<JSON
{
  "request_number": "${REQUEST_NUMBER}",
  "activity_description": "${ACTIVITY_DESCRIPTION}",
  "activity_start_date": "${ACTIVITY_START_DATE}",
  "activity_duration": ${ACTIVITY_DURATION},
  "environmental_permit_id": ${ENVIRONMENTAL_PERMIT_ID}
}
JSON
)"
