#!/bin/bash

set -eu

os=$(go env GOOS)

# Install ngrok if it's not there.
if ! command -v ngrok >/dev/null; then
    case $os in
    darwin)
        brew install homebrew/cask/ngrok
        ;;
    windows)
        choco install -y ngrok
        ;;
    linux)
        curl -sSL --fail -o /tmp/ngrok.zip https://bin.equinox.io/c/4VmDzA7iaHb/ngrok-stable-linux-amd64.zip && sudo unzip -o -d /usr/local/bin /tmp/ngrok.zip
        ;;
    esac
fi

# Upgrade various items on various operating systems
case $os in
darwin)
    for item in ddev golang golangci-lint mkcert mkdocs python3-yq; do
        brew upgrade $item || brew install $item || true
    done
    ;;
windows)
    choco upgrade -y golang nodejs markdownlint-cli mkcert mkdocs || true
    ;;
esac

yes | ddev delete images

# Remove any -built images, as we want to make sure tests do the building.
docker rmi -f $(docker images --filter "dangling=true" -q --no-trunc) >/dev/null || true
docker rmi -f $(docker images | awk '/drud.*-built/ {print $3}' ) >/dev/null || true

# Make sure the global internet detection timeout is not set to 0 (broken)
perl -pi -e 's/^internet_detection_timeout_ms:.*$/internet_detection_timeout_ms: 750/g' ~/.ddev/global_config.yaml

# Make sure there aren't any dangling NFS volumes
if docker volume ls | grep nfsmount; then
  docker volume rm -f $(docker volume ls | awk '/[Tt]est.*_nfsmount/ { print $2; }')
fi
