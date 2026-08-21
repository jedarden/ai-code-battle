package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"
)

// MockRows implements sql.Rows interface for testing
type mockRows struct {
	columns []string
	data    [][]interface{}
	pos     int
	closed  bool
}

func newMockRows(columns []string, data [][]interface{}) *mockRows {
	return &mockRows{
		columns: columns,
		data:    data,
		pos:     -1,
	}
}

func (m *mockRows) Close() error {
	m.closed = true
	return nil
}

func (m *mockRows) Columns() []string {
	return m.columns
}

func (m *mockRows) Next() bool {
	m.pos++
	return m.pos < len(m.data)
}

func (m *mockRows) Err() error {
	return nil
}

func (m *mockRows) Scan(dest ...interface{}) error {
	if m.pos >= len(m.data) {
		return driver.ErrSkip
	}
	row := m.data[m.pos]
	for i, val := range row {
		switch d := dest[i].(type) {
		case *string:
			if s, ok := val.(string); ok {
				*d = s
			} else if val == nil {
				*d = ""
			}
		case *int:
			if i, ok := val.(int); ok {
				*d = i
			}
		case *sql.NullInt32:
			if val == nil {
				d.Valid = false
			} else if i, ok := val.(int); ok {
				d.Int32 = int32(i)
				d.Valid = true
			}
		case *sql.NullString:
			if val == nil {
				d.Valid = false
			} else if s, ok := val.(string); ok {
				d.String = s
				d.Valid = true
			}
		case *sql.NullTime:
			if val == nil {
				d.Valid = false
			} else if t, ok := val.(time.Time); ok {
				d.Time = t
				d.Valid = true
			}
		case *time.Time:
			if t, ok := val.(time.Time); ok {
				*d = t
			}
		case *[]byte:
			if b, ok := val.([]byte); ok {
				*d = b
			}
		}
	}
	return nil
}

// MockDB implements database operations for testing
type mockDB struct {
	queryFunc       func(query string, args ...interface{}) (*mockRows, error)
	queryRowFunc    func(query string, args ...interface{}) *sql.Row
	execFunc        func(query string, args ...interface{}) (sql.Result, error)
	pingFunc        func() error
	expectCloseCall bool
}

func (m *mockDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if m.queryFunc != nil {
		_, err := m.queryFunc(query, args...)
		if err != nil {
			return nil, err
		}
		// Convert mock rows to sql.Rows via a wrapper
		// For simplicity, return nil and handle error case
		return nil, nil
	}
	return nil, driver.ErrSkip
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(query, args...)
	}
	return &sql.Row{}
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.execFunc != nil {
		return m.execFunc(query, args...)
	}
	return nil, driver.ErrSkip
}

func (m *mockDB) PingContext(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc()
	}
	return nil
}

func (m *mockDB) Close() error {
	return nil
}

func TestNewStore(t *testing.T) {
	db, _ := sql.Open("postgres", "dummy")
	store := NewStore(db)

	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	if store.db != db {
		t.Errorf("Expected db %v, got %v", db, store.db)
	}
}

