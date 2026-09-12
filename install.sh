#!/usr/bin/env bash
# ==============================================================================
# 🪐 SearXGo Universal Linux/macOS Installer Script
# https://github.com/BrianStovia/serxgo
# ==============================================================================

set -euo pipefail

# Visual styling
BOLD="\033[1m"
GREEN="\033[0;32m"
CYAN="\033[0;36m"
YELLOW="\033[1;33m"
RED="\033[0;31m"
RESET="\033[0m"

REPO="BrianStovia/serxgo"
PORT=${SEARXGO_PORT:-8184}
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/searxgo"
SYSTEMD_SERVICE="/etc/systemd/system/searxgo.service"

echo -e "${CYAN}${BOLD}"
echo "=================================================================="
echo "🪐 SearXGo - Privacy Metasearch Engine Auto-Installer"
echo "=================================================================="
echo -e "${RESET}"

# 1. Detect Architecture and OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)
        TARGET_ARCH="amd64"
        ;;
    aarch64|arm64)
        TARGET_ARCH="arm64"
        ;;
    armv7l|armhf)
        TARGET_ARCH="arm"
        ;;
    *)
        echo -e "${RED}❌ Unsupported architecture: ${ARCH}${RESET}"
        exit 1
        ;;
esac

echo -e "${CYAN}➜ Detected Platform:${RESET} ${BOLD}${OS}/${TARGET_ARCH}${RESET}"

# 2. Require root for system installation or fallback to user local bin
USE_SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        USE_SUDO="sudo"
    else
        INSTALL_DIR="${HOME}/.local/bin"
        CONFIG_DIR="${HOME}/.config/searxgo"
        mkdir -p "${INSTALL_DIR}" "${CONFIG_DIR}"
    fi
fi

# 3. Determine installation method: Prebuilt Binary or Local Build
BINARY_NAME="searxgo-linux-${TARGET_ARCH}"
if [ "$OS" = "darwin" ]; then
    BINARY_NAME="searxgo-darwin-${TARGET_ARCH}"
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo -e "${CYAN}➜ Obtaining SearXGo binary...${RESET}"

if [ -f "./searxgo-linux-${TARGET_ARCH}" ]; then
    echo -e "${GREEN}✓ Using local compiled binary: ./searxgo-linux-${TARGET_ARCH}${RESET}"
    cp "./searxgo-linux-${TARGET_ARCH}" "${TMP_DIR}/searxgo"
elif [ -f "./cmd/server/main.go" ] && command -v go >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Compiling from source using Go...${RESET}"
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${TMP_DIR}/searxgo" ./cmd/server
else
    echo -e "${YELLOW}➜ Downloading prebuilt binary for ${OS}/${TARGET_ARCH}...${RESET}"
    MAIN_DIST_URL="https://raw.githubusercontent.com/${REPO}/main/dist/${BINARY_NAME}"
    RELEASE_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
    
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${MAIN_DIST_URL}" -o "${TMP_DIR}/searxgo" 2>/dev/null || curl -fsSL "${RELEASE_URL}" -o "${TMP_DIR}/searxgo" 2>/dev/null || true
    elif command -v wget >/dev/null 2>&1; then
        wget -q "${MAIN_DIST_URL}" -O "${TMP_DIR}/searxgo" 2>/dev/null || wget -q "${RELEASE_URL}" -O "${TMP_DIR}/searxgo" 2>/dev/null || true
    fi

    if [ -s "${TMP_DIR}/searxgo" ]; then
        echo -e "${GREEN}✓ Prebuilt binary downloaded successfully!${RESET}"
    else
        echo -e "${YELLOW}➜ Downloading repository source (${REPO})...${RESET}"
        mkdir -p "${TMP_DIR}/src"
        if command -v git >/dev/null 2>&1; then
            git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src"
        else
            curl -fsSL "https://github.com/${REPO}/archive/refs/heads/main.tar.gz" | tar -xz -C "${TMP_DIR}/src" --strip-components=1
        fi

        # Check if Go is installed on host
        if command -v go >/dev/null 2>&1; then
            echo -e "${GREEN}✓ Building SearXGo using host Go compiler...${RESET}"
            (cd "${TMP_DIR}/src" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "${TMP_DIR}/searxgo" ./cmd/server)
        else
            echo -e "${CYAN}➜ Host system does not have Go installed.${RESET}"
            echo -e "${CYAN}➜ Downloading official portable Go compiler for ${OS}/${TARGET_ARCH}...${RESET}"
            GO_VER="1.22.6"
            GO_TAR="go${GO_VER}.${OS}-${TARGET_ARCH}.tar.gz"
            GO_URL="https://go.dev/dl/${GO_TAR}"
            
            mkdir -p "${TMP_DIR}/go_bootstrap"
            if command -v curl >/dev/null 2>&1; then
                curl -fsSL "${GO_URL}" | tar -xz -C "${TMP_DIR}/go_bootstrap"
            elif command -v wget >/dev/null 2>&1; then
                wget -qO- "${GO_URL}" | tar -xz -C "${TMP_DIR}/go_bootstrap"
            fi

            if [ -x "${TMP_DIR}/go_bootstrap/go/bin/go" ]; then
                echo -e "${GREEN}✓ Compiling SearXGo with portable Go compiler...${RESET}"
                (cd "${TMP_DIR}/src" && CGO_ENABLED=0 "${TMP_DIR}/go_bootstrap/go/bin/go" build -ldflags="-s -w" -o "${TMP_DIR}/searxgo" ./cmd/server)
            fi
        fi

        if [ ! -s "${TMP_DIR}/searxgo" ]; then
            echo -e "${RED}❌ Failed to build SearXGo. Please verify internet connection and retry.${RESET}"
            exit 1
        fi
    fi
