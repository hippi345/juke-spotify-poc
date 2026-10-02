#!/usr/bin/env bash
# End-to-end Compose session walk: staff venue + session, patron join, vote, optional Stripe paid skip.
# Run from repo root: ./scripts/compose-session-walk.sh
#
# Brings the production-shaped stack up (same as CI). Spotify/Gemini/Stripe are optional;
# when STRIPE_TEST_SECRET_KEY is unset in the environment, the paid-skip step is skipped.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

COMPOSE="docker compose"
if ! docker compose ps >/dev/null 2>&1; then
  if sudo docker compose ps >/dev/null 2>&1; then
    COMPOSE="sudo docker compose"
  elif ! docker compose version >/dev/null 2>&1; then
    fail "docker compose is not available"
  fi
fi

API="${JUKE_API_BASE:-http://localhost:8081}"
TEMPO="${TEMPO_URL:-http://localhost:3200}"
FIXTURE_PLAYLIST_ID="compose-session-walk-demo"
FIXTURE_PLAYLIST_NAME="Compose session walk demo"
PLAYLIST_ID="$FIXTURE_PLAYLIST_ID"
PLAYLIST_NAME="$FIXTURE_PLAYLIST_NAME"
USE_FIXTURE_PLAYLIST=1

fail() {
  echo -e "${RED}FAIL:${NC} $*"
  exit 1
}
ok() {
  echo -e "${GREEN}OK:${NC} $*"
}
skip() {
  echo -e "${YELLOW}SKIP:${NC} $*"
}

json_post() {
  local url="$1" token="$2" body="$3"
  if [ -n "$token" ]; then
    curl -sf --max-time 30 -X POST "$url" \
      -H "Authorization: Bearer $token" \
      -H 'Content-Type: application/json' \
      -d "$body"
  else
    curl -sf --max-time 30 -X POST "$url" \
      -H 'Content-Type: application/json' \
      -d "$body"
  fi
}

json_get() {
  local url="$1" token="$2"
  curl -sf --max-time 30 -X GET "$url" -H "Authorization: Bearer $token"
}

register_or_login() {
  local email="$1" password="$2" role="$3"
  local resp token
  resp=$(curl -s --max-time 20 -X POST "$API/api/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"$password\",\"role\":\"$role\"}" || true)
  token=$(echo "$resp" | jq -r '.token // empty' | tr -d '\r')
  if [ -z "$token" ]; then
    resp=$(curl -sf --max-time 20 -X POST "$API/api/auth/login" \
      -H 'Content-Type: application/json' \
      -d "{\"email\":\"$email\",\"password\":\"$password\"}")
    token=$(echo "$resp" | jq -r '.token // empty' | tr -d '\r')
  fi
  [ -n "$token" ] || fail "auth for $email ($role)"
  printf '%s' "$token"
}

trace_has_span() {
  local trace_id="$1" span_name="$2"
  local blob
  blob=$(curl -sf --max-time 15 "$TEMPO/api/traces/$trace_id" 2>/dev/null || true)
  [ -n "$blob" ] || return 1
  echo "$blob" | jq -e --arg n "$span_name" '
    [.batches[]?.scopeSpans[]?.spans[]?.name | select(. == $n)] | length > 0
  ' >/dev/null 2>&1
}

wait_for_kafka_log() {
  local pattern="$1"
  local i
  for i in $(seq 1 40); do
    if $COMPOSE logs api-1 api-2 2>&1 | tail -400 | grep -qF "$pattern"; then
      return 0
    fi
    sleep 2
  done
  return 1
}

spotify_env_complete() {
  [ -n "${SPOTIFY_CLIENT_ID:-}" ] && [ -n "${SPOTIFY_CLIENT_SECRET:-}" ] && [ -n "${SPOTIFY_REFRESH_TOKEN:-}" ]
}

