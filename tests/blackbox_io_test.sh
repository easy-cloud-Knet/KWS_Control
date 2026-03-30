#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# KWS_Control Black-Box Integration Test
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.test.yml"
PROJECT_NAME="kws-test"
BASE_URL="http://localhost:18081"
REDIS_CONTAINER="kws-test-redis"
MAX_WAIT=90

# --- counters ---
PASS=0
FAIL=0
TOTAL=0

# --- colors ---
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ============================================================
# Helper functions
# ============================================================

pass() {
    ((PASS++)) || true
    ((TOTAL++)) || true
    echo -e "  ${GREEN}PASS${NC} $1"
}

fail() {
    ((FAIL++)) || true
    ((TOTAL++)) || true
    echo -e "  ${RED}FAIL${NC} $1  (expected=$2, got=$3)"
}

section() {
    echo -e "\n${CYAN}${BOLD}=== $1 ===${NC}"
}

# assert_status <description> <expected_http_code> <actual_http_code>
assert_status() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        pass "$desc"
    else
        fail "$desc" "$expected" "$actual"
    fi
}

# assert_body_contains <description> <expected_substring> <body>
assert_body_contains() {
    local desc="$1" substr="$2" body="$3"
    if echo "$body" | grep -q "$substr"; then
        pass "$desc"
    else
        fail "$desc" "body contains '$substr'" "body='$body'"
    fi
}

# http <method> <path> [body] -> sets HTTP_CODE and HTTP_BODY
http() {
    local method="$1" path="$2" body="${3:-}"
    local tmp
    tmp=$(mktemp)

    if [ -n "$body" ]; then
        HTTP_CODE=$(curl -s -o "$tmp" -w "%{http_code}" \
            -X "$method" \
            -H "Content-Type: application/json" \
            -d "$body" \
            "${BASE_URL}${path}" 2>/dev/null) || HTTP_CODE="000"
    else
        HTTP_CODE=$(curl -s -o "$tmp" -w "%{http_code}" \
            -X "$method" \
            "${BASE_URL}${path}" 2>/dev/null) || HTTP_CODE="000"
    fi

    HTTP_BODY=$(cat "$tmp" 2>/dev/null || echo "")
    rm -f "$tmp"
}

# redis_exec <args...> -> runs redis-cli inside the redis container
redis_exec() {
    docker exec "$REDIS_CONTAINER" redis-cli "$@" 2>/dev/null
}

# ============================================================
# Lifecycle
# ============================================================

