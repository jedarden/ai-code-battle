package db

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// openTestDB opens a PostgreSQL connection using ACB_TEST_DATABASE_URL.
// Tests that call this function are skipped when the env var is absent.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("ACB_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ACB_TEST_DATABASE_URL not set; skipping DB integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// setupTestSchema creates the programs table and registers cleanup to drop it.
func setupTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	t.Cleanup(func() {
		db.ExecContext(ctx, `DROP TABLE IF EXISTS programs`)
	})
}

func TestCreate_Get(t *testing.T) {
	db := openTestDB(t)
	setupTestSchema(t, db)
	s := NewStore(db)
	ctx := context.Background()

	p := &Program{
		Code:           "func strategy() {}",
		Language:       "go",
		Island:         IslandAlpha,
		Generation:     0,
		ParentIDs:      []int64{},
		BehaviorVector: []float64{0.9, 0.2},
		Fitness:        0.0,
		Promoted:       false,
	}

	id, err := s.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil for existing program")
	}
	if got.Code != p.Code {
		t.Errorf("Code: got %q, want %q", got.Code, p.Code)
	}
	if got.Language != p.Language {
		t.Errorf("Language: got %q, want %q", got.Language, p.Language)
	}
	if got.Island != p.Island {
		t.Errorf("Island: got %q, want %q", got.Island, p.Island)
	}
	if len(got.BehaviorVector) != 2 || got.BehaviorVector[0] != 0.9 || got.BehaviorVector[1] != 0.2 {
		t.Errorf("BehaviorVector: got %v, want [0.9 0.2]", got.BehaviorVector)
	}
}

func TestGet_NotFound(t *testing.T) {
	db := openTestDB(t)
	setupTestSchema(t, db)
	s := NewStore(db)
	ctx := context.Background()

	got, err := s.Get(ctx, 999999)
	if err != nil {
		t.Fatalf("Get non-existent: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent program")
	}
}

