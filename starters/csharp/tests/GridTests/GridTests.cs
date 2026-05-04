using Xunit;

// Minimal types needed for Grid tests
record Position
{
    public int Row { get; init; }
    public int Col { get; init; }
}

public class GridTests
{
    [Fact]
    public void ToroidalManhattan_SamePosition_ReturnsZero()
    {
        int result = Grid.ToroidalManhattan(5, 5, 5, 5, 10, 10);
        Assert.Equal(0, result);
    }

    [Fact]
    public void ToroidalManhattan_AdacentNoWrap_ReturnsOne()
    {
        int result = Grid.ToroidalManhattan(5, 5, 5, 6, 10, 10);
        Assert.Equal(1, result);
    }

    [Fact]
    public void ToroidalManhattan_WrapAroundRows_ShortestPath()
    {
        // On a 10x10 grid, distance from row 0 to row 9 should be 1 (wrap) not 9
        int result = Grid.ToroidalManhattan(0, 5, 9, 5, 10, 10);
        Assert.Equal(1, result);
    }

    [Fact]
    public void ToroidalManhattan_WrapAroundCols_ShortestPath()
    {
        // On a 10x10 grid, distance from col 0 to col 9 should be 1 (wrap) not 9
        int result = Grid.ToroidalManhattan(5, 0, 5, 9, 10, 10);
        Assert.Equal(1, result);
    }

    [Fact]
    public void ToroidalManhattan_DiagonalNoWrap_ReturnsTwo()
    {
        int result = Grid.ToroidalManhattan(5, 5, 6, 6, 10, 10);
        Assert.Equal(2, result);
    }

    [Fact]
    public void ToroidalManhattan_OppositeCorners_WrapPath()
    {
        // On a 10x10 grid, (0,0) to (9,9): best is wrap both ways = 1 + 1 = 2
        int result = Grid.ToroidalManhattan(0, 0, 9, 9, 10, 10);
        Assert.Equal(2, result);
    }

    [Fact]
    public void ToroidalChebyshev_SamePosition_ReturnsZero()
    {
        int result = Grid.ToroidalChebyshev(5, 5, 5, 5, 10, 10);
        Assert.Equal(0, result);
    }

    [Fact]
    public void ToroidalChebyshev_AdacentNoWrap_ReturnsOne()
    {
        int result = Grid.ToroidalChebyshev(5, 5, 5, 6, 10, 10);
        Assert.Equal(1, result);
    }

    [Fact]
    public void ToroidalChebyshev_DiagonalNoWrap_ReturnsOne()
    {
        int result = Grid.ToroidalChebyshev(5, 5, 6, 6, 10, 10);
        Assert.Equal(1, result);
    }

    [Fact]
    public void ToroidalChebyshev_WrapAroundRows_ShortestPath()
    {
        int result = Grid.ToroidalChebyshev(0, 5, 9, 5, 10, 10);
        Assert.Equal(1, result);
    }

    [Fact]
    public void ToroidalChebyshev_WrapAroundCols_ShortestPath()
    {
        int result = Grid.ToroidalChebyshev(5, 0, 5, 9, 10, 10);
        Assert.Equal(1, result);
    }

    [Fact]
    public void ToroidalChebyshev_KnightMoveNoWrap_ReturnsTwo()
    {
        int result = Grid.ToroidalChebyshev(5, 5, 7, 6, 10, 10);
        Assert.Equal(2, result);
    }

    [Fact]
    public void Neighbors_ReturnsEightPositions()
    {
        var pos = new Position { Row = 5, Col = 5 };
        var neighbors = Grid.Neighbors(pos, 10, 10);
        Assert.Equal(8, neighbors.Length);
    }

    [Fact]
    public void Neighbors_CornerPosition_WrapsCorrectly()
    {
        var pos = new Position { Row = 0, Col = 0 };
        var neighbors = Grid.Neighbors(pos, 10, 10);

        // Should include wrap-around positions
        Assert.Contains(neighbors, n => n.Row == 9 && n.Col == 9); // NW wrap
        Assert.Contains(neighbors, n => n.Row == 9 && n.Col == 0);  // N wrap
        Assert.Contains(neighbors, n => n.Row == 9 && n.Col == 1);  // NE wrap
        Assert.Contains(neighbors, n => n.Row == 0 && n.Col == 9);  // W wrap
        Assert.Contains(neighbors, n => n.Row == 0 && n.Col == 1);  // E
        Assert.Contains(neighbors, n => n.Row == 1 && n.Col == 9);  // SW wrap
        Assert.Contains(neighbors, n => n.Row == 1 && n.Col == 0);  // S
        Assert.Contains(neighbors, n => n.Row == 1 && n.Col == 1);  // SE
    }

