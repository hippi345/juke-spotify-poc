#!/bin/bash
# End-to-end stack verification script
# Run from project root: ./verify-stack.sh

set -e
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Juke Spotify POC - Stack Verification ==="
echo ""

# 1. MySQL
echo -n "MySQL (localhost:3306): "
if docker compose ps mysql 2>/dev/null | grep -q "Up"; then
  echo -e "${GREEN}running${NC}"
else
  echo -e "${RED}not running${NC} - run: docker compose up -d"
  exit 1
fi

# 2. API health
echo -n "API (localhost:8080): "
if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC} - run: cd server && go run ."
  exit 1
fi

# 3. Client
echo -n "Client (localhost:5173): "
if curl -sf -o /dev/null http://localhost:5173/ 2>/dev/null; then
  echo -e "${GREEN}ok${NC}"
else
  echo -e "${RED}not responding${NC} - run: cd client && npm run dev"
  exit 1
fi

echo ""
echo -e "${GREEN}All services running. Stack verified.${NC}"