spotify_access_token() {
  curl -sf --max-time 30 -X POST "https://accounts.spotify.com/api/token" \
    -u "${SPOTIFY_CLIENT_ID}:${SPOTIFY_CLIENT_SECRET}" \
    -d "grant_type=refresh_token" \
    -d "refresh_token=${SPOTIFY_REFRESH_TOKEN}" | jq -r '.access_token // empty'
}

spotify_search_track_uri() {
  local token="$1" query="$2"
  local enc uri
  enc=$(printf '%s' "$query" | jq -sRr @uri)
  uri=$(curl -sf --max-time 30 -H "Authorization: Bearer $token" \
    "https://api.spotify.com/v1/search?q=${enc}&type=track&limit=1" \
    | jq -r '.tracks.items[0].uri // empty')
  [ -n "$uri" ] || return 1
  printf '%s' "$uri"
}

create_session_walk_spotify_playlist() {
  local token="$1" name="$2"
  local pl_id body uris_json queries q uri
  pl_id=$(curl -sf --max-time 30 -X POST "https://api.spotify.com/v1/me/playlists" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$(jq -nc --arg n "$name" --arg d "Ephemeral playlist for Compose session walk (automation only)" \
      '{name:$n,description:$d,public:false}')" | jq -r '.id // empty')
  [ -n "$pl_id" ] || return 1

  uris_json='[]'
  queries=(
    "track:Mr. Brightside artist:The Killers"
    "track:Yellow artist:Coldplay"
    "track:Blinding Lights artist:The Weeknd"
    "track:Levitating artist:Dua Lipa"
    "track:Don't Stop Believin' artist:Journey"
  )
  for q in "${queries[@]}"; do
    uri=$(spotify_search_track_uri "$token" "$q") || continue
    uris_json=$(echo "$uris_json" | jq --arg u "$uri" '. + [$u]')
  done
  if [ "$(echo "$uris_json" | jq 'length')" -lt 3 ]; then
    return 1
  fi
  body=$(jq -nc --argjson uris "$uris_json" '{uris:$uris}')
  curl -sf --max-time 30 -X POST "https://api.spotify.com/v1/playlists/${pl_id}/items" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$body" >/dev/null || return 1
  printf '%s' "$pl_id"
}

wait_for_tempo_span() {
  local span_name="$1"
  local i trace_id
  for i in $(seq 1 30); do
    while IFS= read -r trace_id; do
      [ -n "$trace_id" ] || continue
      if trace_has_span "$trace_id" "$span_name"; then
        echo "$trace_id"
        return 0
      fi
    done < <(curl -sf --max-time 15 "$TEMPO/api/search?limit=100" | jq -r '.traces[]?.traceID // empty' 2>/dev/null || true)
    sleep 3
  done
  return 1
}

echo "=== Compose session walk ==="

echo "Starting production Compose stack..."
$COMPOSE up -d --build --wait
$COMPOSE ps

lb_health=$(curl -sf --max-time 15 "$API/health")
echo "$lb_health" | jq -e '.status == "ok"' >/dev/null || fail "load balancer health: $lb_health"
ok "Stack healthy at $API"

suffix=$(date +%s)

if spotify_env_complete; then
  spotify_connected=$(curl -sf --max-time 15 "$API/api/spotify/status" | jq -r '.connected // false')
  if [ "$spotify_connected" = "true" ]; then
    walk_token=$(spotify_access_token || true)
    if [ -n "$walk_token" ]; then
      walk_pl_name="Compose session walk ${suffix}"
      if new_pl_id=$(create_session_walk_spotify_playlist "$walk_token" "$walk_pl_name"); then
        PLAYLIST_ID="$new_pl_id"
        PLAYLIST_NAME="$walk_pl_name"
        USE_FIXTURE_PLAYLIST=0
        ok "Venue Spotify connected; session walk will use dedicated playlist id=${PLAYLIST_ID}"
      else
        skip "Spotify credentials present but could not create/populate walk playlist — using session-walk fixture"
      fi
    else
      skip "Spotify credentials present but refresh token exchange failed — using session-walk fixture"
    fi
  else
    skip "Spotify env vars set but API has no connected venue account — using session-walk fixture"
  fi
else
  skip "SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET, and SPOTIFY_REFRESH_TOKEN not all set — using session-walk fixture playlist"
fi

staff_email="session-walk-staff-${suffix}@example.com"
patron_email="session-walk-patron-${suffix}@example.com"
pass="sessionwalkpass123"

echo ""
echo "--- Step 1: Staff registers and creates a venue with a playlist (fixture) ---"
staff_token=$(register_or_login "$staff_email" "$pass" "staff")
ok "Staff registered/logged in"

venue_lat="37.7749"
venue_lng="-122.4194"
venue_name="SessionWalkVenue${suffix}"
venue_resp=$(json_post "$API/api/venues" "$staff_token" \
  "{\"name\":\"$venue_name\",\"latitude\":$venue_lat,\"longitude\":$venue_lng}")
venue_id=$(echo "$venue_resp" | jq -r '.id')
[[ -n "$venue_id" && "$venue_id" != "null" ]] || fail "venue create: $venue_resp"
if [ "$USE_FIXTURE_PLAYLIST" = "1" ]; then
  ok "Venue $venue_id created ($venue_name); playlist id=$PLAYLIST_ID (session-walk fixture)"
else
  ok "Venue $venue_id created ($venue_name); playlist id=$PLAYLIST_ID (venue Spotify)"
fi

echo ""
echo "--- Step 2: Staff starts a session ---"
start_resp=$(json_post "$API/api/voting/session/start" "$staff_token" \
  "{\"venue_id\":$venue_id,\"playlist_id\":\"$PLAYLIST_ID\",\"playlist_name\":\"$PLAYLIST_NAME\",\"refill_threshold\":0}")
session_id=$(echo "$start_resp" | jq -r '.session.id // empty')
[[ -n "$session_id" ]] || fail "session start: $start_resp"
echo "$start_resp" | jq -e '.status == "started"' >/dev/null || fail "session not started: $start_resp"
ok "Voting session $session_id started on playlist $PLAYLIST_ID"

echo ""
echo "--- Step 3: Patron registers, finds nearby session, joins (no password) ---"
patron_token=$(register_or_login "$patron_email" "$pass" "patron")
ok "Patron registered/logged in"

$COMPOSE exec -T redis redis-cli DEL "jukespotify:nearby:${venue_lat}:${venue_lng}:2000" >/dev/null 2>&1 || true
nearby=$(json_get "$API/api/venues/nearby?lat=${venue_lat}&lng=${venue_lng}" "$patron_token")
nearby_hit=$(echo "$nearby" | jq -r --argjson vid "$venue_id" '.sessions[]? | select(.venue.id == $vid) | .session_id' | head -1)
[[ "$nearby_hit" == "$session_id" ]] || fail "nearby sessions missing venue $venue_id: $nearby"
requires_pw=$(echo "$nearby" | jq -r --argjson vid "$venue_id" '.sessions[]? | select(.venue.id == $vid) | .requires_password' | head -1)
[[ "$requires_pw" == "false" ]] || fail "expected open join, requires_password=$requires_pw"
ok "Patron found nearby session $session_id (no password)"

join_resp=$(json_post "$API/api/venues/${venue_id}/join" "$patron_token" '{}')
echo "$join_resp" | jq -e '.status == "joined" and (.session_id|tonumber) == '"$session_id" >/dev/null \
  || fail "join failed: $join_resp"
ok "Patron joined session $session_id"

echo ""
echo "--- Step 4: Patron votes for a playlist track (candidate) ---"
state=$(json_get "$API/api/voting/state" "$patron_token")
vote_track=$(echo "$state" | jq -r '.candidates[0].id // empty')
[[ -n "$vote_track" ]] || fail "no vote candidates in state: $state"
vote_resp=$(json_post "$API/api/voting/vote" "$patron_token" "{\"track_id\":\"$vote_track\"}")
echo "$vote_resp" | jq -e '.status == "voted"' >/dev/null || fail "vote failed: $vote_resp"
state_after=$(json_get "$API/api/voting/state" "$patron_token")
votes_for=$(echo "$state_after" | jq -r --arg t "$vote_track" '.votes[$t] // 0')
[[ "${votes_for:-0}" -ge 1 ]] || fail "vote count not reflected: $state_after"
ok "Patron voted for $vote_track (votes=$votes_for)"

kafka_vote_pat="kafka vote: session=${session_id}"
wait_for_kafka_log "$kafka_vote_pat" \
  || fail "Kafka vote event not observed in API logs for session $session_id"
ok "Kafka consumer logged vote for session $session_id"

vote_http_trace=""
vote_kafka_trace=""
if vote_http_trace=$(wait_for_tempo_span "vote.publish"); then
  ok "Tempo trace $vote_http_trace contains vote.publish (API)"
else
  fail "Tempo: no trace with vote.publish span"
fi

wait_for_tempo_kafka_vote() {
  local track="$1"
  local i trace_id blob
  for i in $(seq 1 30); do
    while IFS= read -r trace_id; do
      [ -n "$trace_id" ] || continue
      blob=$(curl -sf --max-time 15 "$TEMPO/api/traces/$trace_id" 2>/dev/null || true)
      if [ -n "$blob" ] && echo "$blob" | grep -q 'kafka.consume' && echo "$blob" | grep -q "$track"; then
        echo "$trace_id"
        return 0
      fi
    done < <(curl -sf --max-time 15 "$TEMPO/api/search?limit=100" | jq -r '.traces[]?.traceID // empty' 2>/dev/null || true)
    sleep 3
  done
  return 1
}

if vote_kafka_trace=$(wait_for_tempo_kafka_vote "$vote_track"); then
  ok "Tempo trace $vote_kafka_trace contains kafka.consume for vote track (Kafka)"
else
  fail "Tempo: no kafka.consume trace for vote track $vote_track"
fi

echo ""
echo "--- Step 5: Paid skip (Stripe test mode, optional) ---"
if [ -z "${STRIPE_TEST_SECRET_KEY:-}" ]; then
  skip "STRIPE_TEST_SECRET_KEY not in environment — paid-skip payment and payment traces skipped"
elif [ -z "${STRIPE_WEBHOOK_SECRET:-}" ]; then
  skip "STRIPE_WEBHOOK_SECRET not in environment — paid-skip payment and payment traces skipped"
else
  if [ "$USE_FIXTURE_PLAYLIST" = "1" ]; then
    paid_track="walk-track-echo"
  else
    paid_track=$(json_get "$API/api/voting/playlist-overview" "$patron_token" \
      | jq -r --arg voted "$vote_track" '.tracks[]?.track.id | select(. != $voted)' | head -1)
    [[ -n "$paid_track" ]] || fail "no alternate playlist track for paid skip (real Spotify playlist)"
  fi
  overview=$(json_get "$API/api/voting/playlist-overview" "$patron_token")
  echo "$overview" | jq -e --arg t "$paid_track" '.tracks[]? | select(.track.id == $t)' >/dev/null \
    || fail "paid-skip track $paid_track not on playlist overview"
  checkout_resp=$(json_post "$API/api/paid-skip/checkout" "$patron_token" "{\"track_id\":\"$paid_track\"}")
  checkout_id=$(echo "$checkout_resp" | jq -r '.session_id // empty')
  [[ -n "$checkout_id" ]] || fail "checkout create: $checkout_resp"
  echo "$checkout_resp" | jq -e '.price_usd == "1.00"' >/dev/null || fail "unexpected checkout price: $checkout_resp"
  ok "Stripe Checkout session created for \$1.00 USD (id prefix ${checkout_id:0:8}…)"

  stripe_api="https://api.stripe.com/v1"
  session_json=$(curl -sf --max-time 30 -u "${STRIPE_TEST_SECRET_KEY}:" \
    "$stripe_api/checkout/sessions/${checkout_id}?expand[]=payment_intent")
  pay_status=$(echo "$session_json" | jq -r '.payment_status // empty')
  pi_id=$(echo "$session_json" | jq -r '.payment_intent.id // .payment_intent // empty')
  if [ "$pay_status" != "paid" ] && [ -n "$pi_id" ] && [[ "$pi_id" == pi_* ]]; then
    curl -sf --max-time 30 -u "${STRIPE_TEST_SECRET_KEY}:" \
      -X POST "$stripe_api/payment_intents/${pi_id}/confirm" \
      -d "payment_method=pm_card_visa" >/dev/null
    session_json=$(curl -sf --max-time 30 -u "${STRIPE_TEST_SECRET_KEY}:" \
      "$stripe_api/checkout/sessions/${checkout_id}")
    pay_status=$(echo "$session_json" | jq -r '.payment_status // empty')
  fi
  [[ "$pay_status" == "paid" ]] || fail "Stripe checkout session not paid (status=$pay_status)"

  event_payload=$(jq -nc --argjson obj "$session_json" \
    '{id:"evt_compose_session_walk",object:"event",type:"checkout.session.completed",data:{object:$obj}}')
  ts=$(date +%s)
  signed_payload="${ts}.${event_payload}"
  whsec_raw="${STRIPE_WEBHOOK_SECRET#whsec_}"
  sig=$(printf '%s' "$signed_payload" | openssl dgst -sha256 -hmac "$whsec_raw" -binary | xxd -p -c 256)
  wh_resp=$(curl -s --max-time 30 -X POST "$API/api/stripe/webhook" \
    -H "Content-Type: application/json" \
    -H "Stripe-Signature: t=${ts},v1=${sig}" \
    -d "$event_payload")
  echo "$wh_resp" | jq -e '.received == true' >/dev/null || fail "webhook failed: $wh_resp"
  ok "Paid skip fulfilled via Stripe test checkout + webhook"

  wait_for_kafka_log "kafka payment: session=${session_id}" \
    || fail "Kafka payment event not observed in API logs for session $session_id"
  ok "Kafka consumer logged payment event"

  payment_http_trace=""
  if payment_http_trace=$(wait_for_tempo_span "payment.publish"); then
    ok "Tempo trace $payment_http_trace contains payment.publish (API)"
  else
    fail "Tempo: no payment.publish span after paid skip"
  fi

  wait_for_tempo_kafka_payment() {
    local track="$1"
    local i trace_id blob
    for i in $(seq 1 30); do
      while IFS= read -r trace_id; do
        [ -n "$trace_id" ] || continue
        blob=$(curl -sf --max-time 15 "$TEMPO/api/traces/$trace_id" 2>/dev/null || true)
        if [ -n "$blob" ] && echo "$blob" | grep -q 'kafka.consume' && echo "$blob" | grep -q "$track"; then
          echo "$trace_id"
          return 0
        fi
      done < <(curl -sf --max-time 15 "$TEMPO/api/search?limit=100" | jq -r '.traces[]?.traceID // empty' 2>/dev/null || true)
      sleep 3
    done
    return 1
  }

  if payment_kafka_trace=$(wait_for_tempo_kafka_payment "$paid_track"); then
    ok "Tempo trace $payment_kafka_trace contains kafka.consume for paid skip track (Kafka)"
  else
    fail "Tempo: no kafka.consume trace for paid skip track $paid_track"
  fi

  paid_overview=$(json_get "$API/api/voting/playlist-overview" "$patron_token")
  echo "$paid_overview" | jq -e --arg t "$paid_track" '.tracks[]? | select(.track.id == $t and .played == true)' >/dev/null \
    || fail "paid-skip track not marked played in session playlist overview: $paid_overview"
  ok "Session playlist overview shows paid-skip track $paid_track as played"
fi

echo ""
echo -e "${GREEN}Compose session walk passed.${NC}"
