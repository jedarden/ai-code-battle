# Ants AI Challenge (aichallenge.org) — Comprehensive Research

## 1. History and Context

The Ants AI Challenge was the **Fall 2011** edition of the AI Challenge, a series of programming competitions organized by the **University of Waterloo Computer Science Club** with sponsorship from **Google**. The competition was codenamed **"Epsilon"** internally.

### Timeline
- **Fall 2010**: Planet Wars (based on Galcon) — the predecessor competition, a 1v1 game
- **October 2011**: Ants announced and launched
- **December 2011**: Competition concluded; results published
- **After 2011**: The AI Challenge project went dormant; no further competitions were run

### Scale
- Approximately **7,900 submissions** during the competition
- Top 100 contestants recognized
- Winner: **Mathis Lichtenberger** ("xathis") from University of Lubeck, Germany
- Runner-up: **Evgeniy Voronyuk** ("GreenTea") from Dnipropetrovsk National University, Ukraine
- Third place: **"protocolocon"** from Spain

### GitHub Repository
- `github.com/aichallenge/aichallenge` — 640 stars, 170 forks, 2,843 commits
- Components: game engine (`ants/`), tournament manager (`manager/`), workers (`worker/`), website, SQL schemas, integration testing

---

## 2. Game Mechanics

### Map / Grid

- **Toroidal grid**: rectangular, wraps both horizontally and vertically (no edges)
- Characters: `.` = land, `%` = water (impassable), `*` = food, `a-j` = player ants, `A-J` = ants on own hill, `0-9` = hill locations
- **Dimensions**: up to 200x200, with 900-5000 area per player, total area capped at 25,000
- **Symmetry**: all maps were symmetric — every player's starting position mirrors every other
- **Map generation constraints**:
  - 2-10 player support
  - Hills must be 20-150 steps apart, with Euclidean distance minimum of 6
  - Must have a path between all hills traversable by a 3x3 block (no narrow chokepoints that block army movement)
  - No islands allowed

### Turn Structure

Each turn follows a strict sequence:

1. **Send game state** to all players (filtered by fog of war)
2. **Receive orders** from all players (within time limit)
3. **Execute six phases** in order:
   - **Move** — execute player movement orders
   - **Attack** — resolve combat between adjacent enemy ants
   - **Raze Hills** — enemy ant on a hill location destroys it
   - **Spawn Ants** — new ants emerge from hills (if hive has food)
   - **Gather Food** — ants within spawn radius collect food
   - **Spawn Food** — new food appears on the map
4. **Check endgame conditions**

### Fog of War

- Each player has **individual vision** — only squares within `viewradius2` (default: 77, i.e., ~8.77 tiles Euclidean) of any owned ant are visible
- Vision is updated incrementally as ants move, spawn, or die
- **Water** is only transmitted to a bot the first turn it becomes visible (permanent terrain)
- **Enemy ants** are only visible within the player's vision radius
- Player identifiers are remapped: you always see yourself as player 0, enemies numbered sequentially by discovery order
- Bots do NOT know the total number of players

### Food Mechanics

**Spawning:**
- Each game has a hidden food rate: `food_extra += (food_rate * num_players) / food_turn` per turn
- `food_rate`: 5-11 total food; `food_turn`: 19-37 turns per cycle
- Initial food: 2-5 items placed within each player's starting vision, plus distributed food based on `food_start` (75-175 per land area ratio)
- **Four spawning algorithms**: none, random, sections (BFS-based region splitting), symmetric (maintains rotational/reflective symmetry — the one used in competition)
- Food is distributed across "symmetric square sets" that shuffle randomly; every set spawns at least once before repeating
- Equidistant squares (from multiple players) form smaller sets
- Strategic implication: the best way to gather the most food is to explore and cover the most area

**Harvesting:**
- Occurs after combat resolution each turn
- If **all ants** within `spawnradius2` (default: 1) of food belong to **one player**, that player harvests it into their "hive"
- If ants from **multiple players** are within spawn radius, the food is **destroyed** — benefits nobody
- Each harvested food produces exactly 1 ant

