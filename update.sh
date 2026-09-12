#!/usr/bin/env bash
# ==============================================================================
# 🪐 SearXGo Universal Linux/macOS Update Script
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
DEFAULT_INSTALL_DIR="/usr/local/bin"
DEFAULT_CONFIG_DIR="/etc/searxgo"
SYSTEMD_SERVICE="searxgo.service"

echo -e "${CYAN}${BOLD}"
echo "=================================================================="
echo "🪐 SearXGo - Auto-Updater for Linux & macOS"
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

# 2. Locate existing installation
INSTALL_PATH=""
if command -v searxgo >/dev/null 2>&1; then
    INSTALL_PATH="$(command -v searxgo)"
elif [ -f "/usr/local/bin/searxgo" ]; then
    INSTALL_PATH="/usr/local/bin/searxgo"
elif [ -f "${HOME}/.local/bin/searxgo" ]; then
    INSTALL_PATH="${HOME}/.local/bin/searxgo"
elif [ -f "/opt/searxgo/searxgo" ]; then
    INSTALL_PATH="/opt/searxgo/searxgo"
else
    INSTALL_PATH="${DEFAULT_INSTALL_DIR}/searxgo"
fi

INSTALL_DIR="$(dirname "${INSTALL_PATH}")"
echo -e "${CYAN}➜ Target Binary:${RESET}    ${BOLD}${INSTALL_PATH}${RESET}"

