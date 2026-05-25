# Starter Kits for AI Code Battle

This document describes the starter kit templates created for AI Code Battle participants.

## Overview

All 8 starter kits are complete and ready for deployment to GitHub repositories. Each starter kit provides:
- A working HTTP server with HMAC verification
- Type definitions for game state and move schemas
- Grid utility functions (toroidal distance, BFS, neighbor enumeration)
- A stub strategy function (holds all bots in place)
- A Dockerfile for container deployment
- A README with quickstart instructions

## Starter Kits

| Kit | Language | Location | Notes |
|-----|----------|----------|-------|
| `acb-starter-python` | Python 3.13 | `~/scratch/starter-kits/acb-starter-python/` | Flask-based, stdlib only |
| `acb-starter-go` | Go 1.23 | `~/scratch/starter-kits/acb-starter-go/` | stdlib `net/http` |
| `acb-starter-rust` | Rust 1.83 | `~/scratch/starter-kits/acb-starter-rust/` | axum + serde |
| `acb-starter-php` | PHP 8.4 | `~/scratch/starter-kits/acb-starter-php/` | Built-in stream socket server |
| `acb-starter-typescript` | TypeScript | `~/scratch/starter-kits/acb-starter-typescript/` | Node.js built-in http, no external deps |
| `acb-starter-javascript` | JavaScript | `~/scratch/starter-kits/acb-starter-javascript/` | Node.js built-in http |
| `acb-starter-java` | Java 21 | `~/scratch/starter-kits/acb-starter-java/` | Javalin framework |
| `acb-starter-csharp` | C# | `~/scratch/starter-kits/acb-starter-csharp/` | ASP.NET Core minimal API |

## Deployment to GitHub

To deploy these starter kits to GitHub repositories:

```bash
# For each starter kit
cd ~/scratch/starter-kits/acb-starter-XXX
git init
git add .
git commit -m "Initial commit: AI Code Battle starter kit for XXX"

# Create GitHub repo and push
# (Requires gh CLI or manual GitHub creation)
gh repo create acb-starter-XXX --public --source . --push
```

## File Structure per Kit

Each starter kit follows this pattern:

```
acb-starter-XXX/
├── README.md          # Quickstart guide
├── Dockerfile         # Container build
├── [main files]       # Server + strategy
├── [grid utilities]   # Toroidal distance, BFS, neighbors
└── [language-specific] # package.json, pom.xml, Cargo.toml, etc.
```

## Verification

All starter kits include:
- ✅ HMAC signature verification (request and response)
- ✅ `/health` endpoint for health checks
- ✅ `/turn` endpoint for game engine requests
- ✅ Game state parsing and type definitions
- ✅ Grid utilities (distance2, distance, neighbors, BFS)
- ✅ Stub strategy function
- ✅ Dockerfile for containerized deployment
- ✅ README with documentation

## Next Steps

1. Create GitHub repositories for each starter kit
2. Add links to platform documentation at `/docs/protocol.md`
3. Update registration page to suggest starter kits for new users