cleanup() {
    echo -e "\n${YELLOW}Cleaning up test environment...${NC}"
    docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

echo -e "${BOLD}Starting test environment...${NC}"
docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" up -d --build

echo -n "Waiting for KWS_Control to be ready"
for i in $(seq 1 $MAX_WAIT); do
    if curl -sf -o /dev/null -X GET -H "Content-Type: application/json" -d '{"uuid":"healthcheck"}' "${BASE_URL}/vm/info" 2>/dev/null; then
        echo -e " ${GREEN}ready${NC} (${i}s)"
        break
    fi
    # 404 also means the service is up (vm not found in redis)
    code=$(curl -s -o /dev/null -w "%{http_code}" -X GET -H "Content-Type: application/json" -d '{"uuid":"healthcheck"}' "${BASE_URL}/vm/info" 2>/dev/null) || code="000"
    if [ "$code" = "404" ]; then
        echo -e " ${GREEN}ready${NC} (${i}s)"
        break
    fi
    if [ "$i" -eq "$MAX_WAIT" ]; then
        echo -e " ${RED}TIMEOUT${NC}"
        echo "Container logs:"
        docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" logs control-test 2>/dev/null | tail -50
        exit 1
    fi
    echo -n "."
    sleep 1
done

# ============================================================
# Section 1: HTTP Method Routing
# ============================================================
section "Section 1: HTTP Method Routing"

http GET "/vm"
assert_status "GET /vm should be 405" "405" "$HTTP_CODE"

http PUT "/vm" '{"test":true}'
assert_status "PUT /vm should be 405" "405" "$HTTP_CODE"

http GET "/vm/shutdown"
assert_status "GET /vm/shutdown should be 405" "405" "$HTTP_CODE"

http DELETE "/vm/redis"
assert_status "DELETE /vm/redis should be 405" "405" "$HTTP_CODE"

http POST "/vm/status" '{"uuid":"test","type":"cpu"}'
assert_status "POST /vm/status should be 405" "405" "$HTTP_CODE"

http POST "/vm/connect"
assert_status "POST /vm/connect should be 405" "405" "$HTTP_CODE"

http POST "/vm/info" '{"uuid":"test"}'
assert_status "POST /vm/info should be 405" "405" "$HTTP_CODE"

http DELETE "/vm/start"
assert_status "DELETE /vm/start should be 405" "405" "$HTTP_CODE"

# ============================================================
# Section 2: Invalid JSON Body
# ============================================================
section "Section 2: Invalid JSON Body"

http POST "/vm" '{invalid-json}'
assert_status "POST /vm with invalid JSON should be 405" "405" "$HTTP_CODE"

http DELETE "/vm" 'not-json'
assert_status "DELETE /vm with invalid JSON should be 400" "400" "$HTTP_CODE"

http POST "/vm/shutdown" '{bad'
assert_status "POST /vm/shutdown with invalid JSON should be 400" "400" "$HTTP_CODE"

http POST "/vm/start" '!!!'
assert_status "POST /vm/start with invalid JSON should be 400" "400" "$HTTP_CODE"

http GET "/vm/status" 'null'
assert_status "GET /vm/status with invalid JSON should be 400" "400" "$HTTP_CODE"

http POST "/vm/redis" '{nope'
assert_status "POST /vm/redis with invalid JSON should be 400" "400" "$HTTP_CODE"

http GET "/vm/info" 'xyz'
assert_status "GET /vm/info with invalid JSON should be 400" "400" "$HTTP_CODE"

# ============================================================
# Section 3: Field Validation
# ============================================================
section "Section 3: Field Validation"

http GET "/vm/status" '{"uuid":"test-uuid"}'
assert_status "GET /vm/status without type field should be 400" "400" "$HTTP_CODE"

http GET "/vm/status" '{"uuid":"test-uuid","type":"network"}'
assert_status "GET /vm/status with invalid type should be 400" "400" "$HTTP_CODE"

# vmConnect uses query parameter
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X GET "${BASE_URL}/vm/connect" 2>/dev/null) || HTTP_CODE="000"
assert_status "GET /vm/connect without uuid param should be 400" "400" "$HTTP_CODE"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X GET "${BASE_URL}/vm/connect?uuid=" 2>/dev/null) || HTTP_CODE="000"
assert_status "GET /vm/connect with empty uuid should be 400" "400" "$HTTP_CODE"

# ============================================================
# Section 4: Non-existent UUID (Core-dependent endpoints)
# ============================================================
section "Section 4: Non-existent UUID"

http DELETE "/vm" '{"uuid":"nonexistent-uuid-0000"}'
assert_status "DELETE /vm with unknown UUID should be 500" "500" "$HTTP_CODE"

http POST "/vm/shutdown" '{"uuid":"nonexistent-uuid-0000"}'
assert_status "POST /vm/shutdown with unknown UUID should be 500" "500" "$HTTP_CODE"

http POST "/vm/start" '{"uuid":"nonexistent-uuid-0000"}'
assert_status "POST /vm/start with unknown UUID should be 500" "500" "$HTTP_CODE"

http GET "/vm/status" '{"uuid":"nonexistent-uuid-0000","type":"cpu"}'
assert_status "GET /vm/status with unknown UUID should be 500" "500" "$HTTP_CODE"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X GET "${BASE_URL}/vm/connect?uuid=nonexistent-uuid-0000" 2>/dev/null) || HTTP_CODE="000"
assert_status "GET /vm/connect with unknown UUID should be 500" "500" "$HTTP_CODE"

# ============================================================
# Section 5: Happy Path - VM Lifecycle
# ============================================================
section "Section 5: Happy Path - VM Lifecycle"

HAPPY_UUID="happy-test-$(date +%s)"

# 5-1. Create VM
echo -e "  ${CYAN}-- Create VM --${NC}"
http POST "/vm" "{
  \"domType\": \"kvm\",
  \"domName\": \"test-vm-happy\",
  \"uuid\": \"${HAPPY_UUID}\",
  \"os\": \"ubuntu\",
  \"HWInfo\": {\"cpu\": 2, \"memory\": 2048, \"disk\": 20},
  \"network\": {\"ips\": [], \"NetType\": 0},
  \"users\": [{\"name\": \"ubuntu\", \"groups\": \"sudo\", \"passWord\": \"testpass\", \"ssh\": []}],
  \"Subnettype\": \"\"
}"
assert_status "POST /vm create VM should be 201" "201" "$HTTP_CODE"

