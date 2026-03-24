# LLM-Driven Bot Evolution — Research

## Overview

Survey of real systems where LLMs iteratively generate, evaluate, and evolve
code. Focus on approaches applicable to evolving game bot strategies.

---

## 1. FunSearch (DeepMind, 2023)

**Paper:** [Nature](https://www.nature.com/articles/s41586-023-06924-6)
**Code:** [github.com/google-deepmind/funsearch](https://github.com/google-deepmind/funsearch)

The cleanest template for "evolve code via LLM."

**Evolutionary loop:**

1. User provides an `evaluate` function (fitness scorer) and a trivial seed
   implementation of the function to evolve.
2. **Programs Database** stores all scored programs using an **island model** —
   multiple independent populations evolving in parallel to maintain diversity.
   Within each island, programs are clustered by score signature. Sampling
   favors high-scoring clusters; within a cluster, favors shorter programs.
3. **Samplers** pull 2–3 high-scoring programs from the database, combine into
   a prompt, and query a pre-trained LLM to generate a new candidate function.
4. **Evaluators** execute the candidate against test inputs and score it. If
   correct, it enters the database.
5. Repeat. System runs 15 samplers and 150 CPU evaluators in parallel.

**Results:** Discovered new constructions for the cap set problem (open math)
and beat human heuristics for online bin-packing.

**Relevance:** The evaluator maps directly to "run the bot in the game engine,
score the match." Island model prevents convergence to one strategy.

---

## 2. AlphaEvolve (DeepMind, 2025) — FunSearch's Successor

**Paper:** [arxiv.org/abs/2506.13131](https://arxiv.org/abs/2506.13131)

Evolves entire codebases (not just single functions). Uses an **ensemble** of
Gemini Flash (exploration/breadth) and Gemini Pro (exploitation/depth).

**Results:** Recovered 0.7% of Google's worldwide compute by optimizing data
center scheduling. Found new matrix multiplication algorithms beating 1969
Strassen result.

**Open-source implementations:**

| Project | URL | Notes |
|---------|-----|-------|
| **OpenEvolve** | [github.com/algorithmicsuperintelligence/openevolve](https://github.com/algorithmicsuperintelligence/openevolve) | `pip install openevolve`. Full pipeline: prompt sampler, LLM ensemble, evaluator pool, program database with MAP-Elites + island model |
| **ShinkaEvolve** (Sakana AI) | [github.com/SakanaAI/ShinkaEvolve](https://github.com/SakanaAI/ShinkaEvolve) | Apache 2.0. Adds novelty rejection-sampling, bandit-based LLM selection. Won ICFP 2025 Programming Contest. Found SOTA for circle packing in ~150 evaluations |
| **OpenAlpha_Evolve** | [github.com/shyamsaktawat/OpenAlpha_Evolve](https://github.com/shyamsaktawat/OpenAlpha_Evolve) | Community implementation |

---

## 3. ELM / OpenELM (Lehman et al., OpenAI, 2022)

**Paper:** [arxiv.org/pdf/2206.08896](https://arxiv.org/pdf/2206.08896)
**Code:** [github.com/CarperAI/OpenELM](https://github.com/CarperAI/OpenELM)

**Key insight:** Treats the LLM as a **diff/mutation operator**, not a
from-scratch generator. Uses commit-message-style prompts so the model
understands what kind of change is being requested.

**Architecture:**

1. **LLM mutation operator** — generates code diffs, not complete programs
2. **MAP-Elites outer loop** — maintains a grid of niches spanning user-defined
   behavior dimensions. Each niche holds the best-performing individual. New
   candidates replace niche inhabitants only if they score higher.
3. **LLM fine-tuning on successful mutations** — model updated based on which
   mutations worked, closing the loop.

**Results:** Generated hundreds of thousands of functional Python programs
producing working robots in the Sodarace domain — a domain the LLM had never
seen in training.

**Relevance:** MAP-Elites ensures diversity of strategies (not just one dominant
approach). The diff-based mutation is practical for evolving bot code — smaller
changes, faster iteration.

---

## 4. AlphaCode / AlphaCode 2 (DeepMind)

**AlphaCode:** [science.org/doi/10.1126/science.abq1158](https://www.science.org/doi/10.1126/science.abq1158)
**AlphaCode 2:** [Technical Report](https://storage.googleapis.com/deepmind-media/AlphaCode2/AlphaCode2_Tech_Report.pdf)

Not evolutionary — purely generative with massive oversampling and filtering:

1. Generate ~1 million candidate solutions per problem
2. Filter by executing against example test cases (eliminates ~99%)
3. Cluster remaining by behavioral similarity (run on synthetic inputs, group
   programs producing identical outputs)
4. Select one representative per cluster, rank by scoring model, submit top 10

**AlphaCode 2:** 85th percentile on Codeforces (vs ~50th for v1). Uses Gemini
Pro with dedicated scoring/reranking model.

**Relevance:** The "generate many, filter by execution, cluster by behavior"
pattern is useful for the initial seeding phase — generate many candidate bots,
test them all, keep the diverse winners.

---

## 5. Voyager (NVIDIA / MineDojo, 2023)

**Paper:** [arxiv.org/abs/2305.16291](https://arxiv.org/abs/2305.16291)
**Code:** [github.com/MineDojo/Voyager](https://github.com/MineDojo/Voyager)

LLM agent that writes its own code in Minecraft.

**Three components:**

1. **Automatic Curriculum** — generates increasingly difficult objectives
2. **Skill Library** — persistent, growing collection of verified code snippets.
   New tasks retrieve relevant skills by embedding similarity. Successful new
   skills get added.
3. **Iterative Prompting with Self-Verification:**
   - GPT-4 generates code for a task
   - Code executes in environment
   - Environment feedback + errors collected
   - Self-verification module (also GPT-4) checks completion
   - If not complete, feedback fed back and code refined
   - Only verified-successful skills enter the library

**Results:** 3.3x more unique items, 15.3x faster tech-tree progression vs
prior SOTA. Skills transfer to new worlds.

**Relevance:** The skill library pattern — decomposing strategy into reusable
verified components — could apply to bot strategies (e.g., verified pathfinding,
verified formation combat, verified scouting behaviors composed into full bots).

---

## 6. Game-Bot-Specific Systems

### LLM-PSRO (IJCAI 2025) — Most Directly Relevant

**Paper:** [ijcai.org/proceedings/2025/1249](https://www.ijcai.org/proceedings/2025/1249)

The most directly applicable system. Uses Policy Space Response Oracle:

1. Start with a population of bots (hand-written or LLM-generated)
2. Run round-robin tournaments → build payoff matrix
3. Compute **Nash equilibrium** mixture over the current population
4. Prompt the LLM to generate a new bot that **beats the Nash mixture**,
   providing the losing bot's code and match results as context
5. Add new bot to population
6. Repeat

**Why Nash matters:** You can only add bots that improve the population's
game-theoretic profile. This is mathematically principled regression prevention
— the new bot must beat the optimal mixed strategy, not just one opponent.

### CATArena (2025)

**Paper:** [arxiv.org/abs/2510.26852](https://arxiv.org/abs/2510.26852)
**Code:** [github.com/AGI-Eval-Official/CATArena](https://github.com/AGI-Eval-Official/CATArena)

LLM code agents play Gomoku, Texas Hold'em, Chess, Bridge. Agents refine
through **self-reflection** (analyzing own losses) and **peer-learning**
(reading opponent code).

**Key finding:** Evolutionary potential doesn't correlate with initial
proficiency — some weaker initial agents evolve faster.

### AlphaCodium (CodiumAI / Qodo)

**Paper:** [arxiv.org/abs/2401.08500](https://arxiv.org/abs/2401.08500)
**Code:** [github.com/Codium-ai/AlphaCodium](https://github.com/Codium-ai/AlphaCodium)

Two-phase iterative flow: pre-processing (self-reflection, test reasoning)
then generate-execute-refine against tests. Boosted GPT-4 from 19% to 44%
on CodeContests with only 15–20 LLM calls per solution.

### STOP — Self-Taught Optimizer (Microsoft Research, COLM 2024)

**Paper:** [arxiv.org/abs/2310.02304](https://arxiv.org/abs/2310.02304)
**Code:** [github.com/microsoft/stop](https://github.com/microsoft/stop)

A seed "improver" program that calls GPT-4 to improve code, then is run on
itself to improve the improver. The self-improved improver discovers strategies
like beam search, genetic algorithms, and simulated annealing — on its own.

---

## 7. Evaluation / Selection Mechanisms

| System | Selection Mechanism |
|--------|---------------------|
| **FunSearch / AlphaEvolve** | Island model with score-based cluster sampling. Higher scores sampled more. Islands prevent premature convergence. |
| **OpenELM** | MAP-Elites quality-diversity grid. New candidate replaces niche inhabitant only if it scores higher. Maintains diversity across behavior dimensions. |
| **AlphaCode** | Generate millions, filter by execution, cluster by behavior, rank by scoring model. Pure oversampling, no evolution. |
| **LLM-PSRO** | Nash equilibrium over population. New bots must beat the Nash mixture. Theoretically grounded regression prevention. |
| **CATArena** | Dual-metric: static proficiency vs evolutionary potential. Global win rate preferred over Elo for stability. |
| **ShinkaEvolve** | Parent sampling balancing exploration/exploitation + novelty rejection-sampling (rejects candidates too similar to existing population). |

---

## 8. Code Sandboxing for LLM-Generated Code

| Solution | Isolation Level | Overhead | Used By |
|----------|----------------|----------|---------|
| **Firecracker MicroVMs** | Strongest (own kernel) | <200ms boot, <5 MiB/VM | AWS Lambda, E2B ([e2b.dev](https://e2b.dev/)), ~50% Fortune 500 |
| **gVisor** | Strong (userspace kernel) | Low | GKE Sandbox, [k8s agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) |
| **nsjail** | Moderate (namespaces + seccomp) | Minimal | FunSearch evaluators (150 nodes) |
| **WASM** | Moderate (no fs/network) | Near-native | Constrained execution environments |

**Recommendation for bot evolution:** nsjail for high-throughput evaluation
(you control the game engine); Firecracker/E2B if executing fully arbitrary
LLM code with network/filesystem access.

**Reference:** [github.com/restyler/awesome-sandbox](https://github.com/restyler/awesome-sandbox)

---

## 9. Recommended Architecture for AI Code Battle

Based on the systems above, the **FunSearch/AlphaEvolve island model +
LLM-PSRO game-theoretic selection** combination is the best fit:

```
┌─────────────────────────────────────────────────────┐
│                   Programs Database                  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐  │
│  │ Island 1 │ │ Island 2 │ │ Island 3 │ │ Island 4 │  │
│  │ (Python) │ │ (Go)     │ │ (Rust)   │ │ (mixed)  │  │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘  │
└───────────────────────┬─────────────────────────────┘
                        │
          sample 2-3 parents + match replays
                        │
              ┌─────────▼──────────┐
              │   Prompt Builder    │
              │  • Parent code      │
              │  • Recent losses    │
              │  • Replay analysis  │
              │  • "Beat this meta" │
              └─────────┬──────────┘
                        │
              ┌─────────▼──────────┐
              │   LLM Ensemble      │
              │  • Fast model       │
              │    (exploration)    │
              │  • Strong model     │
              │    (exploitation)   │
              └─────────┬──────────┘
                        │
              ┌─────────▼──────────┐
              │   Build & Validate  │
              │  • Compile/lint     │
              │  • Schema test      │
              │  • Sandbox execute  │
              └─────────┬──────────┘
                        │
              ┌─────────▼──────────┐
              │   Tournament Gate   │
              │  • Play vs current  │
              │    population       │
              │  • Must beat Nash   │
              │    mixture (PSRO)   │
              └─────────┬──────────┘
                        │
               promote if better
                        │
              ┌─────────▼──────────┐
              │   Deploy as         │
              │   Container         │
              │  • Build image      │
              │  • Register bot     │
              │  • Enter ladder     │
              └────────────────────┘
```

### References

- [FunSearch — GitHub](https://github.com/google-deepmind/funsearch)
- [OpenEvolve — GitHub](https://github.com/algorithmicsuperintelligence/openevolve)
- [ShinkaEvolve — GitHub](https://github.com/SakanaAI/ShinkaEvolve)
- [OpenELM — GitHub](https://github.com/CarperAI/OpenELM)
- [AlphaCode Dataset — GitHub](https://github.com/google-deepmind/code_contests)
- [Voyager — GitHub](https://github.com/MineDojo/Voyager)
- [LLM-PSRO — IJCAI 2025](https://www.ijcai.org/proceedings/2025/1249)
- [CATArena — GitHub](https://github.com/AGI-Eval-Official/CATArena)
- [AlphaCodium — GitHub](https://github.com/Codium-ai/AlphaCodium)
- [STOP — GitHub](https://github.com/microsoft/stop)
- [Awesome LLM Game Agent Papers](https://github.com/git-disl/awesome-LLM-game-agent-papers)
- [Awesome Self-Evolving Agents](https://github.com/EvoAgentX/Awesome-Self-Evolving-Agents)
- [nsjail — GitHub](https://github.com/google/nsjail)
- [E2B Sandbox](https://e2b.dev/)
- [Awesome Sandbox](https://github.com/restyler/awesome-sandbox)
