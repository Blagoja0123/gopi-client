#!/bin/sh
# Install Gopi Client on Linux or macOS:
#
#   curl -fsSL https://raw.githubusercontent.com/Blagoja0123/gopi-client/master/scripts/install.sh | sh
#
# Options (environment variables):
#   VERSION=v1.2.3        install a specific release instead of the latest
#   INSTALL_METHOD=user   Linux: skip the system package and install into
#                         ~/.local without sudo (default: auto)
#
# Linux: installs the .deb (apt) or .rpm (dnf/zypper) so webkit2gtk is pulled
# in automatically; otherwise falls back to a per-user install.
# macOS: copies "Gopi Client.app" into /Applications (or ~/Applications).
set -eu

REPO="${GOPI_REPO:-Blagoja0123/gopi-client}"
VERSION="${VERSION:-latest}"
INSTALL_METHOD="${INSTALL_METHOD:-auto}"
if [ -n "${GOPI_BASE_URL:-}" ]; then
  BASE="$GOPI_BASE_URL" # for testing against a local mirror
elif [ "$VERSION" = latest ]; then
  BASE="https://github.com/$REPO/releases/latest/download"
else
  BASE="https://github.com/$REPO/releases/download/$VERSION"
fi

say() { printf '\033[1m==>\033[0m %s\n' "$*"; }
die() { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }
has() { command -v "$1" >/dev/null 2>&1; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# fetch ASSET: download into $TMP and verify it against SHA256SUMS.
fetch() {
  say "Downloading $1"
  curl -fL --progress-bar -o "$TMP/$1" "$BASE/$1" || die "download failed: $BASE/$1"
  curl -fsSL -o "$TMP/SHA256SUMS" "$BASE/SHA256SUMS" || die "could not download SHA256SUMS"
  want="$(grep " \*\{0,1\}$1\$" "$TMP/SHA256SUMS" | cut -d' ' -f1)"
  [ -n "$want" ] || die "$1 is not listed in SHA256SUMS"
  if has sha256sum; then got="$(sha256sum "$TMP/$1" | cut -d' ' -f1)"
  else got="$(shasum -a 256 "$TMP/$1" | cut -d' ' -f1)"; fi
  [ "$want" = "$got" ] || die "checksum mismatch for $1"
}

# run as root, using sudo when needed.
as_root() {
  if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo "$@"; fi
}

can_root() {
  [ "$(id -u)" -eq 0 ] || has sudo
}

install_user_linux() {
  fetch "gopi-client_linux_$ARCH.tar.gz"
  tar -xzf "$TMP/gopi-client_linux_$ARCH.tar.gz" -C "$TMP"
  bin="$HOME/.local/bin"
  share="$HOME/.local/share"
  mkdir -p "$bin" "$share/applications" "$share/icons"
  install -m 0755 "$TMP/apiclient" "$bin/apiclient"
  install -m 0644 "$TMP/apiclient.png" "$share/icons/apiclient.png"
  sed -e "s|^Exec=.*|Exec=$bin/apiclient|" -e "s|^Icon=.*|Icon=$share/icons/apiclient.png|" \
    "$TMP/gopiclient.desktop" > "$share/applications/gopiclient.desktop"
  has update-desktop-database && update-desktop-database "$share/applications" >/dev/null 2>&1 || true
  say "Installed to $bin/apiclient"
  case ":$PATH:" in *":$bin:"*) ;; *) say "Note: $bin is not on your PATH (the app is still in your launcher)" ;; esac
  if ! ldconfig -p 2>/dev/null | grep -q libwebkit2gtk-4.1; then
    say "Gopi Client needs WebKitGTK 4.1. Install it with your package manager, e.g.:"
    echo "    sudo apt install libwebkit2gtk-4.1-0   |   sudo dnf install webkit2gtk4.1   |   sudo pacman -S webkit2gtk-4.1"
  fi
}

install_linux() {
  case "$(uname -m)" in
    x86_64 | amd64) ARCH=amd64 ;;
    aarch64 | arm64) ARCH=arm64 ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
  if [ "$INSTALL_METHOD" != user ] && can_root; then
    if has apt-get && has dpkg; then
      fetch "gopi-client_linux_$ARCH.deb"
      say "Installing with apt (may ask for your password)"
      chmod 0644 "$TMP/gopi-client_linux_$ARCH.deb" # apt's sandbox user must be able to read it
      as_root apt-get install -y "$TMP/gopi-client_linux_$ARCH.deb"
      return
    elif has dnf; then
      fetch "gopi-client_linux_$ARCH.rpm"
      say "Installing with dnf (may ask for your password)"
      as_root dnf install -y "$TMP/gopi-client_linux_$ARCH.rpm"
      return
    elif has zypper; then
      fetch "gopi-client_linux_$ARCH.rpm"
      say "Installing with zypper (may ask for your password)"
      as_root zypper --non-interactive install --allow-unsigned-rpm "$TMP/gopi-client_linux_$ARCH.rpm"
      return
    fi
  fi
  install_user_linux
}

install_macos() {
  fetch gopi-client_macos_universal.dmg
  mnt="$TMP/mnt"
  mkdir -p "$mnt"
  hdiutil attach -nobrowse -quiet -mountpoint "$mnt" "$TMP/gopi-client_macos_universal.dmg"
  dest=/Applications
  [ -w "$dest" ] || { dest="$HOME/Applications"; mkdir -p "$dest"; }
  rm -rf "$dest/Gopi Client.app"
  ditto "$mnt/Gopi Client.app" "$dest/Gopi Client.app"
  hdiutil detach -quiet "$mnt"
  say "Installed to $dest/Gopi Client.app"
}

has curl || die "curl is required"
case "$(uname -s)" in
  Linux) install_linux ;;
  Darwin) install_macos ;;
  *) die "unsupported OS: $(uname -s). On Windows, use scripts/install.ps1" ;;
esac
say "Done. Launch \"Gopi Client\" from your applications menu."