fi

chmod +x "${TMP_DIR}/searxgo"

# 4. Install Binary
echo -e "${CYAN}➜ Installing binary to ${INSTALL_DIR}/searxgo...${RESET}"
if [ -n "${USE_SUDO}" ]; then
    ${USE_SUDO} mkdir -p "${INSTALL_DIR}"
    if command -v install >/dev/null 2>&1; then
        ${USE_SUDO} install -m 755 "${TMP_DIR}/searxgo" "${INSTALL_DIR}/searxgo"
    else
        ${USE_SUDO} rm -f "${INSTALL_DIR}/searxgo" 2>/dev/null || true
        ${USE_SUDO} cp "${TMP_DIR}/searxgo" "${INSTALL_DIR}/searxgo"
        ${USE_SUDO} chmod 755 "${INSTALL_DIR}/searxgo"
    fi
else
    mkdir -p "${INSTALL_DIR}"
    if command -v install >/dev/null 2>&1; then
        install -m 755 "${TMP_DIR}/searxgo" "${INSTALL_DIR}/searxgo"
    else
        rm -f "${INSTALL_DIR}/searxgo" 2>/dev/null || true
        cp "${TMP_DIR}/searxgo" "${INSTALL_DIR}/searxgo"
        chmod 755 "${INSTALL_DIR}/searxgo"
    fi
fi

# 5. Install Default settings.yml Configuration
echo -e "${CYAN}➜ Setting up configuration in ${CONFIG_DIR}/settings.yml...${RESET}"
if [ -n "${USE_SUDO}" ]; then
    ${USE_SUDO} mkdir -p "${CONFIG_DIR}"
    if [ -f "./settings.yml" ]; then
        ${USE_SUDO} cp "./settings.yml" "${CONFIG_DIR}/settings.yml"
    else
        ${USE_SUDO} curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/settings.yml" -o "${CONFIG_DIR}/settings.yml" || true
    fi
else
    mkdir -p "${CONFIG_DIR}"
    if [ -f "./settings.yml" ]; then
        cp "./settings.yml" "${CONFIG_DIR}/settings.yml"
    else
        curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/settings.yml" -o "${CONFIG_DIR}/settings.yml" || true
    fi
fi

# 6. Systemd Service Setup (Linux only with root/sudo)
if [ "$OS" = "linux" ] && command -v systemctl >/dev/null 2>&1 && [ -n "${USE_SUDO}" ]; then
    echo -e "${CYAN}➜ Configuring systemd service at ${SYSTEMD_SERVICE}...${RESET}"

    # Create unprivileged system user if not exists
    if ! id -u searxgo >/dev/null 2>&1; then
        ${USE_SUDO} useradd -r -s /bin/false -d "${CONFIG_DIR}" searxgo || true
    fi

    ${USE_SUDO} chown -R searxgo:searxgo "${CONFIG_DIR}" || true

    cat <<EOF | ${USE_SUDO} tee "${SYSTEMD_SERVICE}" >/dev/null
[Unit]
Description=SearXGo Privacy Metasearch Aggregator
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=searxgo
Group=searxgo
WorkingDirectory=${CONFIG_DIR}
ExecStart=${INSTALL_DIR}/searxgo -port ${PORT}
Restart=always
RestartSec=3s
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target
EOF

    ${USE_SUDO} systemctl daemon-reload
    ${USE_SUDO} systemctl enable searxgo.service
    ${USE_SUDO} systemctl restart searxgo.service

    echo -e "${GREEN}${BOLD}✓ Systemd service installed and started!${RESET}"
    echo -e "  Status: ${CYAN}systemctl status searxgo${RESET}"
    echo -e "  Logs:   ${CYAN}journalctl -u searxgo -f${RESET}"
fi

echo ""
echo -e "${GREEN}${BOLD}==================================================================${RESET}"
echo -e "${GREEN}${BOLD}🎉 SearXGo installation complete!${RESET}"
echo -e "➜ Web Interface:  ${CYAN}${BOLD}http://localhost:${PORT}${RESET}"
echo -e "➜ Binary Path:    ${BOLD}${INSTALL_DIR}/searxgo${RESET}"
echo -e "➜ Configuration:  ${BOLD}${CONFIG_DIR}/settings.yml${RESET}"
echo -e "${GREEN}${BOLD}==================================================================${RESET}"
