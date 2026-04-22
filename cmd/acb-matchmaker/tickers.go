package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aicodebattle/acb/metrics"
)

const valkeyJobQueue = "acb:jobs:pending"

// candidateBot holds bot data used during the §6.1 matchmaking algorithm.
type candidateBot struct {
	ID           string
	Endpoint     string
	Secret       string
	Mu           float64
	Phi          float64
	LastMatchAt  time.Time
	Games24h     int
	LastPairedAt time.Time // zero = never paired with the seed bot
}

func (m *Matchmaker) StartTickers(ctx context.Context) {
	go m.runTicker(ctx, "matchmaker", time.Duration(m.cfg.MatchmakerSecs)*time.Second, m.tickMatchmaker)
	go m.runTicker(ctx, "health-checker", time.Duration(m.cfg.HealthCheckSecs)*time.Second, m.tickHealthChecker)
	go m.runTicker(ctx, "stale-reaper", time.Duration(m.cfg.ReaperSecs)*time.Second, m.tickStaleReaper)
	go m.runTicker(ctx, "series-scheduler", time.Duration(m.cfg.SeriesSchedSecs)*time.Second, m.tickSeriesScheduler)
	go m.runTicker(ctx, "season-reset", time.Duration(m.cfg.SeasonResetSecs)*time.Second, m.tickSeasonReset)
}

func (m *Matchmaker) runTicker(ctx context.Context, name string, interval time.Duration, fn func(context.Context)) {
	log.Printf("starting ticker: %s (every %s)", name, interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopping ticker: %s", name)
			return
		case <-ticker.C:
			fn(ctx)
		}
	}
}

// tickMatchmaker implements the §6.1 pairing algorithm:
//  1. Seed = bot with longest time since last match (tiebreak: lowest bot ID)
//  2. Format = seed's least-played player count among {2, 3, 4, 6}
//  3. Opponents = Pareto skill-proximity (80% within 16 ranks) + oldest last-pairing + fewest 24h games
//  4. Map = least-recently-used active map for the chosen player count
//  5. Enqueue match job with randomised player slot assignment
func (m *Matchmaker) tickMatchmaker(ctx context.Context) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Step 1: load all eligible bots with last-match time and 24h game count.
	candidates, err := m.queryEligibleCandidates(ctx)
	if err != nil {
		log.Printf("matchmaker: query candidates: %v", err)
		return
	}
	if len(candidates) < 2 {
		return
	}

	// Step 2: seed = bot with oldest last-match timestamp (tiebreak: lowest bot ID).
	seed := candidates[0]
	for _, c := range candidates[1:] {
		if c.LastMatchAt.Before(seed.LastMatchAt) ||
			(c.LastMatchAt.Equal(seed.LastMatchAt) && c.ID < seed.ID) {
			seed = c
		}
	}

	// Step 3: format = seed's least-played player count, feasible given active bot count.
	matchSize, err := m.leastPlayedFormat(ctx, seed.ID, len(candidates))
	if err != nil {
		log.Printf("matchmaker: format select: %v", err)
		matchSize = 2
	}

	// Step 4: annotate pool with last-pairing recency relative to seed.
	pairTimes, err := m.queryPairingTimes(ctx, seed.ID)
	if err != nil {
		log.Printf("matchmaker: pairing times: %v", err)
		pairTimes = map[string]time.Time{}
	}
	pool := make([]candidateBot, 0, len(candidates)-1)
	for _, c := range candidates {
		if c.ID == seed.ID {
			continue
		}
		c.LastPairedAt = pairTimes[c.ID]
		pool = append(pool, c)
	}

	// Step 5: select opponents with Pareto + recency + game-balance criteria.
	opponents := selectOpponents(rng, seed.Mu, pool, matchSize-1)
	if len(opponents) < matchSize-1 {
		// Not enough bots for the desired format — fall back to 2-player.
		matchSize = 2
		opponents = selectOpponents(rng, seed.Mu, pool, 1)
		if len(opponents) == 0 {
			return
		}
	}

	// Step 6: LRU map selection for this player count.
	mapID, mapRows, mapCols, mapSeed := m.selectMapLRU(ctx, matchSize, rng)

	// Step 7: create match DB records and enqueue job.
	participants := append([]candidateBot{seed}, opponents...)
	if err := m.createMatch(ctx, rng, participants, mapID, mapRows, mapCols, mapSeed, matchSize); err != nil {
		log.Printf("matchmaker: create match: %v", err)
		return
	}

	// Update map_scores.last_used_at (best-effort, outside the transaction).
	m.db.ExecContext(ctx, `
		INSERT INTO map_scores (map_id, last_used_at, match_count)
		VALUES ($1, NOW(), 1)
		ON CONFLICT (map_id) DO UPDATE
		SET last_used_at = NOW(), match_count = map_scores.match_count + 1
	`, mapID)
}

