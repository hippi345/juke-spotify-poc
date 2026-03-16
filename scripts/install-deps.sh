#!/bin/bash
# Install Docker and Go in WSL (Ubuntu)
# Run: bash scripts/install-deps.sh

set -e

echo "=== Installing Docker ==="
if command -v docker &>/dev/null; then
  echo "Docker already installed: $(docker --version)"
else
  curl -fsSL https://get.docker.com -o /tmp/get-docker.sh
  sudo sh /tmp/get-docker.sh
  sudo apt-get install -y docker-compose-plugin
  sudo usermod -aG docker "$USER"
  echo "Docker installed. You may need to log out and back in for group changes."
fi

echo ""
echo "=== Installing Go ==="
if command -v go &>/dev/null; then
  echo "Go already installed: $(go version)"
else
  GO_VERSION="1.22.4"
  GO_ARCH="linux-amd64"
  wget -q "https://go.dev/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" -O /tmp/go.tar.gz
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz

  # Add to PATH if not already there
  if ! grep -q '/usr/local/go/bin' ~/.bashrc 2>/dev/null; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    echo "Added Go to PATH in ~/.bashrc"
  fi
  export PATH=$PATH:/usr/local/go/bin
  echo "Go ${GO_VERSION} installed. Run 'source ~/.bashrc' or open a new terminal."
fi

echo ""
echo "=== Done ==="
echo "Next steps:"
echo "  1. Log out and back in (or run: newgrp docker) for Docker group"
echo "  2. Start Docker: sudo service docker start"
echo "  3. Verify: docker --version && go version"
