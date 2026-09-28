# Bot HTTP Protocol Conformance

This document is the normative byte-level contract for the engine-to-bot HTTP protocol. The engine produces requests and consumes responses; bots implement the two endpoints and authenticate the exact request bytes they receive.

## Credential Lifecycle

The protocol authenticates two parties who already hold a shared secret; it defines no registration endpoint of its own. Credentials are minted once at registration, delivered once, and rotated or revoked through the platform API. This section documents that workflow; the sections below remain the byte-level contract.

### Identity and material

- A registered bot holds exactly one credential pair: a bot identifier and a shared secret.
- The bot identifier is minted at registration as `b_` followed by 12 lowercase hexadecimal characters (6 random bytes), for example `b_4e8c1d2f9a03`. It travels verbatim in `X-ACB-Bot-Id` and is public: it appears in match records, replays, and the leaderboard.
- The shared secret is 256 bits of CSPRNG output rendered as 64 lowercase hexadecimal characters (`engine.GenerateSecret` in this repository, `generateSecret` in `cmd/acb-api`). It is the sole authenticator. Every signature in this document is keyed by it and nothing else about a bot's identity participates in verification: `X-ACB-Bot-Id` is not covered by any signature, so a verifier that wants identifier hygiene must check the header itself. Possession of the secret is what verification attests.

### Registration

`POST /api/register` with a JSON body:

```json
{
  "name": "my-bot",
  "owner": "my-handle",
  "endpoint_url": "https://my-bot.example:8080",
  "debug_public": false
}
```

The platform validates the body (missing `name`, `owner`, or `endpoint_url` is `400`), rejects an already-taken `name` with `409`, and probes the endpoint exactly as the engine will: `GET {endpoint_url}/health` must return `200 OK` under the health contract defined above, or registration fails with `400`. On success it mints the identifier and secret, stores the secret, and answers `201 Created`:

```json
{ "bot_id": "b_4e8c1d2f9a03", "shared_secret": "<64 lowercase hex characters>" }
```

The registration response is the only delivery of the secret. There is no read-back endpoint: the platform stores the secret encrypted (see Storage) and can never display it again. Record it at registration time, or rotate to a replacement later.

On the current deployment the match-tier routes — registration, rotation, predictions — answer `503 match_tier_offline` until the `acb-api` backend is redeployed (see `docs/notes/api-transport.md`), and registration is arranged out-of-band with the match coordinator. The workflow in this section is the contract those routes restore.

### Delivery

The secret moves exactly once, from the registration (or rotation) response to the bot operator. It never travels over the turn protocol: requests and responses carry only derived signatures. A secret must not be committed to a repository, pasted into a ticket, or written to a log, and no conformant bot echoes its key material in error output.

### Storage

- **Bot side**, the secret lives in the process environment, read once at boot and never baked into an image layer or a commit. Strategy bots and most starter templates read `BOT_SECRET`; the Go, Python, and Rust starters historically read `SHARED_SECRET`. The spellings mean the same value, and the conformance harness (`conformance/targets.go`) injects whichever spelling a target reads. A bot that boots without its secret must fail fast and refuse to serve `/turn`.
- **Platform side**, the secret is stored AES-256-GCM encrypted under a 32-byte key supplied to the API and the match worker through the environment (`ACB_ENCRYPTION_KEY`; 64 hex or 44 base64 characters). The worker decrypts a match's secrets in memory when the match starts and uses nothing else for the match's lifetime — an in-flight match always completes under the credentials it started with. Running the platform without an encryption key stores plaintext; that is a development configuration only.

### Rotation

`POST /api/rotate-key` authenticates with the current secret and returns a replacement:

```json
{ "bot_id": "b_4e8c1d2f9a03", "shared_secret": "<current secret>" }
```

A `200` response carries `{ "bot_id": "...", "shared_secret": "<new secret>" }`; the current secret being wrong is `401`, an unknown bot is `404`. Rotate on any suspicion of exposure, on operator change, or on a fixed schedule. The procedure is: rotate, update the bot's environment, restart the bot — in that order, between matches. In the window between rotating and restarting, the two sides hold different secrets: the engine's requests fail the bot's verification and the bot's responses fail the platform's, and the protocol scores both as failed turns (see Invalid Responses). Nothing narrows that window except performing the rotation when no match is scheduled.

### Revocation

Revocation is a rotation with `"retire": true`: the stored secret is replaced one last time — invalidating whatever the bot still holds — and the bot's status becomes `retired`. A retired bot can no longer authenticate the rotation endpoint and is excluded from scheduling surfaces that select on active status. Its stale credential authenticates nothing: requests it cannot verify fail as authentication failures (`401` per the Turn Request section), and its responses fail the platform's response verification. Either direction scores failed turns, and ten consecutive failures mark a bot inactive for the rest of a match. Revocation binds new matches immediately; a match already in flight completes under the snapshot taken at its start.

### Local development

