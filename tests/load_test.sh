#!/bin/bash

HOST="${1:-http://localhost:8080}"
TOTAL=100

EVENT_NAMES=("page_view" "button_click" "form_submit" "search" "purchase" "add_to_cart" "logout" "login" "signup" "checkout")
ENTITY_TYPES=("page" "button" "form" "product" "user" "cart" "session")
PLATFORMS=("desktop" "mobile" "tablet")
SOURCES=("web" "ios" "android" "api")
DEVICE_TYPES=("desktop" "phone" "tablet")
USER_IDS=("user_001" "user_002" "user_003" "user_004" "user_005" "user_006" "user_007" "user_008" "user_009" "user_010")
SESSION_IDS=("sess_aaa" "sess_bbb" "sess_ccc" "sess_ddd" "sess_eee")

ok=0
fail=0

echo "Sending $TOTAL events to $HOST ..."
echo ""

for i in $(seq 1 $TOTAL); do
  EVENT_NAME="${EVENT_NAMES[$((RANDOM % ${#EVENT_NAMES[@]}))]}"
  ENTITY_TYPE="${ENTITY_TYPES[$((RANDOM % ${#ENTITY_TYPES[@]}))]}"
  PLATFORM="${PLATFORMS[$((RANDOM % ${#PLATFORMS[@]}))]}"
  SOURCE="${SOURCES[$((RANDOM % ${#SOURCES[@]}))]}"
  DEVICE_TYPE="${DEVICE_TYPES[$((RANDOM % ${#DEVICE_TYPES[@]}))]}"
  ACTOR_USER_ID="${USER_IDS[$((RANDOM % ${#USER_IDS[@]}))]}"
  SESSION_ID="${SESSION_IDS[$((RANDOM % ${#SESSION_IDS[@]}))]}"
  ENTITY_ID="entity_$((RANDOM % 50 + 1))"
  APP_VERSION="1.$((RANDOM % 5)).$((RANDOM % 10))"
  TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$HOST/events" \
    -H "Content-Type: application/json" \
    -d "{
      \"timestamp\": \"$TS\",
      \"event_name\": \"$EVENT_NAME\",
      \"entity_type\": \"$ENTITY_TYPE\",
      \"entity_id\": \"$ENTITY_ID\",
      \"actor_user_id\": \"$ACTOR_USER_ID\",
      \"session_id\": \"$SESSION_ID\",
      \"source\": \"$SOURCE\",
      \"platform\": \"$PLATFORM\",
      \"app_version\": \"$APP_VERSION\",
      \"device_type\": \"$DEVICE_TYPE\",
      \"properties\": {\"index\": $i, \"load_test\": true}
    }")

  if [ "$RESPONSE" = "202" ]; then
    ok=$((ok + 1))
    echo "[$i/$TOTAL] OK (202) — $EVENT_NAME / $ENTITY_TYPE / $PLATFORM"
  else
    fail=$((fail + 1))
    echo "[$i/$TOTAL] FAIL (HTTP $RESPONSE) — $EVENT_NAME"
  fi
done

echo ""
echo "Done. Success: $ok / $TOTAL — Failed: $fail / $TOTAL"