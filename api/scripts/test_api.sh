#!/bin/bash

BASE_URL="http://localhost:8080"
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}✓ PASS${NC}: $1"; }
fail() { echo -e "${RED}✗ FAIL${NC}: $1"; }

echo "================================"
echo "  Lembas Links API Test Suite"
echo "================================"
echo ""

# Test 1 — Health Check
echo "Test 1: Health Check"
response=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/health)
if [ "$response" = "200" ]; then
    pass "GET /health returned 200"
else
    fail "GET /health returned $response"
fi
echo ""

# Test 2 — Create a Link
echo "Test 2: Create a Link"
create_response=$(curl -s -X POST $BASE_URL/links \
    -H "Content-Type: application/json" \
    -d '{"url": "https://google.com", "api_key": "test-key"}')
echo "  Response: $create_response"

slug=$(echo $create_response | grep -o '"slug":"[^"]*"' | cut -d'"' -f4)
if [ -n "$slug" ]; then
    pass "POST /links returned slug: $slug"
else
    fail "POST /links did not return a slug"
fi
echo ""

# Test 3 — Create a Link with no URL
echo "Test 3: Create Link with Missing URL"
response=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE_URL/links \
    -H "Content-Type: application/json" \
    -d '{"api_key": "test-key"}')
if [ "$response" = "400" ]; then
    pass "POST /links with no URL returned 400"
else
    fail "POST /links with no URL returned $response (expected 400)"
fi
echo ""

# Test 4 — Redirect
echo "Test 4: Redirect"
response=$(curl -s -o /dev/null -w "%{http_code}" -L $BASE_URL/$slug)
if [ "$response" = "200" ]; then
    pass "GET /$slug redirected successfully"
else
    fail "GET /$slug returned $response (expected 200 after redirect)"
fi
echo ""

# Test 5 — Redirect from cache (second hit)
echo "Test 5: Redirect from Redis Cache"
response=$(curl -s -o /dev/null -w "%{http_code}" -L $BASE_URL/$slug)
if [ "$response" = "200" ]; then
    pass "GET /$slug served from cache successfully"
else
    fail "GET /$slug from cache returned $response"
fi
echo ""

# Test 6 — Get Stats
echo "Test 6: Get Stats"
stats_response=$(curl -s $BASE_URL/links/$slug/stats)
echo "  Response: $stats_response"
click_count=$(echo $stats_response | grep -o '"click_count":[0-9]*' | cut -d':' -f2)
if [ -n "$click_count" ]; then
    pass "GET /links/$slug/stats returned click_count: $click_count"
else
    fail "GET /links/$slug/stats did not return click_count"
fi
echo ""

# Test 7 — Redirect nonexistent slug
echo "Test 7: Redirect Nonexistent Slug"
response=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/this-slug-does-not-exist)
if [ "$response" = "404" ]; then
    pass "GET /nonexistent returned 404"
else
    fail "GET /nonexistent returned $response (expected 404)"
fi
echo ""

# Test 8 — Delete Link
echo "Test 8: Delete Link"
response=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE_URL/links/$slug)
if [ "$response" = "204" ]; then
    pass "DELETE /links/$slug returned 204"
else
    fail "DELETE /links/$slug returned $response (expected 204)"
fi
echo ""

# Test 9 — Redirect deleted link
echo "Test 9: Redirect Deleted Link"
response=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/$slug)
if [ "$response" = "404" ]; then
    pass "GET /$slug after delete returned 404"
else
    fail "GET /$slug after delete returned $response (expected 404)"
fi
echo ""

# Test 10 — Stats for deleted link
echo "Test 10: Stats for Deleted Link"
response=$(curl -s -o /dev/null -w "%{http_code}" $BASE_URL/links/$slug/stats)
if [ "$response" = "404" ]; then
    pass "GET /links/$slug/stats after delete returned 404"
else
    fail "GET /links/$slug/stats after delete returned $response (expected 404)"
fi
echo ""

echo "================================"
echo "  Tests Complete"
echo "================================"
