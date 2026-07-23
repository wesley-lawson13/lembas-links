#!/usr/bin/env bash
#
# End-to-end smoke test for the Lembas Links API.
#
# Runs curl against a live docker compose stack (assumes `make run` is already
# up, matching how the seed/migrate targets assume the stack is running) and
# asserts the expected status code at every step: health, anonymous session
# minting, link create/list/stats, cross-key ownership isolation, redirect,
# soft-delete, the session-mint rate limit, and CORS preflight.
#
# Usage: make e2e   (or: ./scripts/e2e_smoke_test.sh)
#   API_URL             base URL of the running API   (default http://localhost:8080)
#   ORIGIN              origin used for the CORS step (default http://localhost:5173)
#   SESSION_RATE_LIMIT  mints allowed per window      (default 5, must match the API's setting)

set -u

API_URL="${API_URL:-http://localhost:8080}"
ORIGIN="${ORIGIN:-http://localhost:5173}"
SESSION_RATE_LIMIT="${SESSION_RATE_LIMIT:-5}"

PASS_COUNT=0

# fail(step, detail): print which step failed and exit non-zero immediately.
# Called by every assertion below so the first unexpected status stops the run.
fail() {
    echo "FAIL: $1"
    echo "      $2"
    exit 1
}

# pass(step): log a successful step and bump the counter. Called after each assertion.
pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    echo "ok $PASS_COUNT: $1"
}

# json_field(json, field): extract a string field's value from a compact JSON
# response (gin emits no whitespace) without requiring jq. Used to capture
# api_key and slug values between steps.
json_field() {
    printf '%s' "$1" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p"
}

# reset_session_rate_counters: clear rate:session:* in Redis so session-mint
# steps are deterministic — the script itself mints keys before the rate-limit
# step, and reruns within the window would otherwise carry counters over.
# Called at startup and again right before the rate-limit step.
reset_session_rate_counters() {
    docker compose exec -T redis redis-cli EVAL \
        "for _, k in ipairs(redis.call('KEYS', 'rate:session:*')) do redis.call('DEL', k) end return 1" 0 \
        > /dev/null \
        || fail "reset session rate counters" "could not reach the redis container — is the stack up? (make run)"
}

reset_session_rate_counters

# --- 1. health check ---
status=$(curl -s -o /dev/null -w '%{http_code}' "$API_URL/health")
[ "$status" = "200" ] || fail "GET /health" "expected 200, got $status"
pass "GET /health -> 200"

# --- 2. mint session key A ---
resp=$(curl -s -w '\n%{http_code}' -X POST "$API_URL/session")
status=$(printf '%s' "$resp" | tail -n1)
body=$(printf '%s' "$resp" | sed '$d')
[ "$status" = "201" ] || fail "POST /session (key A)" "expected 201, got $status: $body"
KEY_A=$(json_field "$body" "api_key")
[ -n "$KEY_A" ] || fail "POST /session (key A)" "201 response had no api_key field: $body"
pass "POST /session -> 201, key A captured"

# --- 3. create a link with key A ---
resp=$(curl -s -w '\n%{http_code}' -X POST "$API_URL/links" \
    -H "Authorization: Bearer $KEY_A" \
    -H "Content-Type: application/json" \
    -d '{"url": "https://example.com/e2e-smoke"}')
status=$(printf '%s' "$resp" | tail -n1)
body=$(printf '%s' "$resp" | sed '$d')
[ "$status" = "201" ] || fail "POST /links" "expected 201, got $status: $body"
SLUG=$(json_field "$body" "slug")
[ -n "$SLUG" ] || fail "POST /links" "201 response had no slug field: $body"
pass "POST /links -> 201, slug '$SLUG' captured"

# --- 4. list key A's links, slug must be present ---
resp=$(curl -s -w '\n%{http_code}' "$API_URL/links" -H "Authorization: Bearer $KEY_A")
status=$(printf '%s' "$resp" | tail -n1)
body=$(printf '%s' "$resp" | sed '$d')
[ "$status" = "200" ] || fail "GET /links" "expected 200, got $status: $body"
printf '%s' "$body" | grep -q "\"slug\":\"$SLUG\"" \
    || fail "GET /links" "slug '$SLUG' not present in list: $body"
pass "GET /links -> 200, slug present in list"

# --- 5. stats with the owning key ---
resp=$(curl -s -w '\n%{http_code}' \
    "$API_URL/links/$SLUG/stats" -H "Authorization: Bearer $KEY_A")