**Ant Spawning:**
- Each food in the hive spawns 1 ant at a hill
- Hill must be unrazed and unoccupied
- One ant spawns per hill per turn maximum
- Priority: hills longest without activity get spawns first (ties broken randomly)
- Players can block spawning by keeping an ant on a hill

### Combat Resolution

The competition offered four selectable combat algorithms. The one used in the actual competition was **"focus"** (also called "occupied"):

**Focus Algorithm:**
```
for every ant:
  for each enemy in range of ant (using attackradius2):
    if enemies_in_range(ant) >= enemies_in_range(enemy):
      ant is marked dead
```

- `attackradius2` default: **5** (Euclidean distance ~2.24 tiles)
- An ant dies when its "focus" (number of enemies it faces) is greater than or equal to any opponent's focus
- All deaths are resolved **simultaneously** — no mid-turn recalculation
- Key properties:
  - **Superior numbers win without casualties** — 2v1 kills the lone ant with no friendly losses
  - **Defensive formations are powerful** — a small concentrated force can inflict heavy losses (the "Sparta" effect)
  - **Multi-faction battles create chaos** — third parties can benefit from letting opponents weaken each other
  - **Locally deterministic** — only requires knowledge of immediate surroundings

**Other algorithms available (not used in main competition):**
- **Support**: ant survives if friendly count >= enemy count in range
- **Closest**: iterative BFS elimination; ants in groups >1 all die (discourages concentration)
- **Damage**: each ant deals `1/enemy_count` damage; ants receiving >= 1 total damage die

### Hill Mechanics

- Each player starts with 1+ hills; initial ant spawned at each
- **Razing**: a hill is destroyed when an enemy ant occupies its location after the attack phase (no defending ant present)
- Razing awards **+2 points** to the razer, **-1 point** to the hill owner
- Razed hills stop spawning but the player continues playing with remaining ants
- Even with all hills razed, ants can still move, attack, gather food, and raze enemy hills

### Winning Conditions / Endgame

Five ways a game can end:

1. **Lone Survivor**: only one player has living ants. Bonus: +2 points per unrazed enemy hill, -1 to their owners
2. **No Players Remain**: all eliminated simultaneously
3. **Food Not Gathered (Cutoff)**: food sits at >= 90% of combined food+ant count for 150 turns — targets inactive bots
4. **Dominance Without Hill Razing (Cutoff)**: one player has >= 85% of total food+ants for 150 turns without razing — assumes winner can't lose
5. **Rank Stabilized**: no remaining player can mathematically improve their ranking
6. **Turn Limit**: default 1000 turns (calibrated so 90-95% of matches end via this rule)

### Scoring

- Each player starts with 1 point per hill owned
- Razing an enemy hill: +2 points
- Losing a hill: -1 point
- If you never attack and lose all hills: 0 points (incentivizes aggression)
- Lone survivor bonus: +2 per remaining unrazed enemy hill
- Crashed/timed-out bots: surviving opponents still receive hill-destruction bonus points

### Collision Rules

- Two friendly ants ordered to the same square: **both die**
- Your ant ordered onto an enemy ant's square: **both die** (before attack radius applies)
- Moves into water or food: **ignored** (ant stays in place)

---

## 3. Bot Communication Protocol

### Transport

Bots communicate via **stdin/stdout**. The game engine launches each bot as a subprocess and pipes data through standard streams. Stderr is captured for logging but not processed.

### Initialization (Turn 0)

The engine sends:
```
turn 0
loadtime <ms>
turntime <ms>
rows <height>
cols <width>
turns <max_turns>
viewradius2 <squared_distance>
attackradius2 <squared_distance>
spawnradius2 <squared_distance>
player_seed <seed>
ready
```

The bot performs initialization and responds with:
```
go
```

### Per-Turn Input

Each subsequent turn, the engine sends:
```
turn <turn_number>
w <row> <col>          # water (only first time visible)
f <row> <col>          # food location
a <row> <col> <owner>  # live ant (owner 0 = self)
h <row> <col> <owner>  # hill
d <row> <col> <owner>  # dead ant (visible or own)
go
```

