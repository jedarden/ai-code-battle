# AI Code Battle — Requirements

## Overview

AI Code Battle is a competitive bot programming platform inspired by the classic
aichallenge.org Ants game. Players write bots that control units on a shared grid
world. Bots compete in automated tournaments, and game replays are rendered in the
browser for spectators.

## Two Major Components

### 1. Game Engine

The server-side system that orchestrates matches:

- Manages the game simulation (grid world, unit movement, combat, resource gathering)
- Sends game state to player bots each turn via HTTP
- Receives and validates move responses from player bots
- Enforces turn timing (see timeout policy below)
- Records complete game history for replay
- Serves a browser-based replay visualizer

### 2. Players / Participants

Player bots are **HTTP servers**. This is a deliberate architectural choice that
differs from the original aichallenge (which used stdin/stdout subprocesses):

- The game engine sends game state to each bot's HTTP endpoint as a POST request
- The bot responds with its moves in the HTTP response body
- Bots can be written in any language/framework — they just need to serve HTTP
- Bots run on their own infrastructure, completely isolated from the game server
- No untrusted code runs on the game server — eliminates the entire sandboxing problem

**Advantages of HTTP over stdin/stdout:**

- No sandboxing needed — bots are external services, not subprocesses
- Language-agnostic by nature — anything that can serve HTTP can play
- Bots can use any resources they want (GPUs, databases, etc.)
- Scales naturally — the game engine doesn't need to manage bot lifecycles
- Familiar model for developers (REST API)

**Trade-offs:**

- Network latency becomes a factor (mitigated by generous timeout)
- Bots must be hosted somewhere accessible to the game engine
- No CPU-time fairness guarantee (a bot on a beefy machine computes more than one on a Raspberry Pi)

## Communication Protocol

### Schema Enforcement

The request/response schema between game engine and player is **strictly enforced**.
This is a critical security boundary:

- Game state sent to bots follows a fixed JSON schema
- Bot responses must conform to a fixed JSON schema for moves
- Any response that fails schema validation is treated as a no-op (bot's units hold position)
- The game engine never executes, evaluates, or interprets arbitrary data from bots
- Only structured, validated move commands are accepted

### Response Timeout and Inactivity

Each bot has a configurable response budget. The historical baseline and platform
default are **3 seconds**, generous compared to the original aichallenge (1 second
per turn with local execution):

- The default 3 seconds accommodates network round-trip latency
- Bots hosted in different regions can still compete
- Still fast enough that games don't drag (a 500-turn game completes in ~25 minutes worst case)
- If a bot does not respond within its applicable response budget, the response is **ignored** — the bot's units hold position for that turn
- Bots are NOT killed on a single timeout — their units remain alive and hold position, and the bot continues receiving future turns
- A failed turn attempt is either a timeout or a transport/protocol error returned by the bot. Any successfully processed response received before the deadline resets the consecutive-failure count, including a schema-valid response with zero usable moves
- After **10 consecutive failed turn attempts**, the engine marks that bot inactive for the rest of the match. It sends no further requests to that bot; its living units remain in the game and hold position
- Marking one bot inactive does **not** short-circuit the match or remove its units. Other bots continue to receive turns and the normal game win conditions still apply
- The replay records a `bot_inactive` event on the threshold turn with the player, failure count, and cause (`timeout` or `error`), and the final `result.crashed` array records the bot as `true`

**Decision: YES, the response budget is configurable per tournament tier.**

- Casual/beginner: **5 seconds**, forgiving network and hosting variance
- Competitive: **3 seconds**, the standard platform default
- Speed: **1 second**, for optimized bots on fast connections
- Unknown or empty tier: use the **competitive default of 3 seconds**

**Rationale:** Tier-specific budgets balance accessibility across network and hosting variance with the pace and optimization expected in each tournament.

## Replay Visualization

Game replays must be viewable in the browser:

- Every completed match produces a replay
- Replays are rendered client-side (canvas or WebGL)
- Features needed:
  - Play / pause / scrub through turns
  - Playback speed control
  - Per-player perspective toggle (show fog of war from one player's view)
  - Full-map omniscient view
  - Score/resource overlay per turn
  - Shareable replay URLs
- Replay data format should be compact (the original aichallenge used a clever
  per-ant movement string encoding that kept replays small)

## Game Design (TBD — Documented Separately)

The specific game rules (grid mechanics, combat, resources, win conditions) will be
designed separately. Key principles from aichallenge research:

- Symmetric maps to eliminate positional advantage
- Fog of war to prevent perfect-information exploitation
- Combat system that rewards positioning, not just unit count
- Resource mechanics that incentivize exploration
- Multiple endgame conditions to prevent stalemates
- Multi-player support (not just 1v1)

## Tournament System (TBD — Documented Separately)

- Continuous matchmaking with TrueSkill or Glicko-2 rating
- Leaderboard with public rankings
- Match history per player
- Submission versioning (players can roll back to previous bot versions)

## Non-Goals (Explicitly Out of Scope)

- Running untrusted code on the game server (bots are external HTTP services)
- Supporting stdin/stdout bot protocol (HTTP only)
- Real-time multiplayer with human players (this is bot-vs-bot only)
- Mobile-first UI (desktop browser is the primary target)
