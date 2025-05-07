set -euo pipefail

sudo apt update
sudo apt install -y \
  curl \
  git \
  make \
  protobuf-compiler \
  apt-transport-https \
  ca-certificates \
  gnupg \
  lsb-release

if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
    echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] \
    https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
    sudo apt update
    sudo apt install -y docker-ce docker-ce-cli containerd.io
    sudo groupadd docker 2>/dev/null || true
    sudo usermod -aG docker $USER
    echo "Docker installed. Please log out and log back in to use Docker without sudo."
else
    echo "Docker is already installed."
fi

if ! command -v go >/dev/null 2>&1; then
    echo "Install Go pls :("
    exit 1
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

echo "Setup complete"