All coordinates are 0-indexed. Only information within the player's fog of war is transmitted.

### Bot Output (Orders)

Bots issue movement commands:
```
o <row> <col> <direction>
go
```

Where direction is one of: `N`, `E`, `S`, `W`. Each ant can receive at most one order. Ants without orders remain stationary.

### Game End

When the game ends, bots receive:
```
end
<final state data>
players <num_players>
score <p1_score> <p2_score> ... <pN_score>
go
```

### Timing

- **Load time**: default 3000ms — time allowed for initialization after receiving setup data
- **Turn time**: default 1000ms — time allowed per turn to compute and send orders
- **Timeout behavior**: if a bot exceeds the time limit, it is killed. The bot's status is recorded as "timeout" in the replay. Its ants remain on the map but receive no further orders (effectively becoming stationary targets)
- **Crash handling**: similar to timeout — bot removed from active play, ants become inert
- **Timing measurement**: the platform discussed both CPU time and wall-clock time. The recommended approach was a hybrid: base timeout on CPU time with a 10x multiplied wall-clock backup limit. Server-side monitoring checked ~50 times per turn

### Distance Calculation

All distance checks (vision, attack, spawn) use **squared Euclidean distance** with wrapping:
```
dr = min(abs(a.row - b.row), rows - abs(a.row - b.row))
dc = min(abs(a.col - b.col), cols - abs(a.col - b.col))
distance_squared = dr*dr + dc*dc
```

---

## 4. Tournament / Ranking System

### Rating Algorithm: TrueSkill

The platform used **Microsoft's TrueSkill** rating system via the **JSkills 0.9.0** Java library:
- Each player has **mu** (mean skill estimate) and **sigma** (uncertainty)
- Display rating: mu - 3*sigma (conservative estimate)
- After each game, mu and sigma are updated based on finish order
- Multi-player games supported natively by TrueSkill
- The manager ran `TSUpdate` as a Java subprocess, passing player data and receiving updated ratings

### Matchmaking Algorithm

1. **Seed selection**: pick the bot that hasn't played in the longest time (tiebreak by user_id)
2. **Match size**: determined by the player count the seed has played in least
3. **Opponent selection**: uses a **Pareto distribution** to select skill-rank distance — 80% of matches are within 16 ranks of the seed
4. **Fairness filters**: exclude bots with excessive recent games to keep game counts even
5. **Pairing priority** (in order):
   - Least recent pairing with selected opponent (24-hour window)
   - Fewest total games played recently
   - Best TrueSkill match quality (computed as a 0-1 score from mu, sigma, and beta)
6. **Map selection**: least played by all opponents
7. **Position assignment**: random player slot assignment

### Game Size

- Maps supported **2-10 players** per game
- The matchmaker selected game size based on what the seed player had least experience with

### Match Frequency

Games ran **continuously** via the worker pool. As soon as a worker completed a game, it requested a new task. With the worker fleet running, new ratings appeared "within minutes" of code submission.

### Leaderboard

- Updated periodically via `generate_leaderboard` stored procedure
- Configurable refresh interval
- Displayed mu-3*sigma as the public rating

---

## 5. Replay System

### Two Formats

1. **Streaming format**: used for real-time viewing, ~10x larger than storage format
2. **Storage format**: JSON-based, used for downloads and archival

### Storage Format (JSON) Structure

```json
{
  "challenge": "ants",
  "replayformat": "json",
  "playernames": ["bot1", "bot2", ...],
  "playerstatus": ["survived", "eliminated", "timeout", "crash"],
  "submitids": [123, 456],
  "user_ids": [1, 2],
  "user_url": "http://aichallenge.org/profile.php?user=~",
  "game_url": "http://aichallenge.org/game.php?game=~",
  "date": "2011-12-01T12:00:00Z",
  "game_id": 12345,
  "playercolors": ["#ff0000", "#0000ff"],
  "replaydata": {
    "revision": 2,
    "players": 2,
    "turns": 1000,
    "loadtime": 3000,
    "turntime": 1000,
    "viewradius2": 77,
    "attackradius2": 5,
    "spawnradius2": 1,
    "map": {
      "rows": 100,
      "cols": 100,
      "data": [".%%.....a...", "...%%..b....", ...]
    },
    "ants": [
      [row, col, start_turn, conversion_turn, end_turn, player_id, "moves_string"]
    ],
    "scores": [[1.0, 1.0], [1.0, 3.0], ...],
    "bonus": [0, 2]
  }
}
```

