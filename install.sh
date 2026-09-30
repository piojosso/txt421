#!/bin/sh
# Instala txt421 en macOS o Linux:
#   curl -fsSL https://raw.githubusercontent.com/piojosso/txt421/main/install.sh | sh
#
# Baja el ejecutable de la última versión, verifica su checksum y lo deja en ~/.local/bin
# (o en TXT421_DIR). Si esa carpeta no está en el PATH, la agrega al archivo de tu shell.
set -eu

REPO="piojosso/txt421"
DIR="${TXT421_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
die() { printf 'txt421: %s\n' "$*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "este instalador es para macOS y Linux. En Windows: irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "no hay versión para $(uname -m)" ;;
esac

if command -v curl >/dev/null 2>&1; then
  bajar() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  bajar() { wget -qO "$2" "$1"; }
else
  die "hace falta curl o wget"
fi

archivo="txt421_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/latest/download"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "Bajando txt421 ($os/$arch)…"
bajar "$base/$archivo" "$tmp/$archivo" || die "no pude bajar $base/$archivo"
bajar "$base/checksums.txt" "$tmp/checksums.txt" || die "no pude bajar checksums.txt"

esperado="$(grep " $archivo\$" "$tmp/checksums.txt" | awk '{print $1}')"
[ -n "$esperado" ] || die "$archivo no figura en checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  real="$(sha256sum "$tmp/$archivo" | awk '{print $1}')"
else
  real="$(shasum -a 256 "$tmp/$archivo" | awk '{print $1}')"
fi
[ "$esperado" = "$real" ] || die "la descarga no coincide con su checksum; no se instaló nada"

tar -xzf "$tmp/$archivo" -C "$tmp" txt421
mkdir -p "$DIR"
# mv sobre el anterior (y no cp): así un txt421 abierto sigue andando hasta que se cierre.
mv -f "$tmp/txt421" "$DIR/txt421"
chmod 755 "$DIR/txt421"
if [ "$os" = darwin ]; then
  xattr -d com.apple.quarantine "$DIR/txt421" 2>/dev/null || true
fi

version="$("$DIR/txt421" version 2>/dev/null || echo txt421)"
say "Instalado: $version en $DIR/txt421"

# ¿Está en el PATH?
case ":$PATH:" in
  *":$DIR:"*) say "Listo. Escribí: txt421" ; exit 0 ;;
esac

shell="$(basename "${SHELL:-sh}")"
case "$shell" in
  zsh) rc="$HOME/.zshrc"; linea="export PATH=\"$DIR:\$PATH\"" ;;
  bash)
    rc="$HOME/.bashrc"
    [ "$os" = darwin ] && rc="$HOME/.bash_profile"
    linea="export PATH=\"$DIR:\$PATH\"" ;;
  fish) rc="$HOME/.config/fish/config.fish"; linea="fish_add_path \"$DIR\"" ;;
  *) rc="$HOME/.profile"; linea="export PATH=\"$DIR:\$PATH\"" ;;
esac
if [ -f "$rc" ] && grep -qs "# txt421" "$rc"; then
  :
else
  mkdir -p "$(dirname "$rc")"
  printf '\n%s  # txt421\n' "$linea" >>"$rc"
  say "Agregué $DIR al PATH en $rc."
fi
say "Listo. Abrí una terminal nueva y escribí: txt421"
say "(o ahora mismo: $DIR/txt421)"
