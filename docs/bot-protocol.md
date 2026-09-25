# Bot HTTP Protocol Conformance

This document is the normative byte-level contract for the engine-to-bot HTTP protocol. The engine produces requests and consumes responses; bots implement the two endpoints and authenticate the exact request bytes they receive.

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

A non-200 status, missing or invalid response signature, unreadable body, malformed JSON, or schema-invalid known field is a failed turn. A failed turn is a no-op: the bot's living units hold position. One failure does not remove the bot. Ten consecutive failed turn attempts mark it inactive for the rest of the match, after which the engine sends no more turn requests and its units continue holding position.
