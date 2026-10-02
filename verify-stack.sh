#!/bin/bash
# End-to-end stack verification script
# Run from project root: ./verify-stack.sh

set -e
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

COMPOSE="docker compose"
if ! docker compose ps >/dev/null 2>&1; then
  COMPOSE="sudo docker compose"
fi

echo "=== Juke Spotify POC - Stack Verification ==="
echo ""

# 1. MySQL
echo -n "MySQL (localhost:3306): "
if $COMPOSE ps mysql 2>/dev/null | grep -q "Up"; then
  echo -e "${GREEN}running${NC}"
else
  echo -e "${RED}not running${NC} - run: docker compose up -d"
  exit 1
fi

# 2. Redis
echo -n "Redis: "
if $COMPOSE exec -T redis redis-cli ping 2>/dev/null | grep -q PONG; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 3. OpenSearch
echo -n "OpenSearch: "
if $COMPOSE exec -T opensearch curl -sf http://localhost:9200 >/dev/null 2>&1; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 4. Kafka
echo -n "Kafka: "
if $COMPOSE exec -T kafka /opt/kafka/bin/kafka-broker-api-versions.sh --bootstrap-server localhost:9092 >/dev/null 2>&1; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 5. API (via load balancer)
echo -n "API (localhost:8081 via nginx): "
if curl -sf http://localhost:8081/health > /dev/null 2>&1; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 6. Prometheus targets
echo -n "Prometheus (localhost:9090): "
if curl -sf http://localhost:9090/-/healthy > /dev/null 2>&1; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 7. Grafana
echo -n "Grafana (localhost:3000): "
if curl -sf http://localhost:3000/api/health > /dev/null 2>&1; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 8. Loki
echo -n "Loki: "
if $COMPOSE exec -T loki wget -q -O - http://localhost:3100/ready 2>/dev/null | grep -q ready; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 9. Tempo
echo -n "Tempo: "
if $COMPOSE exec -T tempo wget -q -O - http://localhost:3200/ready 2>/dev/null | grep -q ready; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

# 10. Client
echo -n "Client (localhost:5173): "
if curl -sf -o /dev/null http://localhost:5173/ 2>/dev/null; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC}"
  exit 1
fi

echo ""
echo -e "${GREEN}All services running. Stack verified.${NC}"