    [Fact]
    public void Neighbors_EdgePosition_WrapsCorrectly()
    {
        var pos = new Position { Row = 0, Col = 5 };
        var neighbors = Grid.Neighbors(pos, 10, 10);

        // Should wrap on row axis only
        Assert.Contains(neighbors, n => n.Row == 9);
        Assert.Contains(neighbors, n => n.Row == 1);
    }

    [Fact]
    public void Neighbors_AllNeighborsAreDistinct()
    {
        var pos = new Position { Row = 5, Col = 5 };
        var neighbors = Grid.Neighbors(pos, 10, 10);
        var distinct = neighbors.DistinctBy(n => (n.Row, n.Col)).ToArray();
        Assert.Equal(8, distinct.Length);
    }

    [Fact]
    public void Bfs_StartEqualsGoal_ReturnsEmptyPath()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 5, Col = 5 };
        bool alwaysPassable(Position p) => true;

        var path = Grid.Bfs(start, goal, alwaysPassable, 10, 10);

        Assert.NotNull(path);
        Assert.Empty(path);
    }

    [Fact]
    public void Bfs_AdacentPosition_ReturnsSingleStep()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 5, Col = 6 };
        bool alwaysPassable(Position p) => true;

        var path = Grid.Bfs(start, goal, alwaysPassable, 10, 10);

        Assert.NotNull(path);
        Assert.Single(path);
        Assert.Equal(5, path[0].Row);
        Assert.Equal(6, path[0].Col);
    }

    [Fact]
    public void Bfs_UnobstructedPath_ReturnsShortestPath()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 5, Col = 8 };
        bool alwaysPassable(Position p) => true;

        var path = Grid.Bfs(start, goal, alwaysPassable, 10, 10);

        Assert.NotNull(path);
        Assert.Equal(3, path.Count);
    }

    [Fact]
    public void Bfs_WrappedPath_FindsShortestRoute()
    {
        var start = new Position { Row = 0, Col = 0 };
        var goal = new Position { Row = 9, Col = 9 };
        bool alwaysPassable(Position p) => true;

        var path = Grid.Bfs(start, goal, alwaysPassable, 10, 10);

        Assert.NotNull(path);
        // With 8-directional movement, NW from (0,0) wraps to (9,9) in 1 step
        Assert.Single(path);
        Assert.Equal(9, path[0].Row);
        Assert.Equal(9, path[0].Col);
    }

    [Fact]
    public void Bfs_AllBlocked_ReturnsNull()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 5, Col = 8 };
        bool neverPassable(Position p) => false;

        var path = Grid.Bfs(start, goal, neverPassable, 10, 10);

        Assert.Null(path);
    }

    [Fact]
    public void Bfs_GoalBlocked_ReturnsNull()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 5, Col = 8 };
        bool goalBlocked(Position p) => p.Row == 5 && p.Col == 8;

        var path = Grid.Bfs(start, goal, goalBlocked, 10, 10);

        Assert.Null(path);
    }

    [Fact]
    public void Bfs_NavigatesAroundObstacle()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 5, Col = 7 };

        // Block direct path
        bool hasWall(Position p) => !(p.Row == 5 && p.Col == 6);

        var path = Grid.Bfs(start, goal, hasWall, 10, 10);

        Assert.NotNull(path);
        Assert.NotEmpty(path);
        Assert.Equal(5, path[^1].Row);
        Assert.Equal(7, path[^1].Col);
    }

    [Fact]
    public void Bfs_PathDoesNotIncludeStart()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 5, Col = 7 };
        bool alwaysPassable(Position p) => true;

        var path = Grid.Bfs(start, goal, alwaysPassable, 10, 10);

        Assert.NotNull(path);
        Assert.DoesNotContain(path, p => p.Row == 5 && p.Col == 5);
    }

    [Fact]
    public void Bfs_PathEndsAtGoal()
    {
        var start = new Position { Row = 5, Col = 5 };
        var goal = new Position { Row = 7, Col = 8 };
        bool alwaysPassable(Position p) => true;

        var path = Grid.Bfs(start, goal, alwaysPassable, 10, 10);

        Assert.NotNull(path);
        Assert.Equal(7, path[^1].Row);
        Assert.Equal(8, path[^1].Col);
    }
}
