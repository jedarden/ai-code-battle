# Bot Language and Purpose Analysis

## All 21 Bots Annotated

| Bot Name | Language | Purpose |
|----------|----------|---------|
| **assassin** | Rust | Decapitation archetype - all units rush the enemy core directly, ignoring enemy units and economy |
| **coordinator** | TypeScript | Dynamic role allocation - assigns each bot a role (Attacker/Harvester/Defender/Scout) each turn, rebalancing based on game state |
| **defender** | C# | Core-hugging perimeter defense - bots patrol evenly-spaced slots around nearest active core and intercept enemies entering perimeter |
| **economist** | Python | Energy starvation through node contesting - deliberately contests energy nodes enemies are approaching (destroying energy) while harvesting uncontested nodes |
| **farmer** | Go | Energy collection and farming strategy |
| **gatherer** | Go | Maximizes energy collection while avoiding combat |
| **guardian** | PHP | Defensive core protection with cautious expansion - maintains perimeter bots within 5 tiles of each owned core |
| **hunter** | Java | Targets isolated enemies for efficient kills - sends pairs of bots to intercept isolated enemies (≥4 tiles from friendly support), defaults to gathering when no isolated targets |
| **kamikaze** | JavaScript | Aggressive attacker that finds and attacks nearest enemy |
| **leader-targeter** | Java | In N>2 games, always directs all units toward current score leader (kingmaker dynamic); in 2-player games, falls back to straight aggressor |
| **nomad** | Python | Constant relocation archetype - picks target region every ~20 turns, migrates all units toward it, briefly engages on arrival, then picks new region |
| **opportunist** | Go | Opportunistic strategy taking advantage of tactical opportunities |
| **pacifist** | JavaScript | Pure evasion, never attacks - moves to maximize distance from nearest enemy, retreats toward own core if cornered |
| **phalanx** | Rust | Tight formation archetype - all units move as coordinated group, maximizing local firepower at cost of map coverage |
| **raider** | Java | Hit-and-run harassment - scouts lone enemy bots, attacks from flank for 1-2 turns, then retreats; never attacks groups of ≥3 enemies |
| **random** | Python | Makes random valid moves - reference implementation demonstrating HTTP protocol |
| **rusher** | Rust | Rushes enemy cores aggressively using BFS pathfinding, ignoring energy and enemy bots unless they block path |
| **scout** | Python | Exploration-maximizing - maintains last-seen tick counter per cell, moves toward stalest unobserved cells, multiple bots assigned different map zones |
| **siege** | Go | Blocks enemy core spawning by surrounding cores |
| **swarm** | TypeScript | Formation-based combat strategy |
| **zone-driver** | Rust | Weaponizes the shrinking zone - positions units to block enemy escape routes inward, herds enemies toward zone edge for passive eliminations |

## Language Distribution

- **Rust**: 4 bots (assassin, phalanx, rusher, zone-driver)
- **Java**: 3 bots (hunter, leader-targeter, raider)
- **Python**: 4 bots (economist, nomad, random, scout)
- **Go**: 4 bots (farmer, gatherer, opportunist, siege)
- **TypeScript**: 2 bots (coordinator, swarm)
- **JavaScript**: 2 bots (kamikaze, pacifist)
- **C#**: 1 bot (defender)
- **PHP**: 1 bot (guardian)

**Total: 21 bots**
