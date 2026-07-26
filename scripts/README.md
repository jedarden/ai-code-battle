# Scripts

This directory contains utility scripts for the AI Code Battle project.

## Test Replay Management

### Problem

The web viewer at `/watch/replay` loads replays from the R2 bucket (`acb-data`) via the Pages Function at `web/functions/r2/[[path]].ts`. The `matches/index.json` file lists test matches, but the corresponding replay files were never uploaded to R2, causing 404 errors.

### Solution

Two scripts are provided to generate and upload test replays:

#### 1. `generate-test-replays.sh`

Generates all test replays that match the entries in `web/public/data/matches/index.json`:

```bash
bash scripts/generate-test-replays.sh
```

This creates `test-replays/*.json.gz` files with proper match IDs.

#### 2. `upload-test-replays.sh`

Uploads the generated replays to the R2 bucket:

```bash
# Install wrangler first (one-time)
npm install -g wrangler
wrangler login

# Upload the replays
bash scripts/upload-test-replays.sh
```

### Verification

After uploading, verify the replays are accessible:

```bash
curl -I https://ai-code-battle.pages.dev/r2/replays/m_test_upset_v1.json.gz
```

Or test directly in the viewer:
https://ai-code-battle.pages.dev/#/watch/replay?url=/r2/replays/m_test_upset_v1.json.gz

### R2 Bucket Configuration

- **Bucket Name**: `acb-data`
- **Binding**: `ACB_BUCKET` (configured in `web/wrangler.toml`)
- **Served via**: Pages Function at `/r2/*` (`web/functions/r2/[[path]].ts`) — no separate custom domain
- **Public URL**: `https://ai-code-battle.pages.dev/r2/`

### Expected R2 Layout

```
acb-data/
├── replays/           # Match replay JSON files (gzipped)
│   ├── m_test_upset_v1.json.gz
│   ├── m_test_6p_v1.json.gz
│   └── ...
├── matches/           # Per-match metadata
├── thumbnails/        # Match thumbnails
├── cards/            # Bot profile cards
└── evolution/        # Evolution live feed
```

### Error Handling

The replay viewer (`web/src/pages/replay.ts`) already has proper error handling:
- Checks `response.ok` before parsing JSON
- Displays user-visible error messages via `showLoadError()`

No additional error handling code is needed.
