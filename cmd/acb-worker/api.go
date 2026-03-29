// API types for acb-worker
// HTTP API client removed - worker now uses direct PostgreSQL writes
package main

import (
	"time"
)

// Job represents a pending job (kept for compatibility).
type Job struct {
	ID          string     `json:"id"`
	MatchID     string     `json:"match_id"`
	Status      string     `json:"status"`
	WorkerID    *string    `json:"worker_id"`
	ClaimedAt   *time.Time `json:"claimed_at"`
	HeartbeatAt *time.Time `json:"heartbeat_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// JobClaimResponse contains the data needed to execute a match.
// This maps to JobClaimData from db.go for compatibility.
type JobClaimResponse struct {
	Job          Job           `json:"job"`
	Match        Match         `json:"match"`
	Participants []Participant `json:"participants"`
	Map          MapData       `json:"map"`
	Bots         []BotInfo     `json:"bots"`
	BotSecrets   []BotSecret   `json:"bot_secrets"`
}

// Match represents match metadata.
type Match struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	WinnerID    *string    `json:"winner_id"`
	Turns       *int       `json:"turns"`
	EndReason   *string    `json:"end_reason"`
	MapID       string     `json:"map_id"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// Participant represents a match participant.
type Participant struct {
	ID                   string `json:"id"`
	MatchID              string `json:"match_id"`
	BotID                string `json:"bot_id"`
	PlayerIndex          int    `json:"player_index"`
	Score                int    `json:"score"`
	RatingBefore         int    `json:"rating_before"`
	RatingAfter          *int   `json:"rating_after"`
	RatingDeviationBefore int   `json:"rating_deviation_before"`
	RatingDeviationAfter *int   `json:"rating_deviation_after"`
}

// MapData represents map configuration.
type MapData struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Walls  string `json:"walls"`
	Spawns string `json:"spawns"`
	Cores  string `json:"cores"`
}

// BotInfo contains bot endpoint information.
type BotInfo struct {
	ID          string `json:"id"`
	EndpointURL string `json:"endpoint_url"`
}

// BotSecret contains bot authentication secret.
type BotSecret struct {
	BotID  string `json:"bot_id"`
	Secret string `json:"secret"`
}

// MatchResult represents the result of a match for submission.
type MatchResult struct {
	WinnerID  string         `json:"winner_id"`
	Turns     int            `json:"turns"`
	EndReason string         `json:"end_reason"`
	Scores    map[string]int `json:"scores"`
}

// ConvertDBJobToJob converts a DBJob to Job type.
func ConvertDBJobToJob(dbJob *DBJob) *Job {
	if dbJob == nil {
		return nil
	}
	return &Job{
		ID:          dbJob.ID,
		MatchID:     dbJob.MatchID,
		Status:      dbJob.Status,
		WorkerID:    dbJob.WorkerID,
		ClaimedAt:   dbJob.ClaimedAt,
		HeartbeatAt: dbJob.HeartbeatAt,
		CreatedAt:   dbJob.CreatedAt,
	}
}

// ConvertDBClaimToResponse converts JobClaimData to JobClaimResponse.
func ConvertDBClaimToResponse(data *JobClaimData) *JobClaimResponse {
	if data == nil {
		return nil
	}

	// Convert participants
	participants := make([]Participant, len(data.Participants))
	botSecrets := make([]BotSecret, len(data.Participants))
	bots := make([]BotInfo, len(data.Bots))

	for i, p := range data.Participants {
		participants[i] = Participant{
			ID:                   p.MatchID + "-" + p.BotID,
			MatchID:              p.MatchID,
			BotID:                p.BotID,
			PlayerIndex:          p.PlayerSlot,
			Score:                p.Score,
			RatingBefore:         int(p.RatingMuBefore),
			RatingDeviationBefore: int(p.RatingPhiBefore),
		}
		botSecrets[i] = BotSecret{
			BotID:  p.BotID,
			Secret: "", // Will be filled from bots lookup
		}
	}

	// Convert bots and match secrets
	botSecretMap := make(map[string]string)
	for i, b := range data.Bots {
		bots[i] = BotInfo{
			ID:          b.ID,
			EndpointURL: b.EndpointURL,
		}
		botSecretMap[b.ID] = b.Secret
	}

	// Fill in secrets
	for i, p := range data.Participants {
		botSecrets[i].Secret = botSecretMap[p.BotID]
	}

	return &JobClaimResponse{
		Job: Job{
			ID:        data.Job.ID,
			MatchID:   data.Job.MatchID,
			Status:    data.Job.Status,
			WorkerID:  data.Job.WorkerID,
			ClaimedAt: data.Job.ClaimedAt,
			CreatedAt: data.Job.CreatedAt,
		},
		Match: Match{
			ID:        data.Match.ID,
			Status:    data.Match.Status,
			MapID:     data.Match.MapID,
			CreatedAt: data.Match.CreatedAt,
		},
		Participants: participants,
		Map: MapData{
			ID:     data.Map.ID,
			Width:  data.Map.Width,
			Height: data.Map.Height,
			Walls:  data.Map.Walls,
			Spawns: data.Map.Spawns,
			Cores:  data.Map.Cores,
		},
		Bots:       bots,
		BotSecrets: botSecrets,
	}
}
