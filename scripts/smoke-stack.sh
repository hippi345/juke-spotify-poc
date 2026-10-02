#!/usr/bin/env bash
# End-to-end smoke test for the production-shaped Compose stack.
# Run from repo root: ./scripts/smoke-stack.sh

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

COMPOSE="docker compose"
if ! docker compose ps >/dev/null 2>&1; then
  COMPOSE="sudo docker compose"
fi

fail() {
  local component="${1:?component name required}"
  shift
  echo -e "${RED}FAIL (${component}):${NC} $*"
  exit 1
}
ok() {
  echo -e "${GREEN}OK (${1}):${NC} ${2}"
}

echo "=== Smoke test: Juke Spotify Compose stack ==="

# --- mysql ---
mysql_ping=$($COMPOSE exec -T mysql mysqladmin ping -h localhost -u root -pjukespotify 2>/dev/null || true)
echo "$mysql_ping" | grep -q alive || fail "mysql" "mysqladmin ping failed: $mysql_ping"
ok "mysql" "mysqladmin ping alive"

# --- api-1 ---
i1=$(curl -sf --max-time 10 http://localhost:18081/health | jq -r '.instance_id // empty')
[[ "$i1" == "api-1" ]] || fail "api-1" "direct :18081 health expected instance_id api-1, got '$i1'"
ok "api-1" "GET :18081/health instance_id=api-1"

# --- api-2 ---
i2=$(curl -sf --max-time 10 http://localhost:18082/health | jq -r '.instance_id // empty')
[[ "$i2" == "api-2" ]] || fail "api-2" "direct :18082 health expected instance_id api-2, got '$i2'"
ok "api-2" "GET :18082/health instance_id=api-2"

# --- nginx ---
lb_json=$(curl -sf --max-time 10 http://localhost:8081/health)
lb=$(echo "$lb_json" | jq -r '.instance_id // empty')
lb_status=$(echo "$lb_json" | jq -r '.status // empty')
[[ "$lb_status" == "ok" && -n "$lb" ]] || fail "nginx" "load balancer /health on :8081 failed: $lb_json"
ok "nginx" "GET :8081/health via LB (ip_hash served $lb)"

# --- web ---
web_body=$(curl -sf --max-time 15 http://localhost:5173/)
echo "$web_body" | grep -qi '<div id="root"' || fail "web" "GET :5173/ missing Vite root mount"
ok "web" "GET :5173/ serves client shell"

# --- redis (nearby venues cache miss then hit) ---
staff_json=$(curl -s --max-time 10 -X POST http://localhost:8081/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"smoke-staff@example.com","password":"smokepass123","role":"staff"}' || true)
staff_token=$(echo "$staff_json" | jq -r '.token // empty' | tr -d '\r')
if [ -z "$staff_token" ]; then
  staff_json=$(curl -sf --max-time 10 -X POST http://localhost:8081/api/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"email":"smoke-staff@example.com","password":"smokepass123"}')
  staff_token=$(echo "$staff_json" | jq -r '.token // empty' | tr -d '\r')
fi
if [ -z "$staff_token" ]; then
  fail "redis" "auth prerequisite: could not obtain staff token"
fi

venue_name="SmokeVenue$(date +%s)"
venue_resp=$(curl -sf --max-time 10 -X POST http://localhost:8081/api/venues \
  -H "Authorization: Bearer $staff_token" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"$venue_name\",\"latitude\":37.77,\"longitude\":-122.42}")
venue_id=$(echo "$venue_resp" | jq -r '.id')
[[ -n "$venue_id" && "$venue_id" != "null" ]] || fail "mysql" "venue create via API failed: $venue_resp"

venue_rows=$($COMPOSE exec -T mysql mysql -uroot -pjukespotify -N -e \
  "SELECT COUNT(*) FROM venues WHERE id=${venue_id} AND name='${venue_name}'" jukespotify 2>/dev/null | tr -d '\r')
[[ "${venue_rows:-0}" == "1" ]] || fail "mysql" "venue id $venue_id not found in MySQL"
ok "mysql" "venue $venue_id persisted in venues table"

near_lat="37.$(date +%s | tail -c 5)"
near_lng="-122.42"
$COMPOSE exec -T redis redis-cli DEL "jukespotify:nearby:${near_lat}:${near_lng}:2000" >/dev/null 2>&1 || true
h1=$(curl -sf -D - -o /dev/null --max-time 10 "http://localhost:8081/api/venues/nearby?lat=${near_lat}&lng=${near_lng}" | awk -F': ' 'tolower($1)=="x-cache"{print $2}' | tr -d '\r')
h2=$(curl -sf -D - -o /dev/null --max-time 10 "http://localhost:8081/api/venues/nearby?lat=${near_lat}&lng=${near_lng}" | awk -F': ' 'tolower($1)=="x-cache"{print $2}' | tr -d '\r')
[[ "$h1" == "MISS" ]] || fail "redis" "first nearby expected X-Cache MISS, got $h1"
[[ "$h2" == "HIT" ]] || fail "redis" "second nearby expected X-Cache HIT, got $h2"
$COMPOSE exec -T redis redis-cli KEYS 'jukespotify:nearby:*' | grep -q nearby || fail "redis" "no nearby cache keys in Redis"
ok "redis" "nearby X-Cache MISS then HIT"

# --- opensearch ---
sleep 2
search_resp=$(curl -sf --max-time 15 "http://localhost:8081/api/venues/search?q=${venue_name}")
search_src=$(echo "$search_resp" | jq -r '.source')
search_hit=$(echo "$search_resp" | jq -r --arg n "$venue_name" '.venues[]? | select(.name==$n) | .id' | head -1)
if [[ "$search_src" != "opensearch" || -z "$search_hit" ]]; then
  fail "opensearch" "source=$search_src hit=$search_hit body=$search_resp"
fi
ok "opensearch" "venue search source=opensearch id=$search_hit"

# --- mysql-backup ---
$COMPOSE exec -T mysql-backup bash /backup.sh
backup_count=$($COMPOSE exec -T mysql-backup sh -c 'ls -1 /backups/jukespotify-*.sql.gz 2>/dev/null | wc -l' | tr -d ' ')
[[ "${backup_count:-0}" -ge 1 ]] || fail "mysql-backup" "no dump files in /backups"
latest=$($COMPOSE exec -T mysql-backup sh -c 'ls -1t /backups/jukespotify-*.sql.gz 2>/dev/null | head -1')
[[ -n "$latest" ]] || fail "mysql-backup" "could not locate latest dump"
ok "mysql-backup" "found $backup_count dump file(s); latest $(basename "$latest")"

# --- kafka ---
session_payload='{"action":"started","session_id":999001,"venue_id":1}'
vote_payload='{"session_id":1,"patron_id":1,"track_id":"smoke-track"}'
payment_payload='{"paid_skip_id":1,"voting_session_id":1}'
echo "$session_payload" | $COMPOSE exec -T kafka /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server localhost:9092 --topic jukespotify.session >/dev/null
echo "$vote_payload" | $COMPOSE exec -T kafka /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server localhost:9092 --topic jukespotify.vote >/dev/null
echo "$payment_payload" | $COMPOSE exec -T kafka /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server localhost:9092 --topic jukespotify.payment >/dev/null
sleep 3
if ! $COMPOSE logs api-1 2>&1 | tail -300 | grep -q 'kafka session: action=started session_id=999001'; then
  $COMPOSE logs api-2 2>&1 | tail -300 | grep -q 'kafka session: action=started session_id=999001' || fail "kafka" "session event not consumed by API"
fi
topics=$($COMPOSE exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list 2>/dev/null | grep '^jukespotify\.' || true)
for want in jukespotify.session jukespotify.vote jukespotify.payment; do
  echo "$topics" | grep -qx "$want" || fail "kafka" "topic missing: $want (have: $topics)"
done
extra=$(echo "$topics" | grep -v -E '^jukespotify\.(vote|payment|session)$' || true)
if [ -n "$extra" ]; then
  fail "kafka" "unexpected jukespotify topics: $extra"
fi
ok "kafka" "session event consumed; only jukespotify.* event topics"

# --- prometheus ---
sleep 5
up_count=$(curl -sf "http://localhost:9090/api/v1/query?query=up%7Bjob%3D%22jukespotify-api%22%7D" | jq '[.data.result[] | select(.value[1]=="1")] | length')
[[ "$up_count" -ge 2 ]] || fail "prometheus" "expected 2 up API targets, got $up_count"
metric=$(curl -sf "http://localhost:9090/api/v1/query?query=juke_http_requests_total" | jq -r '.data.result[0].metric.__name__ // empty')
[[ "$metric" == "juke_http_requests_total" ]] || fail "prometheus" "juke_http_requests_total not scraped"
ok "prometheus" "2 API targets up; juke_http_requests_total present"

# --- grafana ---
gf_pass=$(curl -sf -u admin:admin http://localhost:3000/api/datasources 2>/dev/null | jq '[.[] | select(.type=="prometheus" or .type=="loki" or .type=="tempo")] | length')
[[ "$gf_pass" == "3" ]] || fail "grafana" "expected 3 datasources, got $gf_pass"
ok "grafana" "Prometheus, Loki, Tempo datasources provisioned"

# --- loki ---
loki_ok=0
for _ in $(seq 1 30); do
  if curl -sf http://localhost:3100/ready 2>/dev/null | grep -q '^ready$'; then
    loki_ok=1
    break
  fi
  sleep 2
done
[[ "$loki_ok" == "1" ]] || fail "loki" "not ready ($(curl -s http://localhost:3100/ready 2>/dev/null || echo timeout))"
log_lines=0
for _ in $(seq 1 20); do
  log_lines=$(curl -sf -G "http://localhost:3100/loki/api/v1/query" \
    --data-urlencode 'query={job="docker"}' \
    --data-urlencode "limit=5" | jq '.data.result | length' 2>/dev/null || echo 0)
  if [ "${log_lines:-0}" -ge 1 ]; then
    break
  fi
  sleep 3
done
[[ "${log_lines:-0}" -ge 1 ]] || fail "loki" "no api container logs in query"
ok "loki" "docker job logs queryable"

# --- tempo ---
curl -sf --max-time 5 http://localhost:8081/health >/dev/null
api_traces=0
kafka_traces=0
for _ in $(seq 1 25); do
  api_traces=$(curl -sf "http://localhost:3200/api/search?limit=100" | jq '[.traces[] | select(.rootServiceName=="juke-api" and .rootTraceName != "kafka.consume")] | length' 2>/dev/null || echo 0)
  kafka_traces=$(curl -sf "http://localhost:3200/api/search?limit=100" | jq '[.traces[] | select(.rootTraceName=="kafka.consume")] | length' 2>/dev/null || echo 0)
  if [ "${api_traces:-0}" -ge 1 ] && [ "${kafka_traces:-0}" -ge 1 ]; then
    break
  fi
  sleep 3
done
[[ "${api_traces:-0}" -ge 1 ]] || fail "tempo" "no juke-api HTTP traces found"
[[ "${kafka_traces:-0}" -ge 1 ]] || fail "tempo" "no kafka.consume traces from API consumer"
ok "tempo" "juke-api HTTP traces and kafka.consume spans"

echo ""
echo -e "${GREEN}All smoke checks passed.${NC}"