# 5-2. Verify VM info in Redis after creation
http GET "/vm/info" "{\"uuid\":\"${HAPPY_UUID}\"}"
assert_status "GET /vm/info after create should be 200" "200" "$HTTP_CODE"
assert_body_contains "VM info has correct uuid" "$HAPPY_UUID" "$HTTP_BODY"
assert_body_contains "VM info has correct cpu" '"cpu":2' "$HTTP_BODY"
assert_body_contains "VM info has correct memory" '"memory":2048' "$HTTP_BODY"
assert_body_contains "VM info has correct disk" '"disk":20' "$HTTP_BODY"

# 5-3. Start VM
echo -e "\n  ${CYAN}-- Start VM --${NC}"
http POST "/vm/start" "{\"uuid\":\"${HAPPY_UUID}\"}"
assert_status "POST /vm/start should be 200" "200" "$HTTP_CODE"

# 5-4. Get VM status (cpu)
echo -e "\n  ${CYAN}-- VM Status --${NC}"
http GET "/vm/status" "{\"uuid\":\"${HAPPY_UUID}\",\"type\":\"cpu\"}"
assert_status "GET /vm/status cpu should be 200" "200" "$HTTP_CODE"

http GET "/vm/status" "{\"uuid\":\"${HAPPY_UUID}\",\"type\":\"memory\"}"
assert_status "GET /vm/status memory should be 200" "200" "$HTTP_CODE"

http GET "/vm/status" "{\"uuid\":\"${HAPPY_UUID}\",\"type\":\"disk\"}"
assert_status "GET /vm/status disk should be 200" "200" "$HTTP_CODE"

# 5-5. Shutdown VM
echo -e "\n  ${CYAN}-- Shutdown VM --${NC}"
http POST "/vm/shutdown" "{\"uuid\":\"${HAPPY_UUID}\"}"
assert_status "POST /vm/shutdown should be 200" "200" "$HTTP_CODE"

# Verify Redis status updated to "stopped end" after shutdown
redis_val=$(redis_exec GET "$HAPPY_UUID")
if echo "$redis_val" | grep -q '"status":"stopped end"'; then
    pass "Redis status is 'stopped end' after shutdown"
else
    fail "Redis status should be 'stopped end' after shutdown" '"status":"stopped end"' "$redis_val"
fi

# 5-6. Delete VM
echo -e "\n  ${CYAN}-- Delete VM --${NC}"
http DELETE "/vm" "{\"uuid\":\"${HAPPY_UUID}\"}"
assert_status "DELETE /vm should be 200" "200" "$HTTP_CODE"

# Verify VM removed from Redis after deletion
http GET "/vm/info" "{\"uuid\":\"${HAPPY_UUID}\"}"
assert_status "GET /vm/info after delete should be 404" "404" "$HTTP_CODE"

# ============================================================
# Section 6: Redis Endpoints (E2E)
# ============================================================
section "Section 6: Redis Endpoints"

# 6a. Seed Redis with test VM data
echo -e "  ${YELLOW}Seeding Redis with test data...${NC}"

redis_exec SET "test-uuid-001" \
    '{"uuid":"test-uuid-001","cpu":4,"memory":8192,"disk":40960,"ip":"10.0.0.50","status":"unknown","time":1700000000}' \
    > /dev/null

redis_exec SET "test-uuid-002" \
    '{"uuid":"test-uuid-002","cpu":2,"memory":4096,"disk":20480,"ip":"10.0.0.99","status":"prepare begin","time":1700000000}' \
    > /dev/null

# 6b. POST /vm/redis -- Update VM status
echo -e "\n  ${CYAN}-- POST /vm/redis tests --${NC}"

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"started begin"}'
assert_status "Update status to 'started begin' should be 200" "200" "$HTTP_CODE"
assert_body_contains "Response body contains 'VM status updated'" "VM status updated" "$HTTP_BODY"

# Verify in Redis
redis_val=$(redis_exec GET "test-uuid-001")
if echo "$redis_val" | grep -q '"status":"started begin"'; then
    pass "Redis value has status 'started begin'"
else
    fail "Redis value should have status 'started begin'" '"status":"started begin"' "$redis_val"
