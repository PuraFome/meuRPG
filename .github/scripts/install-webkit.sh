#!/usr/bin/env bash
# Installs WebKit for the Playwright version e2e/package-lock.json pins, with its system packages, and does not
# depend on a package mirror when the cache has them.
#
#   install-webkit.sh <deb-dir>     run from e2e/
#
# <deb-dir> holds the .deb files the cache restores. The packages in it that are not installed yet are installed
# from there, with no `apt-get update`. A cached file never replaces a package that is already installed.
# The mirror is used only when Playwright still reports a missing package; the archives it downloads are kept in
# <deb-dir>, so the job saves them to the cache. The script prints which path it took.
set -euo pipefail

deb_dir="${1:?usage: install-webkit.sh <deb-dir>}"

SUDO=
[ "$(id -u)" = 0 ] || SUDO=sudo

mkdir -p "$deb_dir/partial"

# The cached packages that are not installed.
todo=()
for deb in "$deb_dir"/*.deb; do
  [ -e "$deb" ] || continue
  pkg="$(dpkg-deb -f "$deb" Package)"
  if dpkg-query -W -f='${Status}' "$pkg" 2>/dev/null | grep -q 'install ok installed'; then
    continue
  fi
  todo+=("$deb")
done

if [ "${#todo[@]}" -gt 0 ]; then
  $SUDO apt-get install -y --no-download --no-install-recommends "${todo[@]}"
fi

# Playwright lists what is missing and exits non-zero when something is.
if npx playwright install-deps --dry-run webkit; then
  echo "WebKit system packages: from the cache"
else
  echo "WebKit system packages: from the mirror"
  # Keep what apt downloads in <deb-dir>, readable by the user that saves the cache.
  $SUDO tee /etc/apt/apt.conf.d/99webkit-cache >/dev/null <<CONF
Binary::apt::APT::Keep-Downloaded-Packages "true";
APT::Keep-Downloaded-Packages "true";
Dir::Cache::Archives "$deb_dir";
CONF
  npx playwright install-deps webkit
  $SUDO chmod -R a+rX "$deb_dir"
  $SUDO chown -R "$(id -u):$(id -g)" "$deb_dir"
fi

npx playwright install webkit