- `ants` array: each element tracks an ant's lifecycle — position, birth turn, conversion turn, death turn, owner, and a string of movement commands (`-` = stay, `n`/`e`/`s`/`w` = direction)
- `scores`: 2D array of floating-point values per player per turn
- Map characters: `.` = land, `%` = water, `*` = food, `a-z` = starting ant positions

### Visualizer

- **Template-based HTML generation**: `replay.html.template` produced self-contained replay pages
- **JavaScript rendering** with UglifyJS for minification
- **Rhino** (server-side JS engine) for processing
- **Java components** for core visualization logic
- **Local development**: `visualize_locally.py` for testing
- Visualizers verified `challenge == "ants"` and `replayformat == "json"` before parsing

---

## 6. Infrastructure

### Architecture

Four core components:

1. **Website** (PHP/Apache): user registration, code submission, leaderboard display, replay viewing
2. **Auto-compile system**: detected language by file extension, compiled submissions, generated `run.sh` execution scripts
3. **Tournament manager** (Python): matchmaking, game scheduling, TrueSkill updates, leaderboard generation
4. **Workers** (distributed): standalone game executors with compilation and sandboxing

### Worker System

- **OS**: Ubuntu 11.04
- **Deployment**: automated via shell script — `curl http://server/api_server_setup.php?api_create_key=KEY | sh` — approximately 25 minutes to install
- **Communication**: JSON over HTTP (GET for task assignment, POST for results)
- **Task flow**:
  1. Worker requests task via `api_get_task.php`
  2. Server assigns compilation job or game matchup
  3. Worker downloads submissions (with hash verification) and maps
  4. Worker compiles/runs the game
  5. Worker posts results via `api_compile_result.php` or `api_record_game.php`
- **Idempotency**: unique `post_id` per request prevents duplicate processing
- **Error recovery**: failed games reassigned to different workers
- **Database**: MySQL

### Sandboxing

- **Primary approach**: `schroot` + `jailguard.py` — chroot-based isolation under `/srv/chroot` with dedicated `jailuser` accounts
- **Process control**: signal-based (`KILL`, `STOP`, `CONT`) via jailguard
- **Earlier approach**: **Systrace** (ptrace-based syscall interception) — used in the 2010 Planet Wars competition
  - Problem: severe performance penalty for Java due to syscall interception overhead ("every syscall intercepted requires alternation between Systrace and the untrusted program")
  - Java bots struggled to stay within per-move time limits
- **Evaluated but not deployed**: **SMACK** (Simplified Mandatory Access Control Kernel) — kernel-side decisions, no performance penalty, but required custom kernel compilation
- **Fallback**: "House" class — unsandboxed subprocess execution for development

### Supported Languages (26+)

**Compiled**: Ada, C, C++, C++11, D, Go, Haskell, Java, Lisp, OCaml, Pascal, Scala
**Interpreted**: Clojure, CoffeeScript, Dart, Erlang, Groovy, JavaScript, Lua, Perl, PHP, Python, Python3, PyPy, Racket, Ruby, Scheme, Tcl
**Managed**: C#, VB.NET

Auto-detection by file extension. Memory limit default: 1500MB. Java heap configured via `-Xmx`.

### Submission Pipeline

Status codes tracked the entire lifecycle:
- 10: record created
- 15: archive placed in temp directory
- 20: ready for compilation
- 24: compiling
- 27: compiled, awaiting tests
- 40: **active** — compiled and passed test cases
- 30/50/60/70: various errors (upload, unzip, file problem, compilation)
- 80: compiled but failed test cases
- 90-99: suspended
- 100: retired

### Known Infrastructure Issues

