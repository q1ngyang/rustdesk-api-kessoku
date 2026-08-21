# Web Client

**English** | [简体中文](ZH-CN-Web-Client.md)

Kessoku v2.8.0 does not include a browser remote-desktop client. The embedded
`admin-web/` application is the management console only; it is not WebClient2
and cannot establish a RustDesk desktop session.

## What is implemented

The default `web-client-provider.mode` is `disabled`. In this mode Kessoku
registers no browser-client route or provider manifest.

An operator who independently licenses, reviews, hosts, and maintains a
browser client may choose `external` mode. This enables one read-only endpoint
for authenticated Kessoku users:

```text
GET /api/admin/config/web-client-provider
```

The response is marked `Cache-Control: no-store` and contains only these
reviewed public manifest fields:

```text
id, name, launch_url, allowed_origin,
license, source_url, version, digest
```

Configuration additionally requires an `authorization-record`, but Kessoku
never returns that deployment record through the API. The launch and source
URLs must be absolute HTTPS URLs without credentials, query, or fragment; the
launch origin must exactly match `allowed_origin`; and the artifact digest must
use lowercase `sha256:<64 hexadecimal characters>`. Invalid external-provider
configuration stops startup.

## What is not implemented

The external-provider interface is a launch and governance descriptor, not a
hosting, authorization, or SSO protocol. Kessoku does not:

- fetch, bundle, serve, modify, or proxy provider assets;
- inject an access token into a URL, header, script, or provider session;
- share Kessoku cookies, local storage, address books, user identity, or server
  keys with the provider origin;
- expose a launch callback, token exchange, implicit login, or SSO endpoint;
- restore the removed `resources/web`, `resources/web2`, WebClient2,
  `/api/shared-peer`, `/api/server-config`, or `/api/server-config-v2` paths; or
- implement a licence-check or takedown bypass.

The obsolete `app.web-client` setting must remain `0`; a non-zero value is a
startup error. A future SSO design would require a separately reviewed,
short-lived authorization-code flow with PKCE and exact redirect URI matching.
No such flow exists in v2.8.0.

## Operator responsibility

Before enabling `external`, verify and retain evidence for the provider's
licence, source revision, built artifact digest, hosting origin, content
security policy, update process, and incident owner. Re-verify the digest and
approval record on every provider version change. Provider availability and
remote-session compatibility are not part of the Kessoku v2.8.0 support
promise.

The complete configuration example and validation rules are also recorded in
[`WEB-CLIENT-PROVIDER.md`](../../WEB-CLIENT-PROVIDER.md).
