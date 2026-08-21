# External Web Client Provider

Kessoku does not bundle, download, proxy, or modify a browser-based RustDesk
client. A browser client can only be described as an independently hosted
external provider. The provider is disabled by default.

This interface is a governance and launch descriptor, not an authorization or
SSO protocol. Kessoku never sends an access token, cookie, user identity,
address book, server key, or session state to the provider origin.

## Configuration

```yaml
web-client-provider:
  mode: external
  # Deployment-only evidence that the operator is authorized to use this
  # provider. This value is validated but is never returned by the API.
  authorization-record: "approval ticket SEC-1234; reviewed 2026-08-18"
  manifest:
    id: "approved-browser-client"
    name: "Approved Browser Client"
    launch-url: "https://client.example.com/app"
    allowed-origin: "https://client.example.com"
    license: "Apache-2.0"
    source-url: "https://git.example.com/clients/browser-client"
    version: "1.2.3"
    digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
```

`mode` accepts only `disabled` and `external`. In `external` mode:

- all eight manifest fields and `authorization-record` are required;
- launch, origin, and source URLs must use HTTPS and cannot contain
  credentials, queries, or fragments;
- `allowed-origin` must be an origin only and must exactly match the launch
  URL origin;
- `digest` must be a lowercase SHA-256 digest;
- startup fails on malformed configuration.

The obsolete `app.web-client` setting must remain `0`. Any non-zero value is a
startup error; it cannot restore removed routes.

## API and launch behavior

When enabled, an authenticated Kessoku user may read:

```text
GET /api/admin/config/web-client-provider
```

The response contains exactly the public manifest fields:

```text
id, name, launch_url, allowed_origin,
license, source_url, version, digest
```

The response is marked `Cache-Control: no-store`. The browser may open
`launch_url` as an ordinary, independent HTTPS page. Kessoku has no launch,
callback, token-exchange, asset-proxy, or provider-session endpoint.

When disabled, the manifest endpoint is not registered. Historical routes
such as `/webclient`, `/webclient2`, `/webclient-config/index.js`,
`/api/shared-peer`, `/api/server-config`, and `/api/server-config-v2` are not
registered in either mode.

## Security invariants

- The provider must be independently licensed and operated by the deployer.
- A manifest or authorization record does not prove that the deployer has a
  license; operators must retain the underlying evidence.
- Provider files must not enter Kessoku images, releases, Debian packages, or
  runtime resource directories.
- Kessoku local storage and cookies are not shared with the provider origin.
- Query-string bearer tokens and implicit token injection are prohibited.
- A future SSO design requires a separately reviewed, short-lived
  authorization-code flow with PKCE and exact redirect URI matching. No such
  flow exists in this version.
- The removed WebClient2 assets and any mechanism intended to bypass a license
  check or takedown are outside this project's scope.

Before enabling a provider, independently verify the source revision,
license, artifact digest, hosting origin, content-security policy, and incident
owner. Re-verify the digest and update the governance record on every version
change.