- Disk space nearly exhausted during the Winter 2010 contest (monitored afterward)
- Workers did not gzip results (bandwidth inefficiency)
- Sandbox lacked proper timestamp logging
- Planned Django migration never completed
- No proper licensing across the codebase
- The project became unmaintainable after the organizers graduated/moved on
- Deployment documentation was sparse — by 2019, it was unclear if the project could still be deployed

### Health Monitoring

Automated scripts checked: account creation, code submission, leaderboard updates, entry suspension, disk space usage.

---

## 7. Game Balance and Dominant Strategies

### Design Principles

The game was designed around explicit criteria:
1. **Easy** — low barrier to entry; 5 minutes to submit a starter bot
2. **Familiar** — ant colony concept universally understood
3. **Interesting** — appeals to programmers at first glance
4. **Technically feasible** — turn-based, bounded computation
5. **Non-trivial** — no discoverable "perfect solution"
6. **Fun to watch** — ant swarms make compelling visualizations

### Strategic Layers

**Exploration**: covering the most map area directly correlated with food income. BFS-based territory partitioning was the standard approach.

**Food Gathering**: BFS from food squares to find closest ant was the baseline. Advanced bots used simultaneous multi-source BFS across all food to optimally assign ants.

**Combat**: the focus algorithm rewarded tight formations. Key tactics:
- **2v1 superiority**: two ants adjacent to one enemy kill it with zero losses
- **Defensive lines**: concentrated forces could hold chokepoints (the "Sparta" strategy)
- **Surrounding**: spreading ants around an enemy force could create local numerical advantages everywhere
- **Third-party exploitation**: in multi-player games, letting opponents fight and cleaning up survivors

**Hill Offense/Defense**: razing hills was the primary scoring mechanism (+2 points). This created tension between:
- Defending your own hills (keeping ants near home)
- Sending expeditionary forces to destroy enemy hills
- The optimal balance shifted through the competition as stronger bots emerged

**Pathfinding**: BFS was the standard. A* was recommended for known-target navigation. Multi-source BFS from all ant positions partitioned the map into influence zones.

### Balance Observations

- **Symmetric maps eliminated positional advantage** — pure strategy and code quality determined outcomes
- **Fog of war prevented perfect-information exploitation** — bots needed exploration and scouting
- **The focus combat system rewarded formation play** — raw ant count wasn't enough; positioning mattered
- **Food competition (contested food destroyed)** prevented simple rushing strategies
- **Multi-player games (2-10 players)** prevented degenerate 1v1 metagames from dominating
- **Cutoff rules** prevented boring games where dominant bots couldn't finish (stalemate prevention)
- **The 85% dominance cutoff** acknowledged that some bots were smart enough to win but not smart enough to execute the final hill-razing — this was a practical concession

### Known Balance Issues

- The competition was heavily explored — by the end, top bots converged on similar strategies
- Java bots had a **systematic disadvantage** due to Systrace sandbox overhead eating into their turn time budget
- Language performance disparities existed: C++ bots could compute much more within the 1000ms turn limit than Python bots
- Map generation symmetry was essential but also predictable — top bots could infer enemy positions from their own

---

## 8. Successor Platforms

### Halite (Two Sigma) — 2016-2019

**Created by**: Ben Spector and Michael Truell during their **summer 2016 internship** at Two Sigma. Additional contributors from Two Sigma and Cornell Tech.

**Three seasons:**
- **Halite I (2016)**: grid-based territory control. Players moved pieces up/down/left/right on a rectangular grid. Website: `2016.halite.io`
- **Halite II (2017)**: continuous 2D space with ships mining planets and building fleets. "Bots battle for control of a virtual continuous universe." Created by David Li, Jaques Clapauch, Harikrishna Menon, Julia Kastner.
- **Halite III (2018)**: resource management — bots navigate the sea collecting halite on a square grid. 9,528 commits in the repo.

**Scale**: "developing a flourishing community of bot builders from around the globe, representing 35+ universities and 20+ organizations"

**Key improvements over AI Challenge:**
- Professional backing (Two Sigma funded and hosted)
- New game each season to prevent stale metagames
- Modern web infrastructure
- WebAssembly-based visualizer (Halite III was 81.2% WebAssembly)