func TestListByIsland(t *testing.T) {
	db := openTestDB(t)
	setupTestSchema(t, db)
	s := NewStore(db)
	ctx := context.Background()

	programs := []*Program{
		{Code: "a", Language: "go", Island: IslandAlpha, BehaviorVector: []float64{0.9, 0.2}, Fitness: 10.0, ParentIDs: []int64{}},
		{Code: "b", Language: "go", Island: IslandAlpha, BehaviorVector: []float64{0.8, 0.3}, Fitness: 5.0, ParentIDs: []int64{}},
		{Code: "c", Language: "go", Island: IslandBeta, BehaviorVector: []float64{0.1, 0.9}, Fitness: 8.0, ParentIDs: []int64{}},
	}
	for _, p := range programs {
		if _, err := s.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	alphaList, err := s.ListByIsland(ctx, IslandAlpha)
	if err != nil {
		t.Fatalf("ListByIsland: %v", err)
	}
	if len(alphaList) != 2 {
		t.Fatalf("expected 2 programs on alpha, got %d", len(alphaList))
	}
	// Verify descending fitness order
	if alphaList[0].Fitness < alphaList[1].Fitness {
		t.Error("expected programs ordered by fitness DESC")
	}

	betaList, err := s.ListByIsland(ctx, IslandBeta)
	if err != nil {
		t.Fatalf("ListByIsland beta: %v", err)
	}
	if len(betaList) != 1 {
		t.Fatalf("expected 1 program on beta, got %d", len(betaList))
	}

	// Empty island returns empty slice (not an error)
	gammaList, err := s.ListByIsland(ctx, IslandGamma)
	if err != nil {
		t.Fatalf("ListByIsland gamma: %v", err)
	}
	if len(gammaList) != 0 {
		t.Errorf("expected empty gamma island, got %d programs", len(gammaList))
	}
}

func TestUpdateFitness(t *testing.T) {
	db := openTestDB(t)
	setupTestSchema(t, db)
	s := NewStore(db)
	ctx := context.Background()

	id, err := s.Create(ctx, &Program{
		Code: "x", Language: "go", Island: IslandDelta,
		BehaviorVector: []float64{0.3, 0.4}, ParentIDs: []int64{},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.UpdateFitness(ctx, id, 42.5, []float64{0.35, 0.45}); err != nil {
		t.Fatalf("UpdateFitness: %v", err)
	}

	got, _ := s.Get(ctx, id)
	if got.Fitness != 42.5 {
		t.Errorf("Fitness: got %f, want 42.5", got.Fitness)
	}
	if len(got.BehaviorVector) != 2 || got.BehaviorVector[0] != 0.35 {
		t.Errorf("BehaviorVector after update: got %v", got.BehaviorVector)
	}
}

func TestSetPromoted(t *testing.T) {
	db := openTestDB(t)
	setupTestSchema(t, db)
	s := NewStore(db)
	ctx := context.Background()

	id, err := s.Create(ctx, &Program{
		Code: "y", Language: "rust", Island: IslandGamma,
		BehaviorVector: []float64{0.7, 0.3}, ParentIDs: []int64{},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, _ := s.Get(ctx, id)
	if got.Promoted {
		t.Fatal("new program should not be promoted")
	}

	if err := s.SetPromoted(ctx, id); err != nil {
		t.Fatalf("SetPromoted: %v", err)
	}

	got, _ = s.Get(ctx, id)
	if !got.Promoted {
		t.Error("program should be promoted after SetPromoted")
	}
}

func TestCountByIsland(t *testing.T) {
	db := openTestDB(t)
	setupTestSchema(t, db)
	s := NewStore(db)
	ctx := context.Background()

	for _, prog := range []*Program{
		{Code: "1", Language: "go", Island: IslandAlpha, BehaviorVector: []float64{0.9, 0.1}, ParentIDs: []int64{}},
		{Code: "2", Language: "go", Island: IslandAlpha, BehaviorVector: []float64{0.8, 0.2}, ParentIDs: []int64{}},
		{Code: "3", Language: "go", Island: IslandBeta, BehaviorVector: []float64{0.1, 0.9}, ParentIDs: []int64{}},
	} {
		if _, err := s.Create(ctx, prog); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	counts, err := s.CountByIsland(ctx)
	if err != nil {
		t.Fatalf("CountByIsland: %v", err)
	}
	if counts[IslandAlpha] != 2 {
		t.Errorf("alpha count: got %d, want 2", counts[IslandAlpha])
	}
	if counts[IslandBeta] != 1 {
		t.Errorf("beta count: got %d, want 1", counts[IslandBeta])
	}
}

func TestParentIDs_Roundtrip(t *testing.T) {
	db := openTestDB(t)
	setupTestSchema(t, db)
	s := NewStore(db)
	ctx := context.Background()

	// First create two parent programs
	p1, _ := s.Create(ctx, &Program{Code: "parent1", Language: "go", Island: IslandAlpha, BehaviorVector: []float64{0.9, 0.2}, ParentIDs: []int64{}})
	p2, _ := s.Create(ctx, &Program{Code: "parent2", Language: "go", Island: IslandAlpha, BehaviorVector: []float64{0.8, 0.3}, ParentIDs: []int64{}})

	// Create child with both parents
	childID, err := s.Create(ctx, &Program{
		Code: "child", Language: "go", Island: IslandAlpha,
		Generation:     1,
		ParentIDs:      []int64{p1, p2},
		BehaviorVector: []float64{0.85, 0.25},
	})
	if err != nil {
		t.Fatalf("Create child: %v", err)
	}

	child, err := s.Get(ctx, childID)
	if err != nil {
		t.Fatalf("Get child: %v", err)
	}
	if len(child.ParentIDs) != 2 {
		t.Fatalf("expected 2 parent IDs, got %d", len(child.ParentIDs))
	}
	if child.ParentIDs[0] != p1 || child.ParentIDs[1] != p2 {
		t.Errorf("ParentIDs: got %v, want [%d %d]", child.ParentIDs, p1, p2)
	}
	if child.Generation != 1 {
		t.Errorf("Generation: got %d, want 1", child.Generation)
	}
}
