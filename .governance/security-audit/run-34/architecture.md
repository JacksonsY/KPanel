# Architecture and scoped source map

## Scope and product

This is a scoped, source-only review of feature commit `c4ef4c5d82589ed8bf94dcce5c1c107c75be65da` (tree `9cf28dd2868fbdcd3152c0158d57e1ab027d2655`) against comparison base `4c0694aa8e02e46145a775707b8d5a0355f7ce10`. The panel is a self-hosted Linux server administration product. Its normal user is an authenticated panel operator; the Settings Guides page shows locally catalogued Bilibili or YouTube links and illustrated help. No source-grounded comparable product baseline was identified.

The feature adds two authenticated GET APIs, a fixed video catalog, provider metadata lookup, an image proxy, and localized video titles/descriptions and fallback artwork. Protected interests include the operator's session and panel process availability/egress. Video titles and thumbnails are public provider content; the feature does not store tenant-owned records or credentials.

## Entry and data paths

- Frontend: `web/src/views/SettingsGuidesView.vue` catalog → `web/src/lib/api.ts` query encoding and same-origin credentials → `/api/v1/tutorials/metadata` and `/api/v1/tutorials/thumbnail` routes in `internal/panel/server.go` → handlers in `internal/panel/tutorial_metadata.go`.
- URL policy: exact supported video hosts, HTTPS, no user info/opaque/RawPath, allowed port, fixed video ID format and at most 2048 decoded URL bytes → fixed provider endpoints. Provider redirects are not followed and the client timeout is 8 seconds (`tutorial_metadata.go:43-49,128-190,237-305`).
- Provider content: JSON is read up to 1 MiB, the optional YouTube page up to 4 MiB, thumbnails up to 2 MiB. Thumbnail URL host/path is platform-allowlisted, and image MIME is checked against body signatures (`tutorial_metadata.go:307-405,408-434`; `site_icons.go:87-104`). Metadata cache is process-local, TTL 6 hours, capped at 128 entries (`tutorial_metadata.go:197-234`).
- Response/UI: metadata and image responses are marked private with six-hour freshness; thumbnail uses a digest ETag. The Vue view renders provider title as interpolation, links only to fixed catalog URLs, and uses local fallback covers (`tutorial_metadata.go:67-125`; `SettingsGuidesView.vue:31-150,196-235`).
- Authentication: both handlers call `requireSession` before parsing or fetching (`tutorial_metadata.go:67-98`; `server.go:1452-1465`). Authenticated read access is intended for the panel operator.

## Boundaries and strongest source controls

The browser-to-route boundary is protected by session authentication and GET-only handlers. The untrusted URL-to-network boundary strips the URL down to a validated platform/ID and uses fixed outbound hosts, rejects redirects, and applies timeout and response limits. Provider response-to-process/UI is bounded and validated for thumbnail host/path, image type, and title size; Vue interpolation is the browser sink control. The route-to-cache boundary uses explicit `private` response directives; actual reverse-proxy/CDN behavior is not visible in the source snapshot.

## Prior coverage and scope limits

Prior compatible ledgers contain no unit or finding for `tutorial_metadata`, Settings Guides, or these route conditions. Runs 10, 30, and 32 cover adjacent but different source patterns; run 33 is incomplete and supplies no matching coverage. The selected companion blocks are HTTP authentication/cache, resource exhaustion, and client-side DOM handling. Data-isolation blocks are excluded because the cache stores public provider metadata, with no private tenant records. This batch selects only the named feature commit; `scope_complete=false` and other pending commits remain unreviewed.

## Execution limits

Target code, tests, builds, fixtures, browsers, and external services are not run. Ubuntu WSL Node 24.18.1 and `O_NOFOLLOW` are present for the two trusted validators. An OS sandbox with verified network isolation, resource limits, and scratch-only writes was not established; any claim requiring execution or deployment behavior must remain blocked or `needs_validation`.