Local development needs no registration: the author is both parties, so any non-empty secret works. `SHARED_SECRET=test` in starter quick-starts is a placeholder chosen for the example, not a credential, and a locally chosen secret never reaches the platform. To exercise the real machinery against a bot under development, run it through the conformance harness: it boots the target with its own documented suite key and drives the byte-level contract of this document. Test suites in this repository generate secrets at runtime with `engine.GenerateSecret(rand.Reader)` and assert on verification outcomes, never on secret values; no test embeds key material.

## Transport

- The engine uses `GET /health` without authentication.
- The engine uses `POST /turn` for every game turn.
- The turn body and response body are JSON encoded as UTF-8.
- Paths and methods are exact. Query parameters and redirects do not change the contract.

## Health

`GET /health` returns `200 OK` directly when the bot is ready to receive turns. Redirects are not followed. The response body is not interpreted. Any other status is a failed health check. Health requests do not carry the ACB authentication or `Content-Type` headers.

## Turn Request

The engine sets all of these headers:

| Header | Required value |
| --- | --- |
| `Content-Type` | `application/json` |
| `X-ACB-Match-Id` | The match identifier from the body |
| `X-ACB-Turn` | The non-negative decimal turn from the body |
| `X-ACB-Timestamp` | Current Unix time in seconds as a decimal integer |
| `X-ACB-Bot-Id` | The registered bot identifier |
| `X-ACB-Signature` | Lowercase hexadecimal HMAC-SHA256 described below |

`X-ACB-Turn` and `X-ACB-Timestamp` use canonical base-10 spelling: no sign, whitespace, leading zero, or other alternate representation. The engine always sends canonical spellings, and a conformant bot must accept them. On receiving a non-canonical but well-formed spelling, a bot chooses one of two conformant behaviors: reject the request as an authentication failure (`401`), or accept it by verifying the signature over the exact spelling received and using the parsed integer value. Neither choice is required for conformance; what conformance requires is that a canonical request always executes, and that a non-canonical spelling whose value disagrees with the authenticated body never executes.

The body is one JSON object with these required fields:

```json
{
  "match_id": "m_7f3a9b2c",
  "turn": 42,
  "config": {
    "rows": 40,
    "cols": 40,
    "max_turns": 500,
    "vision_radius2": 49,
    "attack_radius2": 25,
    "spawn_cost": 3,
    "energy_interval": 10,
    "cores_per_player": 2,
    "zone_enabled": true,
    "zone_start_turn": 10,
    "zone_shrink_interval": 1,
    "zone_shrink_step": 1,
    "zone_min_radius": 2,
    "kill_score": 1
  },
  "you": { "id": 0, "energy": 7, "score": 3 },
  "bots": [
    { "position": { "row": 10, "col": 15 }, "owner": 0 }
  ],
  "energy": [
    { "row": 20, "col": 25 }
  ],
  "cores": [
    { "position": { "row": 5, "col": 5 }, "owner": 0, "active": true }
  ],
  "walls": [
    { "row": 10, "col": 10 }
  ],
  "dead": [],
  "zone": {
    "center": { "row": 20, "col": 20 },
    "radius": 10,
    "active": true
  }
}
```

`zone` is included whenever `config.zone_enabled` is true; its `active` value reports whether shrinking is active. It is omitted when zone support is disabled. Optional `config.map_id`, `config.season_id`, `config.rules_version`, and `config.turn_timeout` fields carry match metadata. `turn_timeout` is a duration, not a clock reading: it is the per-turn response budget, encoded as an integer number of nanoseconds (the JSON form of a Go `time.Duration`). It shares no unit or epoch with `X-ACB-Timestamp`, which is an instant in Unix time measured in seconds. All other shown request fields are required. A request is malformed when it is not exactly one JSON object, omits a required field, gives a required field the wrong JSON type, contains an unknown field, or has different `match_id` or `turn` values in the body and headers.

Authentication failures, including a missing required header, a stale or future timestamp, a bad signature, and a body/header identity mismatch, must not execute the request. A syntactically malformed authenticated body must not execute the request either. Implementations should return `401 Unauthorized` for authentication failures and `400 Bad Request` for authenticated malformed input.

## Signature Bytes

There is no JSON canonicalization. The signed body hash covers the exact raw bytes on the wire, including whitespace, member order, escapes, and a final newline if present. Parsing and re-serializing JSON before hashing changes the signature.

The HMAC key is the UTF-8 byte representation of the `SHARED_SECRET` value. Do not hex-decode a textual secret before using it as the key.

1. Compute `SHA-256(raw_body)`.
2. Render the digest as exactly 64 lowercase hexadecimal characters with no `0x` prefix.
3. Construct the canonical request payload from the authenticated header values and that digest:

```text
{match_id}.{turn}.{timestamp}.{body_sha256_hex}
```

4. Encode the canonical payload as UTF-8 with no implicit newline or terminator.
5. Compute `HMAC-SHA256(key, canonical_payload_bytes)` to obtain 32 bytes.
6. Render the HMAC as exactly 64 lowercase hexadecimal characters in `X-ACB-Signature`.

For example, with key `sëcret-🔑`, match `m_conformance`, turn `7`, timestamp `1711200000`, and exact body bytes:

