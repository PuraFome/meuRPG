# API

Every API call is declared in a `.proto` file, and buf generates the Go and TypeScript code from it.
This page is the contract for how calls are named and versioned, how they fail, how they are repeated safely and how the server protects itself from a client that does too much.

## Contracts

Rules for the files in `proto/meurpg/<module>/v1/`:

1. **One package per module and version:** `meurpg.<module>.v1`.
   The modules and their services follow the code, not a table here: read `proto/`.
2. **Services by subject.** A module has one service as a rule; a separate subject may have its own service in the same package (`campaigns` has `CampaignService` and `CampaignDocumentService`; `play` has several).
   Every call has its own `XxxRequest` and `XxxResponse`, even when empty, as `buf lint` demands.
3. **A field number never changes and is never reused.** A removed field becomes `reserved`.
4. **A breaking change becomes a `v2` package.** CI runs `buf breaking` against `main`.
5. **Generated code is committed** (`backend/gen`, `web/src/gen`).
   CI generates again and fails on any difference.
   `proto/buf.gen.yaml` runs `protoc-gen-go` and `protoc-gen-connect-go` (pinned as `tool` directives in `backend/go.mod`) and `protoc-gen-es` (from `web/node_modules`).
6. **Fields are `snake_case`** in the `.proto`; the generated TypeScript is `camelCase`.
7. **A read-only call declares an idempotency level**, which depends on the request:
   - A request with no ID and no personal data (empty, like `GetMe`, `ListMyCampaigns` and `GetServerInfo`) is `NO_SIDE_EFFECTS`.
     Connect then accepts GET and the browser may cache it.
   - A request with an ID or personal data (like `GetCampaign` or `GetCharacter`) is `IDEMPOTENT`.
     It is a read and safe to repeat, but accepts only POST: in a GET the whole message would be in the URL, and URLs reach the platform logs ([Privacy](../privacy.md)).
   - `TestConnectGETOnlyForRequestsWithoutData` (`backend/cmd/api`) fails if a `NO_SIDE_EFFECTS` method has a request with a field.
8. **Names follow Google AIP.**
   Standard methods start with `Get`, `List`, `Create`, `Update` or `Delete` plus the resource (AIP-131 to 135).
   Custom methods start with the action verb (`AcceptInvite`, `RevokeInvite`; AIP-136).
   A search with a filter is `Search<Resource>`, `IDEMPOTENT`.
   An existing name never changes: renaming an RPC breaks the contract.