# 3. Determine privilege requirement
USE_SUDO=""
if [ ! -w "${INSTALL_DIR}" ] && [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        USE_SUDO="sudo"
    else
        echo -e "${RED}❌ Write permission denied for ${INSTALL_DIR} and sudo is unavailable.${RESET}"
        exit 1
    fi
fi

# 4. Create temporary workspace
TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

# 5. Acquire latest SearXGo binary
BINARY_NAME="searxgo-linux-${TARGET_ARCH}"
if [ "$OS" = "darwin" ]; then
    BINARY_NAME="searxgo-darwin-${TARGET_ARCH}"
fi

echo -e "${CYAN}➜ Fetching latest SearXGo release...${RESET}"

# Option A: Running from inside git repository clone with Go compiler
if [ -d "./.git" ] && [ -f "./cmd/server/main.go" ] && command -v go >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Updating local git repository and compiling latest source...${RESET}"
    git pull --ff-only origin main || git pull origin main || true
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${TMP_DIR}/searxgo" ./cmd/server
# Option B: Download prebuilt binary from GitHub Releases / Raw Repo
else
    MAIN_DIST_URL="https://raw.githubusercontent.com/${REPO}/main/dist/${BINARY_NAME}"
    RELEASE_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
    
    DOWNLOADED=false
    if command -v curl >/dev/null 2>&1; then
        if curl -fsSL "${MAIN_DIST_URL}" -o "${TMP_DIR}/searxgo" 2>/dev/null || curl -fsSL "${RELEASE_URL}" -o "${TMP_DIR}/searxgo" 2>/dev/null; then
            if [ -s "${TMP_DIR}/searxgo" ]; then
                DOWNLOADED=true
            fi
        fi
    elif command -v wget >/dev/null 2>&1; then
        if wget -q "${MAIN_DIST_URL}" -O "${TMP_DIR}/searxgo" 2>/dev/null || wget -q "${RELEASE_URL}" -O "${TMP_DIR}/searxgo" 2>/dev/null; then
            if [ -s "${TMP_DIR}/searxgo" ]; then
                DOWNLOADED=true
            fi
        fi
    fi

    if [ "$DOWNLOADED" = false ]; then
        echo -e "${YELLOW}➜ Prebuilt binary unavailable. Building latest source from GitHub...${RESET}"
        mkdir -p "${TMP_DIR}/src"
        if command -v git >/dev/null 2>&1; then
            git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src"
        else
            curl -fsSL "https://github.com/${REPO}/archive/refs/heads/main.tar.gz" | tar -xz -C "${TMP_DIR}/src" --strip-components=1
        fi

        if command -v go >/dev/null 2>&1; then
            echo -e "${GREEN}✓ Compiling with host Go compiler...${RESET}"
            (cd "${TMP_DIR}/src" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "${TMP_DIR}/searxgo" ./cmd/server)
        else
            echo -e "${CYAN}➜ Bootstrapping Go compiler for build...${RESET}"
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
                (cd "${TMP_DIR}/src" && CGO_ENABLED=0 "${TMP_DIR}/go_bootstrap/go/bin/go" build -ldflags="-s -w" -o "${TMP_DIR}/searxgo" ./cmd/server)
            fi
        fi
    fi
fi

if [ ! -s "${TMP_DIR}/searxgo" ]; then
    echo -e "${RED}❌ Failed to build or download the latest SearXGo binary.${RESET}"
    exit 1
fi

chmod +x "${TMP_DIR}/searxgo"

# 6. Backup existing binary before replacement (using mv to release active inode)
if [ -f "${INSTALL_PATH}" ]; then
    BACKUP_PATH="${INSTALL_PATH}.bak.$(date +%Y%m%d_%H%M%S)"
    echo -e "${CYAN}➜ Creating backup of current binary at ${BACKUP_PATH}...${RESET}"
    if [ -n "${USE_SUDO}" ]; then
        ${USE_SUDO} mv "${INSTALL_PATH}" "${BACKUP_PATH}"
    else
        mv "${INSTALL_PATH}" "${BACKUP_PATH}"
    fi
fi

# 7. Atomic installation of new executable binary
echo -e "${CYAN}➜ Installing new binary to ${INSTALL_PATH}...${RESET}"
if [ -n "${USE_SUDO}" ]; then
    if command -v install >/dev/null 2>&1; then
        ${USE_SUDO} install -m 755 "${TMP_DIR}/searxgo" "${INSTALL_PATH}"
    else
        ${USE_SUDO} rm -f "${INSTALL_PATH}" 2>/dev/null || true
        ${USE_SUDO} cp "${TMP_DIR}/searxgo" "${INSTALL_PATH}"
        ${USE_SUDO} chmod 755 "${INSTALL_PATH}"
    fi
else
    if command -v install >/dev/null 2>&1; then
        install -m 755 "${TMP_DIR}/searxgo" "${INSTALL_PATH}"
    else
        rm -f "${INSTALL_PATH}" 2>/dev/null || true
        cp "${TMP_DIR}/searxgo" "${INSTALL_PATH}"
        chmod 755 "${INSTALL_PATH}"
    fi
fi

# 8. Check and update configuration template if available (preserve user settings)
if [ -d "${DEFAULT_CONFIG_DIR}" ]; then
    if [ -n "${USE_SUDO}" ]; then
        ${USE_SUDO} curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/settings.yml" -o "${DEFAULT_CONFIG_DIR}/settings.yml.new" 2>/dev/null || true
    fi
fi

# 9. Restart Systemd Service if running
SERVICE_RESTARTED=false
if [ "$OS" = "linux" ] && command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet "${SYSTEMD_SERVICE}" 2>/dev/null || [ -f "/etc/systemd/system/${SYSTEMD_SERVICE}" ]; then
        echo -e "${CYAN}➜ Restarting systemd service (${SYSTEMD_SERVICE})...${RESET}"
        if [ -n "${USE_SUDO}" ]; then
            ${USE_SUDO} systemctl daemon-reload
            ${USE_SUDO} systemctl restart "${SYSTEMD_SERVICE}"
        else
            systemctl daemon-reload 2>/dev/null || true
            systemctl restart "${SYSTEMD_SERVICE}" 2>/dev/null || true
        fi
        SERVICE_RESTARTED=true
        echo -e "${GREEN}✓ Service restarted successfully!${RESET}"
    fi
fi

# 10. Display Completion Summary
echo ""
echo -e "${GREEN}${BOLD}==================================================================${RESET}"
echo -e "${GREEN}${BOLD}🎉 SearXGo updated successfully!${RESET}"
echo -e "➜ Binary:   ${BOLD}${INSTALL_PATH}${RESET}"
if [ "$SERVICE_RESTARTED" = true ]; then
    echo -e "➜ Service:  ${CYAN}systemctl status searxgo${RESET}"
fi
echo -e "${GREEN}${BOLD}==================================================================${RESET}"
