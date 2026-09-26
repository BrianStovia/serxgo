#!/usr/bin/env bash
# ==============================================================================
# 🪐 SearXGo Universal Linux/macOS Uninstaller Script
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

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/searxgo"
SYSTEMD_SERVICE="/etc/systemd/system/searxgo.service"
SYSTEM_USER="searxgo"

PURGE=false
AUTO_YES=false

# Parse arguments
for arg in "$@"; do
    case "$arg" in
        --purge|-p)
            PURGE=true
            ;;
        --yes|-y)
            AUTO_YES=true
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -p, --purge    Remove configuration files (/etc/searxgo) and system user"
            echo "  -y, --yes      Assume yes for all prompts (non-interactive)"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *)
            ;;
    esac
done

if [ "${SEARXGO_PURGE:-false}" = "true" ] || [ "${SEARXGO_PURGE:-false}" = "1" ]; then
    PURGE=true
fi

echo -e "${CYAN}${BOLD}"
echo "=================================================================="
echo "🪐 SearXGo - Uninstaller for Linux & macOS"
echo "=================================================================="
echo -e "${RESET}"

# 1. Setup Privilege Escalation Runner
USE_SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        USE_SUDO="sudo"
    else
        INSTALL_DIR="${HOME}/.local/bin"
        CONFIG_DIR="${HOME}/.config/searxgo"
    fi
fi

run_elevated() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif [ -n "${USE_SUDO}" ]; then
        ${USE_SUDO} "$@"
    else
        "$@"
    fi
}

# 2. Confirmation prompt if running interactively
if [ "$AUTO_YES" = false ] && [ -t 0 ]; then
    echo -e "${YELLOW}This script will remove SearXGo from your system.${RESET}"
    read -rp "Are you sure you want to proceed with uninstallation? [y/N]: " confirm
    case "$confirm" in
        [yY][eE][sS]|[yY])
            ;;
        *)
            echo -e "${RED}Uninstallation aborted by user.${RESET}"
            exit 0
            ;;
    esac

    if [ "$PURGE" = false ]; then
        read -rp "Do you also want to purge configuration (/etc/searxgo) and the searxgo user? [y/N]: " purge_confirm
        case "$purge_confirm" in
            [yY][eE][sS]|[yY])
                PURGE=true
                ;;
            *)
                PURGE=false
                ;;
        esac
    fi
fi

# 3. Stop and Remove Systemd Service (Linux)
if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet searxgo.service 2>/dev/null || [ -f "${SYSTEMD_SERVICE}" ]; then
        echo -e "${CYAN}➜ Stopping and disabling systemd service (searxgo.service)...${RESET}"
        run_elevated systemctl stop searxgo.service 2>/dev/null || true
        run_elevated systemctl disable searxgo.service 2>/dev/null || true
        if [ -f "${SYSTEMD_SERVICE}" ]; then
            echo -e "${CYAN}➜ Removing ${SYSTEMD_SERVICE}...${RESET}"
            run_elevated rm -f "${SYSTEMD_SERVICE}"
            run_elevated rm -rf "/etc/systemd/system/searxgo.service.d" 2>/dev/null || true
            run_elevated systemctl daemon-reload
            run_elevated systemctl reset-failed 2>/dev/null || true
        fi
        echo -e "${GREEN}✓ Systemd service removed successfully.${RESET}"
    fi
fi

# 4. Stop any lingering searxgo process
echo -e "${CYAN}➜ Checking for running searxgo processes...${RESET}"
if command -v pkill >/dev/null 2>&1; then
    run_elevated pkill -x searxgo 2>/dev/null || true
fi

# 5. Remove Executable Binary
echo -e "${CYAN}➜ Removing binary files...${RESET}"
BINARY_FOUND=false

TARGET_BINS=(
    "${INSTALL_DIR}/searxgo"
    "/usr/local/bin/searxgo"
    "${HOME}/.local/bin/searxgo"
    "/opt/searxgo/searxgo"
)

# Also check command -v if still resolving
if command -v searxgo >/dev/null 2>&1; then
    TARGET_BINS+=("$(command -v searxgo)")
fi

for bin in "${TARGET_BINS[@]}"; do
    if [ -f "$bin" ]; then
        echo -e "  Removing: ${bin}"
        run_elevated rm -f "$bin"
        BINARY_FOUND=true
    fi
done

if [ "$BINARY_FOUND" = true ]; then
    echo -e "${GREEN}✓ SearXGo binary deleted.${RESET}"
else
    echo -e "${YELLOW}ℹ No binary found in standard locations.${RESET}"
fi

# 6. Purge or Retain Configuration Files
if [ "$PURGE" = true ]; then
    echo -e "${CYAN}➜ Purging configuration and runtime directories...${RESET}"
    for cfg in "${CONFIG_DIR}" "/etc/searxgo" "${HOME}/.config/searxgo"; do
        if [ -d "$cfg" ]; then
            echo -e "  Purging: ${cfg}"
            run_elevated rm -rf "$cfg"
        fi
    done

    # Remove system user if exists
    if id -u "${SYSTEM_USER}" >/dev/null 2>&1; then
        echo -e "${CYAN}➜ Removing system user '${SYSTEM_USER}'...${RESET}"
        if command -v deluser >/dev/null 2>&1; then
            run_elevated deluser --system "${SYSTEM_USER}" 2>/dev/null || true
        elif command -v userdel >/dev/null 2>&1; then
            run_elevated userdel -r "${SYSTEM_USER}" 2>/dev/null || run_elevated userdel "${SYSTEM_USER}" 2>/dev/null || true
        fi
        echo -e "${GREEN}✓ System user removed.${RESET}"
    fi

    echo -e "${GREEN}✓ All configuration and data purged.${RESET}"
else
    if [ -d "${CONFIG_DIR}" ]; then
        echo -e "${YELLOW}ℹ Configuration preserved at ${CONFIG_DIR}.${RESET}"
        echo -e "  To remove configuration manually, run: ${BOLD}sudo rm -rf ${CONFIG_DIR}${RESET}"
    fi
fi

echo ""
echo -e "${GREEN}${BOLD}==================================================================${RESET}"
echo -e "${GREEN}${BOLD}🎉 SearXGo has been successfully uninstalled.${RESET}"
echo -e "${GREEN}${BOLD}==================================================================${RESET}"