fi

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"stopped end"}'
assert_status "Update status to 'stopped end' should be 200" "200" "$HTTP_CODE"

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"prepare begin"}'
assert_status "Update status to 'prepare begin' should be 200" "200" "$HTTP_CODE"

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"start begin"}'
assert_status "Update status to 'start begin' should be 200" "200" "$HTTP_CODE"

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"release end"}'
assert_status "Update status to 'release end' should be 200" "200" "$HTTP_CODE"

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"migrate begin"}'
assert_status "Update status to 'migrate begin' should be 200" "200" "$HTTP_CODE"

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"restort begin"}'
assert_status "Update status to 'restort begin' should be 200" "200" "$HTTP_CODE"

http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"unknown"}'
assert_status "Update status to 'unknown' should be 200" "200" "$HTTP_CODE"

# Invalid status -> normalized to "unknown"
http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"bogus-status"}'
assert_status "Update with invalid status should be 200 (normalized)" "200" "$HTTP_CODE"

redis_val=$(redis_exec GET "test-uuid-001")
if echo "$redis_val" | grep -q '"status":"unknown"'; then
    pass "Invalid status normalized to 'unknown' in Redis"
else
    fail "Invalid status should normalize to 'unknown'" '"status":"unknown"' "$redis_val"
fi

# Empty status -> normalized to "unknown"
http POST "/vm/redis" '{"UUID":"test-uuid-001","status":""}'
assert_status "Update with empty status should be 200 (normalized)" "200" "$HTTP_CODE"

# "null" string -> normalized to "unknown"
http POST "/vm/redis" '{"UUID":"test-uuid-001","status":"null"}'
assert_status "Update with 'null' string status should be 200 (normalized)" "200" "$HTTP_CODE"

# Non-existent UUID -> 500 (UpdateVMStatusInRedis requires existing data)
http POST "/vm/redis" '{"UUID":"no-such-uuid-999","status":"started begin"}'
assert_status "Update status for non-existent UUID should be 500" "500" "$HTTP_CODE"

# 6c. GET /vm/info tests
echo -e "\n  ${CYAN}-- GET /vm/info tests --${NC}"

http GET "/vm/info" '{"uuid":"test-uuid-001"}'
assert_status "Get info for existing UUID should be 200" "200" "$HTTP_CODE"
assert_body_contains "Response contains uuid field" "test-uuid-001" "$HTTP_BODY"
assert_body_contains "Response contains cpu field" '"cpu":4' "$HTTP_BODY"
assert_body_contains "Response contains memory field" '"memory":8192' "$HTTP_BODY"
assert_body_contains "Response contains disk field" '"disk":40960' "$HTTP_BODY"
assert_body_contains "Response contains ip field" '"ip":"10.0.0.50"' "$HTTP_BODY"

http GET "/vm/info" '{"uuid":"does-not-exist"}'
assert_status "Get info for non-existent UUID should be 404" "404" "$HTTP_CODE"

http GET "/vm/info" '{}'
assert_status "Get info with empty body should be 404" "404" "$HTTP_CODE"

# 6d. Write-then-read flow
echo -e "\n  ${CYAN}-- Write-Read Flow --${NC}"

http POST "/vm/redis" '{"UUID":"test-uuid-002","status":"started begin"}'
assert_status "Flow: update test-uuid-002 status should be 200" "200" "$HTTP_CODE"

http GET "/vm/info" '{"uuid":"test-uuid-002"}'
assert_status "Flow: read test-uuid-002 info should be 200" "200" "$HTTP_CODE"
assert_body_contains "Flow: response has correct cpu" '"cpu":2' "$HTTP_BODY"
assert_body_contains "Flow: response has correct ip" '"ip":"10.0.0.99"' "$HTTP_BODY"

redis_val=$(redis_exec GET "test-uuid-002")
if echo "$redis_val" | grep -q '"status":"started begin"'; then
    pass "Flow: Redis confirms status updated to 'started begin'"
else
    fail "Flow: Redis should have status 'started begin'" '"status":"started begin"' "$redis_val"
fi

# ============================================================
# Results
# ============================================================
section "Test Results"

echo -e "  Total:   ${BOLD}${TOTAL}${NC}"
echo -e "  Passed:  ${GREEN}${PASS}${NC}"
echo -e "  Failed:  ${RED}${FAIL}${NC}"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo -e "${RED}${BOLD}SOME TESTS FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}${BOLD}ALL TESTS PASSED${NC}"
    exit 0
fi