9. **Every call that creates something carries an `idempotency_key`** ([below](#idempotency-keys)).
10. **Every unary call requires `Connect-Protocol-Version: 1`** (`connect.WithRequireConnectProtocolHeader()`), which is also a CSRF layer ([Identity](identity.md#csrf)).
    A `curl` needs the header:

```bash
curl -s -X POST http://localhost:8080/meurpg.system.v1.SystemService/GetServerInfo \
  -H 'Content-Type: application/json' -H 'Connect-Protocol-Version: 1' -d '{}'
```

The server reads at most 4 MiB per RPC message (`maxRequestBytes`).

## Error codes

Every refused business rule returns a Connect code, always the same one for the same case.

| Code | When |
| --- | --- |
| `unauthenticated` | No valid sign-in session. |
| `permission_denied` | The caller is a campaign member but lacks the role (a player tries a master action), or the account may not create campaigns (`CampaignCreationRefused`, `NOT_ALLOWED`, RN-30). |
| `failed_precondition` | The rule does not allow it now: a locked sheet (RN-01), a session that has not started, an unusable invite, an image used on a map. When there is more than one reason, a typed detail says which (`InviteUnusable`, `CharacterBlocked`, `GameSessionBlocked`, `XPBlocked`, `XpModeChangeBlocked`, `ImageInUse`...). |
| `not_found` | It does not exist, or the caller may not know it exists: a campaign the caller is not a member of, a map hidden from a player (ADR-0011). A pending member gets the same `not_found` outside the few calls they may make. |
| `invalid_argument` | Invalid input. |
| `resource_exhausted` | A quota ran out: a campaign, gallery or map limit, the account's campaign cap, pending image requests, live streams per person. A rate limit uses the same code with a `Retry-After` header ([Abuse limits](#abuse-limits)). |
| `aborted` | A transaction conflict (`40001`) that persisted after the retries, or a sheet or document that changed since the app read it (stale revision, AIP-154). The app reloads and the person tries again. |
| `deadline_exceeded`, `canceled`, `unavailable`, `unknown`, `internal` | Infrastructure outcomes, in the next table. |

Typed details are messages in the `.proto` of the service that returns them; each `.proto` comment says which reasons a call can give.

### Database errors

A read or write error becomes a Connect code in one place, `platform/rpcerr` (`rpcerr.FromDB`), which every module calls through its `dbError`.

| What happened | Code | Log level |
| --- | --- | --- |
| A Connect error the handler chose | the same, not logged again | none |
| The request was canceled (the client left) | `canceled` | `DEBUG` |
| The request deadline passed, or the database canceled a statement over `statement_timeout` (SQLSTATE `57014`, 30 s by default, `db.defaultStatementTimeout`) | `deadline_exceeded` | `WARN` |
| `db.InTx` retried `40001` until it gave up | `aborted` | `WARN` |
| A commit lost its connection midway (`crdb.AmbiguousCommitError`): the change may or may not be saved | `unknown`, with a sentence telling the client to check before trying again (never `unavailable`: a blind retry of a create without a key could duplicate it) | `ERROR` |
| The database was unreachable (class `08`, `57P01`, `57P03`) | `unavailable` | `ERROR` |
| Anything else | `internal` | `ERROR` |

The client receives a fixed sentence, never the error text (it may hold SQL or a server name); the cause goes to the log line, once.
The error keeps the original as its cause (`Unwrap`), so a method that runs inside another module's `db.InTx` may return it from the closure and `InTx` still sees a `40001` and retries.
A module handles its own special cases first (the row that vanished between the check and the query).

### Panics

`net/http` recovers only from a panic in the handler's own goroutine; a goroutine the server starts that panics takes down the process and every live session with it.

- **Connect handlers** have `connect.WithRecover` right after `rpclog`: the panic becomes `internal` with a fixed sentence, the stack goes to the log (`rpc handler panicked`, with the request id, user and campaign), and `rpclog` still writes the `rpc` line with `code=internal`.
- **Background goroutines** (the image generation request, the live-hint timers) start with `defer safego.Recover(logger, "...")` (`platform/safego`): the work that broke is lost, the stack goes to the log and the server keeps going.

## Idempotency keys

A call that creates something carries an `idempotency_key`, so a second tap, or a retry after a lost response, does not create a second resource (AIP-155).
One mechanism serves them all; the shared code is `platform/idem` (`Clean`, `Scope`, `Hash`, `Create`) and each module adds its columns and queries.

- **The key** is 1 to 64 characters, chosen by the app once per action and repeated on the retry.
  `ActionKey` (`web/src/app/core/connect/idempotency.ts`) keeps the same key while the values stay the same and makes a new one when they change or when the action succeeded.
- **The row stores** `create_key` (the key's owner, a colon and the key, so it is unique within the owner) and `create_hash` (the hash of the whole request minus the key), with a partial unique index on `create_key`.
- **Inside the create's transaction** (`db.InTx`, only `WithTx`), the server looks the key up.
  The same hash returns what the first call did and creates, publishes and counts nothing.
  Another hash is `invalid_argument` ("idempotency_key was used for another change").
  No row means create.
  Two simultaneous calls with the same key create one: the loser's `INSERT ... ON CONFLICT DO NOTHING` returns no row and it reads the winner's.
- **Where the key lives** differs by call (a column on the created row, the hash on a session event, on an XP award, on an image request...).
  `grep idempotency_key proto/` lists the calls; the `.proto` comment of each says what a repeat returns.
- **State changes that are not creates** keep the last key and request hash on the row they change, or take the revision the caller read (`expected_revision`): a call on another revision is refused with `STALE_REVISION` and changes nothing.
- **Exempt on purpose:**
  - Calls that return a secret once (`CreateInvite`, `CreateClaimLink`): only the hash is stored, so a replay could not return the first secret.
    A retry makes a new link, and a new claim link revokes the live one.
  - Calls that only compute from a seed and store nothing (`GenerateTreasure`, `GenerateEncounter`).
  - Calls that are already idempotent by nature: `PlaceMapToken` (put or move), `TakeBackLeftImage`, `TakeOffStage`, `RollAbilityScores` (a second call returns the same dice).
- **The key is optional** on calls that existed without it; the app always sends one.
  Newer writes require it.

## Slow clients

The server has no global `ReadTimeout` or `WriteTimeout`, on purpose: they would cut the streams.
The limit sits on the operation that can hang (`platform/slowclient`):

| Operation | Deadline |
| --- | --- |
| Each `Send` of a live stream | 30 s to reach the client (`streamSendTimeout`). A reader that stopped reading ends the stream as `client_gone`. |
| The body of a unary RPC | 1 minute to arrive (`UnaryBodyTimeout`), cleared once read. A stream's request has none. |
| The body of an image upload | 2 minutes to arrive until the upload takes the processing slot, then 45 s for the file itself (`uploadSlotReadTimeout`): the slot is shared with fog tiles and other image jobs. |
| The `POST /auth/login` form | 10 s (`slowclient.ReadBody`). |
| The body of an image, thumbnail, fog tile or app file | 2 minutes to be written (`slowclient.WriteBody`, cleared when the handler ends). |
| A part of a campaign import | 2 minutes (`partReadTimeout`). |

`ReadHeaderTimeout` (10 s) and `IdleTimeout` (2 minutes) apply to every connection.
A live stream ends by itself after `play.DefaultMaxLifetime` (10 minutes) and the app opens a new one.

## Abuse limits

There are in-memory rate limits and per-account caps.
The rate limits stop a script; none hinders a real table.
All rate limits use `platform/ratelimit`, a token bucket with a per-client and a global limit.
`RATE_LIMIT_MULTIPLIER` scales all of them (default 1; the e2e stack uses 10, see [Operations](../operations.md#abuse-limits)).
The numbers are in `ratelimit.NewPolicy` and in the constants next to each limiter.

| Limit | Key | Where | Answer |
| --- | --- | --- | --- |
| Every API request (`/meurpg.*`, `/images/`, `/uploads/`) | Client IP | `ratelimit.Middleware`, before any session lookup | `429` with `Retry-After` and JSON `{"code":"resource_exhausted","reason":"RATE_LIMITED"}` |
| Connect calls and stream opens | Signed-in user | Interceptor right after the session one (`limitedSessions` in `cmd/api`) | `resource_exhausted` with `Retry-After` |
| Images, thumbnails and fog tiles (downloads) | User | `maps.Mount`, after the session | `429`, reason `RATE_LIMITED` |
| Fog tiles that need rendering | User | `maps/tiles.go`, stricter than the download limit | the same |
| Image uploads (`POST /uploads/images`) | User | `maps.Mount` | the same |
| Campaign package: start an export, begin, preview and create an import, download an export | User | `campaignpackage`, in each handler | `resource_exhausted` with `Retry-After` (`429` on the download) |
| Parts of an import upload | User | `campaignpackage` | `429`, reason `RATE_LIMITED` |
| `/auth/login` and `/auth/callback` | Client IP | `identity`, first thing in each handler ([Identity](identity.md#login-attempt-limit)) | `429` with `Retry-After`, plain text |
| `PreviewClaim` and `ClaimCharacter` | User, and client IP | `characters`, first thing in each handler ([Campaigns](campaigns.md#claim-links)) | `resource_exhausted` with `Retry-After` |

- **The IP limit comes before the session** because a request with no session, or with an invented cookie, costs a database read to be refused.
  It counts everything on the API with or without a session, so its value is generous.
  The per-user limit holds back a member who repeats an expensive call.
- **Outside the limit:** the static app and `/healthz` touch no database.
  `/readyz` does, but is not limited by IP: Cloud Run's probes can share an address, and refusing a probe takes the instance out of service.
  It is bounded instead by sharing one database ping among all callers per 2 seconds (`pingCache` in `platform/httpserver`), so a change in database state shows up to 2 seconds late.
- **A call with no session** passes the per-user limit (the IP limit covered it and the handler refuses it).
  A stream spends one token on opening, not per message.
- **Each refusal logs one `WARN`** (`rate limit hit`) with the limit's name and the `user_id`, at most one per 10 s per limit, with the count of those left out.
  Never with the IP ([Privacy](../privacy.md)).
- **The client IP** comes from the connection (`RemoteAddr`) locally.
  On Cloud Run it is the item of `X-Forwarded-For` that `TRUSTED_PROXY_HOPS` counts from the right (1 on Cloud Run alone, 2 behind the load balancer): Google appends the address of whoever connected after what the client sent, so only what comes from the right is trustworthy.
  Setting the variable off Cloud Run is a configuration error.
- **The web app does not retry on its own** a refused call.
  `rateLimitInterceptor` (`web/src/app/core/connect/transport.ts`) turns a `resource_exhausted` with `Retry-After` into `unavailable` with "Muitas ações em pouco tempo. Espere N segundos e tente de novo.", which every screen treats as "try again", never as the final refusal of a full gallery (also `resource_exhausted`).
  The exceptions are images: a `<img>` cannot read `Retry-After`, so a fog tile answered `429` or `503` is asked again after 2, 4, 8, 16 and 30 s, and the map picture and gallery thumbnails do the same through the `appRetryImage` directive (`shared/retry-image`).
- **One instance only:** the counters live in process memory.
  With two instances each would have its own.

### What an account or a campaign may hold (RN-30)

| Cap | Value and where it is set | Refusal |
| --- | --- | --- |
| Campaigns an account is master of | `MAX_CAMPAIGNS_PER_USER`, default 10 (`campaigns.DefaultMaxCampaignsPerUser`). The local stacks and CI use `off`; the API refuses `off` on Cloud Run. | `resource_exhausted` with `CampaignCreationRefused` `LIMIT_REACHED` |
| Who may create campaigns | `CAMPAIGN_CREATORS`: verified e-mails, comma-separated; empty means anyone. Someone invited to a campaign plays without being on the list. | `permission_denied` with `NOT_ALLOWED` |
| Invites that still work in a campaign | 50 (`campaigns.MaxActiveInvites`): not revoked, not expired, with uses left | `resource_exhausted` |
| Characters and NPCs in a campaign, living or dead | 1,000 (`characters.DefaultMaxCharactersPerCampaign`) | `resource_exhausted` |
| Images generated per campaign per month, and per server per Brasília day | `IMAGE_MONTHLY_LIMIT` and `IMAGE_DAILY_LIMIT` ([Operations](../operations.md#generated-images-the-gemini-api)) | `failed_precondition` with reason `DAILY_LIMIT_REACHED` for the daily cap |

Each count runs in the transaction of the create it guards, with `WithTx(tx)`, so under `SERIALIZABLE` two creates racing for the last place conflict and the one that retries counts again.
The campaign cap reads verified e-mails through `identity` (`PostgresStore.VerifiedEmails`); `campaigns` never reads the users table.
The gallery, map and point limits belong to the `maps` module.