// queryEligibleCandidates returns active bots not on crash cooldown (§4.5, §6.1),
// annotated with their last-match timestamp and 24-hour game count.
func (m *Matchmaker) queryEligibleCandidates(ctx context.Context) ([]candidateBot, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT
			b.bot_id,
			b.endpoint_url,
			b.shared_secret,
			b.rating_mu,
			b.rating_phi,
			COALESCE(lm.last_match_at, '1970-01-01 00:00:00+00'::timestamptz) AS last_match_at,
			COALESCE(g.games_24h, 0) AS games_24h
		FROM bots b
		LEFT JOIN (
			SELECT mp.bot_id, MAX(m.created_at) AS last_match_at
			FROM match_participants mp
			JOIN matches m ON mp.match_id = m.match_id
			GROUP BY mp.bot_id
		) lm ON lm.bot_id = b.bot_id
		LEFT JOIN (
			SELECT mp.bot_id, COUNT(*)::int AS games_24h
			FROM match_participants mp
			JOIN matches m ON mp.match_id = m.match_id
			WHERE m.created_at >= NOW() - INTERVAL '24 hours'
			GROUP BY mp.bot_id
		) g ON g.bot_id = b.bot_id
		WHERE b.status = 'active'
		  AND (b.cooldown_until IS NULL OR b.cooldown_until < NOW())
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []candidateBot
	for rows.Next() {
		var c candidateBot
		if err := rows.Scan(&c.ID, &c.Endpoint, &c.Secret, &c.Mu, &c.Phi, &c.LastMatchAt, &c.Games24h); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// leastPlayedFormat returns the player count (2/3/4/6) that seedID has participated
// in fewest times. Skips formats that require more bots than numCandidates.
func (m *Matchmaker) leastPlayedFormat(ctx context.Context, seedID string, numCandidates int) (int, error) {
	rows, err := m.db.QueryContext(ctx, `
		WITH seed_sizes AS (
			SELECT COUNT(mp2.bot_id)::int AS player_count
			FROM match_participants mp1
			JOIN matches mx ON mx.match_id = mp1.match_id
			JOIN match_participants mp2 ON mp2.match_id = mx.match_id
			WHERE mp1.bot_id = $1
			GROUP BY mx.match_id
		),
		format_counts AS (
			SELECT player_count, COUNT(*) AS cnt
			FROM seed_sizes
			GROUP BY player_count
		)
		SELECT f.n, COALESCE(fc.cnt, 0) AS cnt
		FROM (VALUES (2), (3), (4), (6)) f(n)
		LEFT JOIN format_counts fc ON fc.player_count = f.n
		ORDER BY cnt ASC, f.n ASC
	`, seedID)
	if err != nil {
		return 2, err
	}
	defer rows.Close()

	for rows.Next() {
		var n, cnt int
		if err := rows.Scan(&n, &cnt); err != nil {
			return 2, err
		}
		if numCandidates >= n {
			return n, nil
		}
	}
	return 2, rows.Err()
}

// queryPairingTimes returns a map of bot_id → most recent time it shared a match
// with seedID. Bots that have never been paired with seedID are absent from the map.
func (m *Matchmaker) queryPairingTimes(ctx context.Context, seedID string) (map[string]time.Time, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT mp2.bot_id, MAX(mx.created_at) AS last_paired_at
		FROM match_participants mp1
		JOIN matches mx ON mx.match_id = mp1.match_id
		JOIN match_participants mp2
		    ON mp2.match_id = mx.match_id AND mp2.bot_id != $1
		WHERE mp1.bot_id = $1
		GROUP BY mp2.bot_id
	`, seedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]time.Time)
	for rows.Next() {
		var botID string
		var t time.Time
		if err := rows.Scan(&botID, &t); err != nil {
			return nil, err
		}
		out[botID] = t
	}
	return out, rows.Err()
}

// selectOpponents picks `count` opponents from pool using §6.1 criteria:
//   - Pareto: 80% chance restrict selection to the 16 rating-closest bots
//   - Secondary: oldest last-pairing with seed (zero = never = most preferred)
//   - Tertiary: fewest 24h games breaks remaining ties
func selectOpponents(rng *rand.Rand, seedMu float64, pool []candidateBot, count int) []candidateBot {
	remaining := make([]candidateBot, len(pool))
	copy(remaining, pool)

	selected := make([]candidateBot, 0, count)
	for i := 0; i < count && len(remaining) > 0; i++ {
		// Sort by rating proximity to seed.
		sort.Slice(remaining, func(a, b int) bool {
			return math.Abs(remaining[a].Mu-seedMu) < math.Abs(remaining[b].Mu-seedMu)
		})

		// Pareto: 80% from the 16 closest, 20% from all.
		eligible := remaining
		if rng.Float64() < 0.80 {
			n := 16
			if n > len(remaining) {
				n = len(remaining)
			}
			eligible = remaining[:n]
		}

		best := bestCandidate(eligible)
		selected = append(selected, best)

		for j, c := range remaining {
			if c.ID == best.ID {
				remaining = append(remaining[:j], remaining[j+1:]...)
				break
			}
		}
	}
	return selected
}

// bestCandidate picks the best opponent from a pool by secondary criteria:
// oldest last-pairing (zero = never = most preferred), then fewest 24h games.
func bestCandidate(pool []candidateBot) candidateBot {
	best := pool[0]
	for _, c := range pool[1:] {
		bz := best.LastPairedAt.IsZero()
		cz := c.LastPairedAt.IsZero()
		switch {
		case cz && !bz:
			best = c
		case !cz && !bz && c.LastPairedAt.Before(best.LastPairedAt):
			best = c
		case bz == cz && c.Games24h < best.Games24h:
			best = c
		}
	}
	return best
}

// selectMapLRU returns the active map for playerCount with the oldest last_used_at.
// Falls back to a random procedural seed if no maps exist for that player count.
func (m *Matchmaker) selectMapLRU(ctx context.Context, playerCount int, rng *rand.Rand) (string, int, int, int64) {
	var mapID string
	var gridH, gridW int
	err := m.db.QueryRowContext(ctx, `
		SELECT mp.map_id, mp.grid_height, mp.grid_width
		FROM maps mp
		LEFT JOIN map_scores ms ON ms.map_id = mp.map_id
		WHERE mp.player_count = $1 AND mp.status = 'active'
		ORDER BY COALESCE(ms.last_used_at, '1970-01-01 00:00:00+00'::timestamptz) ASC
		LIMIT 1
	`, playerCount).Scan(&mapID, &gridH, &gridW)
	if err != nil {
		seed := rng.Int63()
		rows, cols := gridForPlayers(playerCount)
		return fmt.Sprintf("map_%d", seed%100000), rows, cols, seed
	}
	return mapID, gridH, gridW, rng.Int63()
}

// gridForPlayers returns default grid dimensions for a given player count,
// mirroring the formula in engine.ConfigForPlayers (~2000 tiles per player).
func gridForPlayers(n int) (rows, cols int) {
	if n <= 2 {
		return 60, 60
	}
	side := int(math.Sqrt(float64(2000 * n)))
	if side < 40 {
		side = 40
	}
	if side > 200 {
		side = 200
	}
	return side, side
}

// createMatch inserts match, participants, and job rows, then enqueues in Valkey.
func (m *Matchmaker) createMatch(
	ctx context.Context,
	rng *rand.Rand,
	participants []candidateBot,
	mapID string,
	mapRows, mapCols int,
	mapSeed int64,
	playerCount int,
) error {
	matchID, err := generateID("m_", 8)
	if err != nil {
		return err
	}
	jobID, err := generateID("j_", 8)
	if err != nil {
		return err
	}

	// Compute max turns from grid size; floor at 500.
	maxTurns := mapRows * 8
	if maxTurns < 500 {
		maxTurns = 500
	}

	// Randomise player slots.
	slots := rng.Perm(len(participants))

	type botConfig struct {
		BotID    string `json:"bot_id"`
		Endpoint string `json:"endpoint"`
		Secret   string `json:"secret"`
		Slot     int    `json:"slot"`
	}
	botCfgs := make([]botConfig, len(participants))
	for i, p := range participants {
		secret := p.Secret
		if m.cfg.EncryptionKey != "" {
			if dec, decErr := decryptSecret(p.Secret, m.cfg.EncryptionKey); decErr == nil {
				secret = dec
			}
		}
		botCfgs[i] = botConfig{
			BotID:    p.ID,
			Endpoint: p.Endpoint,
			Secret:   secret,
			Slot:     slots[i],
		}
	}

	type jobConfig struct {
		MatchID  string      `json:"match_id"`
		MapSeed  int64       `json:"map_seed"`
		MaxTurns int         `json:"max_turns"`
		Rows     int         `json:"rows"`
		Cols     int         `json:"cols"`
		Bots     []botConfig `json:"bots"`
	}
	cfg := jobConfig{
		MatchID:  matchID,
		MapSeed:  mapSeed,
		MaxTurns: maxTurns,
		Rows:     mapRows,
		Cols:     mapCols,
		Bots:     botCfgs,
	}
	configJSON, _ := json.Marshal(cfg)

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO matches (match_id, map_id, map_seed, status) VALUES ($1, $2, $3, 'pending')`,
		matchID, mapID, mapSeed); err != nil {
		return fmt.Errorf("insert match: %w", err)
	}

	// Build multi-row INSERT for participants.
	clauses := make([]string, len(participants))
	args := make([]interface{}, 0, 1+2*len(participants))
	args = append(args, matchID)
	for i, p := range participants {
		clauses[i] = fmt.Sprintf("($1, $%d, $%d)", 2+2*i, 3+2*i)
		args = append(args, p.ID, slots[i])
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO match_participants (match_id, bot_id, player_slot) VALUES "+strings.Join(clauses, ", "),
		args...); err != nil {
		return fmt.Errorf("insert participants: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO jobs (job_id, match_id, status, config_json) VALUES ($1, $2, 'pending', $3)`,
		jobID, matchID, configJSON); err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if err := m.rdb.LPush(ctx, valkeyJobQueue, jobID).Err(); err != nil {
		return fmt.Errorf("valkey push: %w", err)
	}

	depth, _ := m.rdb.LLen(ctx, valkeyJobQueue).Result()
	metrics.JobQueueDepth.Set(float64(depth))

	opIDs := make([]string, len(participants)-1)
	for i, p := range participants[1:] {
		opIDs[i] = p.ID
	}
	log.Printf("matchmaker: created %d-player match %s (seed=%s vs %v), job %s, map=%s",
		playerCount, matchID, participants[0].ID, opIDs, jobID, mapID)
	return nil
}

// tickHealthChecker pings each active bot's /health endpoint.
func (m *Matchmaker) tickHealthChecker(ctx context.Context) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT bot_id, endpoint_url, status, consec_fails FROM bots WHERE status IN ('active', 'inactive')`)
	if err != nil {
		log.Printf("health-checker: query error: %v", err)
		return
	}

	type botRow struct {
		ID          string
		Endpoint    string
		Status      string
		ConsecFails int
	}
	var bots []botRow
	for rows.Next() {
		var b botRow
		if err := rows.Scan(&b.ID, &b.Endpoint, &b.Status, &b.ConsecFails); err != nil {
			rows.Close()
			log.Printf("health-checker: scan error: %v", err)
			return
		}
		bots = append(bots, b)
	}
	rows.Close()

	client := &http.Client{Timeout: time.Duration(m.cfg.BotTimeoutSecs) * time.Second}

	var activeCount, failingCount int
	for _, bot := range bots {
		healthy := false
		resp, err := client.Get(bot.Endpoint + "/health")
		if err == nil {
			healthy = resp.StatusCode == http.StatusOK
			resp.Body.Close()
		}

		if healthy {
			activeCount++
			if bot.Status == "inactive" || bot.ConsecFails > 0 {
				m.db.ExecContext(ctx,
					`UPDATE bots SET status = 'active', consec_fails = 0, last_active = NOW()
					 WHERE bot_id = $1`, bot.ID)
				log.Printf("health-checker: %s recovered → active", bot.ID)
				if bot.Status == "inactive" {
					m.alerter.BotRecovered(ctx, bot.ID)
				}
			}
		} else {
			failingCount++
			newFails := bot.ConsecFails + 1
			newStatus := bot.Status
			if newFails >= m.cfg.MaxConsecFails {
				newStatus = "inactive"
			}
			m.db.ExecContext(ctx,
				`UPDATE bots SET status = $1, consec_fails = $2 WHERE bot_id = $3`,
				newStatus, newFails, bot.ID)
			if newStatus != bot.Status {
				log.Printf("health-checker: %s marked inactive after %d failures", bot.ID, newFails)
				m.alerter.BotMarkedInactive(ctx, bot.ID, newFails)
				metrics.BotCrashed.Inc()
			}
		}
	}

	metrics.BotsActive.Set(float64(activeCount))
	metrics.BotsFailing.Set(float64(failingCount))
}

// tickStaleReaper re-enqueues jobs that have been running too long.
func (m *Matchmaker) tickStaleReaper(ctx context.Context) {
	threshold := time.Duration(m.cfg.StaleJobMinutes) * time.Minute

	rows, err := m.db.QueryContext(ctx,
		`SELECT job_id FROM jobs
		 WHERE status = 'running' AND claimed_at < $1`,
		time.Now().Add(-threshold))
	if err != nil {
		log.Printf("stale-reaper: query error: %v", err)
		return
	}

	var staleJobs []string
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			rows.Close()
			return
		}
		staleJobs = append(staleJobs, jobID)
	}
	rows.Close()

	for _, jobID := range staleJobs {
		result, err := m.db.ExecContext(ctx,
			`UPDATE jobs SET status = 'pending', worker_id = NULL, claimed_at = NULL
			 WHERE job_id = $1 AND status = 'running'`, jobID)
		if err != nil {
			log.Printf("stale-reaper: update error for %s: %v", jobID, err)
			continue
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			continue // already completed or re-enqueued by another reaper
		}

		if err := m.rdb.LPush(ctx, valkeyJobQueue, jobID).Err(); err != nil {
			log.Printf("stale-reaper: re-enqueue error for %s: %v", jobID, err)
			continue
		}

		log.Printf("stale-reaper: re-enqueued stale job %s", jobID)
	}

	if len(staleJobs) > 0 {
		log.Printf("stale-reaper: processed %d stale jobs", len(staleJobs))
		m.alerter.StaleJobsReaped(ctx, staleJobs)
	}
	metrics.StaleJobCount.Set(float64(len(staleJobs)))
}

// queryActiveBotCount returns the number of active bots (used by tests).
func (m *Matchmaker) queryActiveBotCount(ctx context.Context) (int, error) {
	var count int
	err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bots WHERE status = 'active'`).Scan(&count)
	return count, err
}