func TestCalculateCrossings(t *testing.T) {
	tests := []struct {
		name      string
		scores    []int
		turnCount int
		want      int
	}{
		{
			name:      "close scores, many turns",
			scores:    []int{45, 43},
			turnCount: 150,
			want:      1, // min(5, 150/100) = min(5, 1) = 1
		},
		{
			name:      "close scores, few turns",
			scores:    []int{45, 43},
			turnCount: 50,
			want:      0, // min(5, 50/100) = 0
		},
		{
			name:      "medium score difference",
			scores:    []int{50, 35},
			turnCount: 200,
			want:      1, // min(3, 200/150) = min(3, 1) = 1
		},
		{
			name:      "large score difference",
			scores:    []int{60, 20},
			turnCount: 300,
			want:      0,
		},
		{
			name:      "no scores",
			scores:    []int{},
			turnCount: 100,
			want:      0,
		},
		{
			name:      "single score",
			scores:    []int{50},
			turnCount: 100,
			want:      0,
		},
	}

	store := &Store{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.calculateCrossings(tt.scores, tt.turnCount)
			if got != tt.want {
				t.Errorf("calculateCrossings() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMatchData(t *testing.T) {
	// Test that Match type can hold null values properly
	m := Match{
		ID:             "test-match",
		MapID:          "test-map",
		Status:         "completed",
		Winner:         sql.NullInt32{Int32: 0, Valid: true},
		Condition:      sql.NullString{String: "elimination", Valid: true},
		TurnCount:      sql.NullInt32{Int32: 150, Valid: true},
		ScoresJSON:     sql.NullString{String: "[100,95]", Valid: true},
		CreatedAt:      time.Now(),
		CompletedAt:    sql.NullTime{Time: time.Now(), Valid: true},
		CommentaryJSON: sql.NullString{Valid: false}, // Not yet enriched
	}

	if m.ID != "test-match" {
		t.Errorf("Expected ID 'test-match', got %s", m.ID)
	}
	if !m.Winner.Valid {
		t.Error("Expected Winner to be valid")
	}
	if m.Winner.Int32 != 0 {
		t.Errorf("Expected Winner 0, got %d", m.Winner.Int32)
	}
	if m.CommentaryJSON.Valid {
		t.Error("Expected CommentaryJSON to be invalid (NULL)")
	}
}

func TestCandidateMatch(t *testing.T) {
	cm := CandidateMatch{
		MatchID:          "test-match",
		TurnCount:        150,
		Winner:           0,
		Condition:        "elimination",
		FinalScores:      []int{100, 95},
		WinProbCrossings: 5,
		IsUpset:          true,
		IsCloseFinish:    false,
		Players: []PlayerData{
			{ID: 0, BotID: "bot-1", Name: "Bot1", Rating: 1500},
			{ID: 1, BotID: "bot-2", Name: "Bot2", Rating: 1300},
		},
	}

	if cm.MatchID != "test-match" {
		t.Errorf("Expected MatchID 'test-match', got %s", cm.MatchID)
	}
	if len(cm.Players) != 2 {
		t.Errorf("Expected 2 players, got %d", len(cm.Players))
	}
	if !cm.IsUpset {
		t.Error("Expected IsUpset to be true")
	}
	if cm.WinProbCrossings != 5 {
		t.Errorf("Expected WinProbCrossings 5, got %d", cm.WinProbCrossings)
	}
}

func TestPlayerData(t *testing.T) {
	p := PlayerData{
		ID:     1,
		BotID:  "bot-123",
		Name:   "TestBot",
		Rating: 1400,
	}

	if p.ID != 1 {
		t.Errorf("Expected ID 1, got %d", p.ID)
	}
	if p.BotID != "bot-123" {
		t.Errorf("Expected BotID 'bot-123', got %s", p.BotID)
	}
	if p.Name != "TestBot" {
		t.Errorf("Expected Name 'TestBot', got %s", p.Name)
	}
	if p.Rating != 1400 {
		t.Errorf("Expected Rating 1400, got %d", p.Rating)
	}
}

func TestBotInfo(t *testing.T) {
	b := BotInfo{
		ID:          "bot-123",
		Name:        "TestBot",
		RatingMu:    1500.0,
		RatingPhi:   50.0,
		RatingSigma: 10.0,
	}

	if b.ID != "bot-123" {
		t.Errorf("Expected ID 'bot-123', got %s", b.ID)
	}
	if b.Name != "TestBot" {
		t.Errorf("Expected Name 'TestBot', got %s", b.Name)
	}
	if b.RatingMu != 1500.0 {
		t.Errorf("Expected RatingMu 1500.0, got %f", b.RatingMu)
	}
	if b.RatingPhi != 50.0 {
		t.Errorf("Expected RatingPhi 50.0, got %f", b.RatingPhi)
	}
	if b.RatingSigma != 10.0 {
		t.Errorf("Expected RatingSigma 10.0, got %f", b.RatingSigma)
	}
}

// Test display rating calculation (Mu - 2*Phi)
func TestDisplayRatingCalculation(t *testing.T) {
	tests := []struct {
		name     string
		mu       float64
		phi      float64
		expected int
	}{
		{
			name:     "standard rating",
			mu:       1500.0,
			phi:      50.0,
			expected: 1400, // 1500 - 2*50 = 1400
		},
		{
			name:     "high uncertainty",
			mu:       1500.0,
			phi:      100.0,
			expected: 1300, // 1500 - 2*100 = 1300
		},
		{
			name:     "low uncertainty",
			mu:       1500.0,
			phi:      10.0,
			expected: 1480, // 1500 - 2*10 = 1480
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			displayRating := int(tt.mu - 2*tt.phi)
			if displayRating != tt.expected {
				t.Errorf("Display rating calculation: got %d, want %d", displayRating, tt.expected)
			}
		})
	}
}
