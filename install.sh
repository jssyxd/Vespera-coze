#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

print_step() {
  printf "\n==> %s\n" "$1"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf "Missing dependency: %s\n" "$1" >&2
    exit 1
  fi
}

require_cmd python3
require_cmd pip3
require_cmd go

# Check for git (needed for go mod download)
require_cmd git

print_step "Install Python toolchain dependencies"
# Try installing with --break-system-packages (for newer PEP 668 envs), fallback to normal install
if ! python3 -m pip install --upgrade pip --break-system-packages 2>/dev/null; then
    python3 -m pip install --upgrade pip
fi

if ! python3 -m pip install --upgrade slither-analyzer crytic-compile py-solc-x solc-select --break-system-packages 2>/dev/null; then
    python3 -m pip install --upgrade slither-analyzer crytic-compile py-solc-x solc-select
fi

# Ensure user local bin is in PATH for the current script execution
export PATH="$HOME/.local/bin:$PATH"

print_step "Initialize solc-select"
if command -v solc-select >/dev/null 2>&1; then
  # Install a default version (e.g., 0.8.23) to ensure solc command is available
  printf "Installing default solc version (0.8.23)...\n"
  solc-select install 0.8.23 >/dev/null 2>&1 || true
  solc-select use 0.8.23 >/dev/null 2>&1 || true
else
  printf "Warning: solc-select not found in PATH. Skipping initialization.\n" >&2
fi

print_step "Install Solidity compilers (install all versions)"
if [[ ! -f "$ROOT_DIR/scripts/install_all_solc.py" ]]; then
  printf "Missing script: %s\n" "$ROOT_DIR/scripts/install_all_solc.py" >&2
  exit 1
fi

read -r -p "Parallel workers (default: 4): " SOLC_WORKERS
SOLC_WORKERS="${SOLC_WORKERS:-4}"
if ! python3 "$ROOT_DIR/scripts/install_all_solc.py" --workers "$SOLC_WORKERS"; then
  printf "\nWarning: some solc versions failed to install; continuing.\n" >&2
fi

print_step "Install Foundry"
if ! command -v foundryup >/dev/null 2>&1; then
  curl -L https://foundry.paradigm.xyz | bash
fi

if [[ -x "$HOME/.foundry/bin/foundryup" ]]; then
  export PATH="$HOME/.foundry/bin:$PATH"
  "$HOME/.foundry/bin/foundryup"
fi

print_step "Download Go module dependencies"
cd "$ROOT_DIR"
go mod download

print_step "Build Vespera binary"
cd "$ROOT_DIR"
go build -o vespera src/main.go

print_step "Done"
printf "Installation complete! Run with: ./vespera\n"
printf "\nNote: You may need to add ~/.local/bin to your PATH if you haven't already:\n"
printf "  export PATH=\"\$HOME/.local/bin:\$PATH\"\n"
