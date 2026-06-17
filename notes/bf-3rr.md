# Leader-Targeter Bot Implementation (Bead bf-3rr)

## Summary
The leader-targeter bot was successfully implemented in Java with Maven build system.

## Implementation Details

### Strategy
The bot implements multi-player game theory by:
1. **Score Calculation**: Estimates opponent scores based on visible bots (10 points each) and active cores (2 points each)
2. **Target Selection**: Identifies the score leader and directs all units toward them
3. **Tiebreaking**: Uses nearest distance when scores are tied
4. **Core Defense**: Detaches 2 bots to defend own core when enemy is within 6 tiles
5. **2-Player Fallback**: Falls back to straight aggression when only one opponent exists

### Files
- `bots/leader-targeter/src/main/java/com/acb/targeter/App.java` - HTTP server with signature verification
- `bots/leader-targeter/src/main/java/com/acb/targeter/LeaderTargeterStrategy.java` - Core strategy implementation
- `bots/leader-targeter/src/main/java/com/acb/targeter/GameState.java` - Game state parsing
- `bots/leader-targeter/Dockerfile` - Docker build configuration
- `bots/leader-targeter/pom.xml` - Maven build configuration

### Build
- Maven shade plugin creates executable JAR with bundled dependencies
- Javalin HTTP server on port 8085
- Jackson for JSON processing
- Java 21 target

## Retrospective

### What worked
The Maven project structure with Javalin and Jackson provides a clean foundation for bot implementations. The strategy correctly models the N-player problem and applies the kingmaker dynamic.

### What didn't
N/A - implementation was straightforward

### Surprise
The bot was already fully implemented in a previous commit, so this task primarily involved verifying the build and pushing existing work.

### Reusable pattern
- Maven shade plugin configuration for executable JARs
- Toroidal distance calculations for grid-based games
- Centroid calculation for targeting multiple assets
