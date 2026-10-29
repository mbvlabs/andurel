# Beta.3 live fresh-scaffold gate

status: pass

candidate: `f2e0833236ba98626ad864ac7efedd42bd00e7ca`

Scratch: `/tmp/opencode/andurel-beta3-live-GckeKV` (removed unless --keep-scratch)

## Checks

| Check | Result |
| --- | --- |
| prerequisites | pass |
| candidate CLI | pass |
| 02-new contains Successfully created project | pass |
| fresh scaffold generated | pass |
| candidate pkg modules replaced locally | pass |
| candidate module graph tidied | pass |
| project tools synced | pass |
| frontend dependencies installed | pass |
| production app, migrate, queue, and SSR binaries built | pass |
| disposable PostgreSQL ready on 127.0.0.1:21960 | pass |
| production runtime configuration written | pass |
| migrations applied | pass |
| 11-seeds contains Created admin user: admin@example.com | pass |
| 11-user-check contains 1 | pass |
| verified seed user created | pass |
| production app and SSR sidecar running | pass |
| home-anonymous status 200 | pass |
| home-anonymous has no Location | pass |
| home-anonymous contains "component":"Welcome" | pass |
| home-anonymous contains data-server-rendered="true" | pass |
| home-anonymous contains Space-grade Go, wired locally. | pass |
| home-anonymous contains "isAuthenticated":false | pass |
| signin-anonymous status 200 | pass |
| signin-anonymous has no Location | pass |
| signin-anonymous contains "component":"Auth\/Login" | pass |
| signin-anonymous contains data-server-rendered="true" | pass |
| anonymous home and sign-in are public and server-rendered | pass |
| login status 302 | pass |
| login header Location | pass |
| home-authenticated status 200 | pass |
| home-authenticated contains data-server-rendered="true" | pass |
| home-authenticated contains "isAuthenticated":true | pass |
| home-authenticated excludes Create account | pass |
| signin-authenticated status 200 | pass |
| signin-authenticated contains "component":"Auth\/Login" | pass |
| seeded login and authenticated SSR session preserved | pass |
| ready-initial status 200 | pass |
| ready-initial contains "status":"ready" | pass |
| ready-ssr-outage status 503 | pass |
| ready-ssr-outage contains "status":"unready" | pass |
| ready-ssr-outage contains "reason":"unavailable" | pass |
| home-ssr-outage status 500 | pass |
| home-ssr-outage contains "component":"Errors\/InternalError" | pass |
| home-ssr-outage contains data-page="app" | pass |
| home-ssr-outage contains rel="stylesheet" | pass |
| home-ssr-outage excludes data-server-rendered="true" | pass |
| home-ssr-outage excludes Internal Server Error | pass |
| ready-ssr-recovered status 200 | pass |
| ready-ssr-recovered contains "status":"ready" | pass |
| home-ssr-recovered status 200 | pass |
| home-ssr-recovered contains data-server-rendered="true" | pass |
| home-ssr-recovered contains Space-grade Go, wired locally. | pass |
| ready-db-outage status 503 | pass |
| ready-db-outage contains "status":"unready" | pass |
| ready-db-outage contains "reason":"database_unavailable" | pass |
| ready-db-recovered status 200 | pass |
| ready-db-recovered contains "status":"ready" | pass |
| home-db-recovered status 200 | pass |
| home-db-recovered contains data-server-rendered="true" | pass |
| readiness outage and recovery for SSR and PostgreSQL | pass |
| compiled cmd/ssr exits nonzero on a duplicate listener | pass |
| compiled cmd/ssr exits nonzero after its JavaScript child crashes | pass |
| concurrent isolated builds are independent and leave the source tree untouched | pass |
