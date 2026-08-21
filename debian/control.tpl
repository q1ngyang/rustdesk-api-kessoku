Source: rustdesk-api-kessoku
Section: net
Priority: optional
Maintainer: q1ngyang <q1ngyang@users.noreply.github.com>
Build-Depends: debhelper (>= 10), pkg-config
Standards-Version: 4.5.0
Homepage: https://github.com/q1ngyang/rustdesk-api-kessoku/

Package: kessoku-api
Architecture: {{ ARCH }}
Depends: adduser, systemd, ${misc:Depends}
Conflicts: rustdesk-api-server
Description: Kessoku account and control plane for RustDesk
 Kessoku provides accounts, EdDSA token lifecycle, and a typed Starry control
 API. It does not bundle a browser client or expose arbitrary server commands.
