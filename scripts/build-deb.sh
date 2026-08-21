#!/bin/sh

set -eu
umask 022

if [ "$#" -ne 4 ]; then
    echo "usage: $0 RELEASE_DIR VERSION ARCH OUTPUT_DIR" >&2
    exit 64
fi

release_dir=$1
version=$2
architecture=$3
output_dir=$4

case "$architecture" in
    amd64) ;;
    *)
        echo "unsupported Debian architecture: $architecture" >&2
        exit 64
        ;;
esac

if [ ! -x "$release_dir/kessoku-api" ]; then
    echo "candidate binary is missing or not executable: $release_dir/kessoku-api" >&2
    exit 66
fi
if find "$release_dir" -type l -print -quit | grep -q .; then
    echo "symbolic link entered candidate release tree" >&2
    exit 65
fi

case "$version" in
    *[!0-9A-Za-z.+:~_-]*|'')
        echo "invalid Debian version: $version" >&2
        exit 64
        ;;
esac

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
build_root=$(mktemp -d "${TMPDIR:-/tmp}/kessoku-deb.XXXXXX")
trap 'rm -rf "$build_root"' EXIT HUP INT TERM

package_root="$build_root/kessoku-api"
state_root="$package_root/var/lib/kessoku-api"
mkdir -p \
    "$package_root/DEBIAN" \
    "$package_root/usr/bin" \
    "$package_root/usr/share/doc/kessoku-api" \
    "$package_root/lib/systemd/system" \
    "$state_root" \
    "$output_dir"

install -m 0755 "$release_dir/kessoku-api" "$package_root/usr/bin/kessoku-api"
for directory in conf docs resources; do
    cp -a "$release_dir/$directory" "$state_root/$directory"
done
find "$state_root/conf" "$state_root/docs" "$state_root/resources" \
    -type d -exec chmod 0755 {} +
find "$state_root/conf" "$state_root/docs" "$state_root/resources" \
    -type f -exec chmod 0644 {} +
install -d -m 0700 "$state_root/data" "$state_root/runtime"
install -m 0644 "$repo_root/systemd/kessoku-api.service" \
    "$package_root/lib/systemd/system/kessoku-api.service"
install -m 0644 "$repo_root/debian/copyright" \
    "$package_root/usr/share/doc/kessoku-api/copyright"

cat > "$package_root/DEBIAN/control" <<EOF
Package: kessoku-api
Version: $version
Section: net
Priority: optional
Architecture: $architecture
Maintainer: q1ngyang <q1ngyang@users.noreply.github.com>
Depends: adduser, ca-certificates, init-system-helpers
Homepage: https://github.com/q1ngyang/rustdesk-api-kessoku
Description: Kessoku account and typed Starry control plane for RustDesk
 Kessoku provides accounts, EdDSA token lifecycle, and a versioned typed
 Starry Control API. It does not bundle a browser client or expose arbitrary
 server commands.
EOF

for maintainer_script in postinst prerm postrm; do
    install -m 0755 "$repo_root/debian/kessoku-api.$maintainer_script" \
        "$package_root/DEBIAN/$maintainer_script"
done

if find "$package_root" -type d \( -name web -o -name web2 \) -print -quit \
    | grep -q .; then
    echo "browser-client directory entered Debian package" >&2
    exit 65
fi
if find "$package_root" -type f \
    \( -name '*.key' -o -name '*.pem' -o -name '*.p12' \) -print -quit \
    | grep -q .; then
    echo "private key or certificate bundle entered Debian package" >&2
    exit 65
fi

# Normalize metadata so the same candidate tree and version produce the same
# package bytes with the pinned dpkg implementation.
find "$package_root" -exec touch -h -d '@0' {} +
SOURCE_DATE_EPOCH=0 dpkg-deb --build --root-owner-group -Zgzip \
    "$package_root" "$output_dir/kessoku-api_${version}_${architecture}.deb"
