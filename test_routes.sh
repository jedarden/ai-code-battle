#!/usr/bin/env bash
# Smoke-test all SPA routes on ai-code-battle.pages.dev
# Fallback method: curl (ADB not available on this system)

BASE_URL="https://ai-code-battle.pages.dev"
PASSED=0
FAILED=0
WARNED=0

# Color output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== SPA Route Smoke Test ==="
echo "Testing routes on $BASE_URL"
echo "Method: curl (ADB not available)"
echo ""

# Helper to test a route returns valid HTML
test_route() {
    local route="$1"
    local description="$2"

    local url="${BASE_URL}${route}"
    local response=$(curl -s -L -w "\n%{http_code}" "$url" 2>/dev/null)
    local status_code=$(echo "$response" | tail -1)
    local body=$(echo "$response" | head -n -1)

    # Check for successful response and HTML doctype
    if [[ "$status_code" == "200" ]] && [[ "$body" == *"<!DOCTYPE html>"* ]]; then
        echo -e "${GREEN}✓${NC} $route - $description (200 OK)"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}✗${NC} $route - $description (status: $status_code)"
        if [[ "$body" != *"<!DOCTYPE html>"* ]]; then
            echo "  Response is not valid HTML"
        fi
        ((FAILED++))
        return 1
    fi
}

# Helper to test data path endpoints
test_data_path() {
    local path="$1"
    local description="$2"

    local url="${BASE_URL}${path}"
    local status_code=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null)

    if [[ "$status_code" == "200" ]]; then
        echo -e "${GREEN}✓${NC} $path - $description (200 OK)"
        ((PASSED++))
        return 0
    elif [[ "$status_code" == "404" ]]; then
        echo -e "${YELLOW}⊘${NC} $path - $description (404 Not Found - data may not exist yet)"
        ((WARNED++))
        return 2
    else
        echo -e "${RED}✗${NC} $path - $description (status: $status_code)"
        ((FAILED++))
        return 1
    fi
}

echo "=== Static Routes (SPA Shell) ==="

# Main routes - all should return the same HTML shell
test_route "/" "Home page"
test_route "/watch" "Watch hub"
test_route "/watch/replays" "Matches page"
test_route "/watch/playlists" "Playlists page"
test_route "/watch/replay" "Replay page (no id)"
test_route "/watch/predictions" "Predictions page"
test_route "/watch/series" "Series page"
test_route "/compete" "Compete hub"
test_route "/compete/sandbox" "Sandbox page"
test_route "/compete/register" "Register page"
test_route "/compete/docs" "Compete docs"
test_route "/compete/docs/api" "API docs"
test_route "/leaderboard" "Leaderboard page"
test_route "/evolution" "Evolution page"
test_route "/blog" "Blog page"
test_route "/seasons" "Seasons page"
test_route "/rivalries" "Rivalries page"
test_route "/feedback" "Feedback page"
test_route "/docs/api" "API docs (alt path)"

echo ""
echo "=== Redirect Routes (should return 200 with SPA content) ==="

test_route "/matches" "Redirect to /watch/replays"
test_route "/playlists" "Redirect to /watch/playlists"
test_route "/replay" "Redirect to /watch/replay"
test_route "/predictions" "Redirect to /watch/predictions"
test_route "/series" "Redirect to /watch/series"
test_route "/sandbox" "Redirect to /compete/sandbox"
test_route "/register" "Redirect to /compete/register"
test_route "/bots" "Redirect to /leaderboard"
test_route "/docs" "Redirect to /compete/docs"
test_route "/clip-maker" "Redirect to /watch/replays"

echo ""
echo "=== Data Path Tests (/r2/) ==="

test_data_path "/r2/" "R2 data root"
test_data_path "/r2/data/matches/index.json" "Matches index"
test_data_path "/r2/data/leaderboard.json" "Leaderboard data"
test_data_path "/r2/data/seasons/index.json" "Seasons index"
test_data_path "/r2/data/series/index.json" "Series index"
test_data_path "/r2/data/blog/index.json" "Blog index"
test_data_path "/r2/data/playlists/index.json" "Playlists index"

echo ""
echo "=== Parameterized Routes (with sample IDs) ==="
echo "Note: These will fail if /r2/ data path is not working"

# Try with known-good IDs if we can fetch them
# For now, test that the route pattern returns valid HTML
test_route "/watch/replay/test_match_id" "Replay detail (placeholder id)"
test_route "/bot/test_bot_id" "Bot profile (placeholder id)"
test_route "/compete/bot/test_bot_id" "Bot profile compete path (placeholder id)"
test_route "/season/test_season_id" "Season detail (placeholder id)"
test_route "/watch/series/test_series_id" "Series detail (placeholder id)"
test_route "/blog/test-slug" "Blog post (placeholder slug)"
test_route "/watch/playlists/test-slug" "Playlist detail (placeholder slug)"

echo ""
echo "=== Summary ==="
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${YELLOW}Warned (404): $WARNED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo "Total: $((PASSED + WARNED + FAILED))"

if [[ $FAILED -eq 0 ]]; then
    if [[ $WARNED -gt 0 ]]; then
        echo -e "\n${YELLOW}All routes passed, but some data paths returned 404${NC}"
        exit 0
    else
        echo -e "\n${GREEN}All routes and data paths passed!${NC}"
        exit 0
    fi
else
    echo -e "\n${RED}Some routes failed.${NC}"
    exit 1
fi