```text
{"match_id":"m_conformance","turn":7}
```

the body digest, canonical payload, and signature are:

```text
866d18ae5b374c10ae5cca15e0c2f0a21b58bff2ccd1a1d2722879080e92cf56
m_conformance.7.1711200000.866d18ae5b374c10ae5cca15e0c2f0a21b58bff2ccd1a1d2722879080e92cf56
e034fde18f2f4416b66b1d2357b280ed82f3deca0fd880e035ed709cd21622fc
```

Changing whitespace, member order, escaping, or a trailing newline changes the body digest even when the parsed JSON is equivalent. Changing any canonical field changes the HMAC input. Verification must decode the supplied 64-character hex signature to 32 bytes and compare it with the expected digest using a constant-time byte comparison. Uppercase hex is non-conformant and must be rejected.

Request verification order is security-sensitive:

1. Require `POST` and `Content-Type: application/json`.
2. Read and retain the unparsed request body bytes.
3. Require all five `X-ACB-*` headers and validate integer spelling and timestamp freshness.
4. Recompute the request HMAC from the retained bytes and reject a mismatch.
5. Decode the authenticated body once, reject a malformed or non-conformant schema, and require its `match_id` and `turn` to equal the headers.
6. Execute bot logic only after every check succeeds.

## Turn Response

A successful response has status `200 OK` and must include `X-ACB-Signature`. It should set `Content-Type: application/json`. The required response shape is:

```json
{
  "moves": [
    { "position": { "row": 5, "col": 5 }, "direction": "N" }
  ],
  "debug": {
    "reasoning": "optional telemetry"
  }
}
```

`moves` is required and must be an array, including an empty array for a deliberate hold. Every move must be an object with integer, non-negative `position.row` and `position.col` fields and a string `direction` equal to `N`, `E`, `S`, or `W`. The string `stay` is accepted and produces a no-op for that unit. `debug` is optional and is the only other recognized top-level field. Unknown additive response fields and nested fields are ignored, but a malformed recognized field invalidates the entire response; no move from that response is applied.

Moves address units by position, not by identifier: the protocol has no unit IDs, and a move's `position` selects the unit standing on that coordinate in the request's `bots` array with `owner` equal to `you.id`. Order adjudication is then mechanical:

- **Omitted orders.** A response may order any subset of the caller's units; a unit given no order holds position, and `"moves": []` is the explicit all-hold.
- **Duplicate orders.** Multiple orders for the same position are resolved by array order: the first is kept, the rest are ignored.
- **Unknown positions.** An order whose position does not hold one of the caller's living units — an empty tile, an enemy unit, a dead unit, or a coordinate beyond the grid — is dropped. A dropped order is not an error: the response is still a successful turn and resets the consecutive-failure counter exactly as an empty `moves` array does. Only a schema-invalid recognized field invalidates the whole response.

`stay` is itself an order to hold, so a stay order never appears among the engine's applied moves.

Acceptance is not execution. Grid adjudication happens after the response is accepted: movement wraps toroidally across the map edges, an order into a blocked tile is ignored so the unit holds, and steering two of the caller's own units into one tile kills both per the self-collision rules. None of that can fail the turn — a turn's outcome is decided entirely by the response's transport, signature, and schema, never by where its accepted orders land.

To sign the response, use the authenticated request's `match_id` and integer `turn`:

```text
{match_id}.{turn}.{sha256_hex(exact_raw_response_body)}
```

For key `sëcret-🔑`, match `m_conformance`, turn `7`, and exact response bytes `{"moves":[]}`, the canonical payload and signature are:

```text
m_conformance.7.4cba52032dfb0839b8138eb84ebcb5bd281253019dfc6efb447d211ac4333f8e
82fe0af54ea110caded8f3147065992368d04cedadb6bb2441daf3d25cb6a1ee
```

The bot must construct the response body first, sign those exact bytes, set the signature header, and then write the same bytes to the response.

## Invalid Responses

A non-200 status, missing or invalid response signature, unreadable body, malformed JSON, a schema-invalid known field, or no fully received response by the per-turn deadline is a failed turn. A failed turn is a no-op: the bot's living units hold position. One failure does not remove the bot. Ten consecutive failed turn attempts mark it inactive for the rest of the match, after which the engine sends no more turn requests and its units continue holding position.

The per-turn deadline is the `config.turn_timeout` budget described above. It is set per tournament tier — 5 seconds casual, 3 seconds competitive, 1 second speed — and an absent field means the 3-second competitive default. The clock starts when the engine begins waiting for the bot's response, and it covers the response as a whole: a body still being written when the deadline passes fails like any other failed turn. A response fully received before the deadline is a successful turn even when `moves` is empty, and resets the consecutive-failure count. A response that misses the deadline is discarded no matter how soon it arrives afterward — the engine never applies a late response to the turn it missed, and never credits it to a later one. A timed-out turn advances the same consecutive-failure counter as every other failed turn, so a bot that misses ten deadlines in a row is marked inactive; the replay's `bot_inactive` event then carries `"reason": "timeout"`.