status=$(printf '%s' "$resp" | tail -n1)
body=$(printf '%s' "$resp" | sed '$d')
[ "$status" = "200" ] || fail "GET /links/:slug/stats (owner)" "expected 200, got $status: $body"
printf '%s' "$body" | grep -q '"daily_clicks":\[' \
    || fail "GET /links/:slug/stats (owner)" "no daily_clicks array in response: $body"
printf '%s' "$body" | grep -q '"short_url":' \
    || fail "GET /links/:slug/stats (owner)" "no short_url field in response: $body"
pass "GET /links/$SLUG/stats (key A) -> 200 with daily_clicks and short_url"

# --- 6. stats with a different key must 404 (ownership isolation) ---
resp=$(curl -s -w '\n%{http_code}' -X POST "$API_URL/session")
status=$(printf '%s' "$resp" | tail -n1)
body=$(printf '%s' "$resp" | sed '$d')
[ "$status" = "201" ] || fail "POST /session (key B)" "expected 201, got $status: $body"
KEY_B=$(json_field "$body" "api_key")
[ -n "$KEY_B" ] || fail "POST /session (key B)" "201 response had no api_key field: $body"

status=$(curl -s -o /dev/null -w '%{http_code}' \
    "$API_URL/links/$SLUG/stats" -H "Authorization: Bearer $KEY_B")
[ "$status" = "404" ] || fail "GET /links/:slug/stats (non-owner)" "expected 404, got $status"
pass "GET /links/$SLUG/stats (key B) -> 404, ownership enforced"

# --- 7. public redirect ---
resp=$(curl -s -o /dev/null -w '%{http_code} %{redirect_url}' "$API_URL/$SLUG")
status=$(printf '%s' "$resp" | cut -d' ' -f1)
location=$(printf '%s' "$resp" | cut -d' ' -f2-)
[ "$status" = "302" ] || fail "GET /:slug" "expected 302, got $status"
[ -n "$location" ] || fail "GET /:slug" "302 response had no Location header"
pass "GET /$SLUG -> 302 to $location"

# --- 8. delete the link ---
status=$(curl -s -o /dev/null -w '%{http_code}' \
    -X DELETE "$API_URL/links/$SLUG" -H "Authorization: Bearer $KEY_A")
[ "$status" = "204" ] || fail "DELETE /links/:slug" "expected 204, got $status"
pass "DELETE /links/$SLUG -> 204"

# --- 9. redirect after soft-delete must 404 ---
status=$(curl -s -o /dev/null -w '%{http_code}' "$API_URL/$SLUG")
[ "$status" = "404" ] || fail "GET /:slug (deleted)" "expected 404, got $status"
pass "GET /$SLUG after delete -> 404"

# --- 10. session-mint rate limit ---
# From a clean counter, exactly SESSION_RATE_LIMIT mints succeed and the next
# one is rejected — the mint cap is the real abuse backstop now that every
# fresh key resets the per-key rate budget.
reset_session_rate_counters
i=1
while [ "$i" -le "$SESSION_RATE_LIMIT" ]; do
    status=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API_URL/session")
    [ "$status" = "201" ] || fail "POST /session (rate limit, mint $i)" \
        "expected 201 within the limit of $SESSION_RATE_LIMIT, got $status"
    i=$((i + 1))
done
status=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API_URL/session")
[ "$status" = "429" ] || fail "POST /session (rate limit, over)" \
    "expected 429 on mint $((SESSION_RATE_LIMIT + 1)), got $status"
pass "POST /session rate limit -> 429 after $SESSION_RATE_LIMIT mints"

# --- 11. CORS preflight ---
resp=$(curl -s -D - -o /dev/null -w '%{http_code}' -X OPTIONS "$API_URL/links" \
    -H "Origin: $ORIGIN" \
    -H "Access-Control-Request-Method: POST" \
    -H "Access-Control-Request-Headers: Authorization,Content-Type")
status=$(printf '%s' "$resp" | tail -n1)
[ "$status" = "204" ] || fail "OPTIONS /links (CORS preflight)" \
    "expected 204, got $status — is CORS_ALLOWED_ORIGINS=$ORIGIN set in the API's env?"
printf '%s' "$resp" | grep -qi "^access-control-allow-origin: $ORIGIN" \
    || fail "OPTIONS /links (CORS preflight)" \
        "204 but no 'Access-Control-Allow-Origin: $ORIGIN' header echoed back"
pass "OPTIONS /links preflight -> 204 with origin echoed"

echo ""
echo "All $PASS_COUNT smoke test steps passed."