**Demise**: halite.io now redirects to twosigma.com. The competition was discontinued after Halite III (2018-2019). Forums also shut down.

### Battlecode (MIT) — 2001-present

**Longest-running** AI programming competition. Organized by MIT students, running annually since 2001.

**Key characteristics:**
- New game theme every year (e.g., "Chromatic Conflict" 2025, "Breadwars" 2024, "Tempest" 2023)
- Java-based game engine
- **Bytecode counting** for fairness — measures computation in bytecodes rather than wall-clock time (eliminates language performance disparity)
- Code instrumentation for isolated, deterministic execution
- Emphasizes "artificial intelligence, pathfinding, distributed algorithms, and communications"
- Tournament finals recorded and published on YouTube
- Post-mortems from top teams published

**2025 winner**: "Just Woke Up" (Tim Gubskiy, Andy Nguyen) — "Chromatic Conflict" (robot bunnies paint 70% of the map)

**Key difference from Ants**: Battlecode uses bytecode limits instead of wall-clock time, making it language-agnostic in terms of performance. It also changes the game every year, preventing long-term metagame stagnation.

### Lux AI Challenge (Kaggle) — 2021-present

**Season 1 (2021):**
- 1v1 resource management with day/night cycles
- Cities that fail to produce enough light are consumed by darkness
- 22,000+ submissions from 1,100+ teams
- TrueSkill ranking (same as Ants)
- Multiple languages supported (Python, JS, Rust, C++, Java, TypeScript, Kotlin)
- $10,000 prize pool

**Season 2 (2023, NeurIPS track):**
- Asymmetric maps
- Larger scale gameplay
- **JAX-based GPU/TPU acceleration** for simulation
- Episode dataset from hundreds of human-written agents for training
- Academic focus through NeurIPS competition track

**Key improvements**: GPU-accelerated environments enable ML-based agents to train at scale. Hosted on Kaggle's infrastructure (no custom deployment needed). Docker-based consistent execution.

### Kaggle Simulation Environments — 2019-present

Kaggle built a general-purpose `kaggle-environments` library for hosting AI competitions:
- Configurable game lifecycles
- Multi-agent orchestration
- Episode serialization and replay
- HTTP server interface for distributed execution
- OpenAI Gym-style training interface
- Docker support for consistency

This standardized the infrastructure that Ants, Halite, and others each built from scratch.

### Other Notable Successors

- **CodinGame** — persistent platform with multiple AI games, monthly competitions
- **Terminal.live (Correlation One)** — tower-defense-style AI competition
- **AIIDE StarCraft AI Competition** — academic competition using StarCraft: Brood War
- **microRTS** — academic RTS AI competition framework

---

## 9. What Went Wrong / Lessons Learned

### Infrastructure
- The platform was built by university volunteers on a shoestring budget
- No professional DevOps — deployment was manual shell scripts
- Worker scaling was "add more Ubuntu boxes" with no auto-scaling
- MySQL as the sole datastore with no documented backup strategy
- The project died when the original organizers (University of Waterloo CS Club) graduated

### Sandboxing
- Systrace caused **measurable unfairness** for Java bots
- The ideal solution (SMACK) was too complex to deploy on standard Ubuntu
- The chroot/jailguard approach was functional but fragile

### Game Design Tension
- Cutoff rules were necessary but imperfect — some genuinely close games were cut short
- The focus combat algorithm was elegant but led to convergent strategies at the top level
- 10-player games on large maps could be chaotic and hard to reason about

### Community
- The competition generated enormous enthusiasm (7,900 submissions)
- But sustaining it required ongoing volunteer effort that couldn't survive beyond the academic cycle
- Google's sponsorship provided visibility but not long-term infrastructure commitment

### Legacy
The Ants AI Challenge demonstrated the viability and appeal of multi-player AI programming competitions. Its open-source codebase directly influenced Halite's architecture. The TrueSkill matchmaking, worker-based game execution, and JSON replay format became standard patterns. The project's ultimate failure was **organizational sustainability**, not technical quality.
