// Package seriesgate holds the one rule that decides whether a game inside a
// multi-game series is runnable (plan §14.7): a game waits until every earlier
// game in the same series has a result, and stops waiting once the series is
// no longer active (decided, or abandoned after a failure).
//
// The worker's job-claim query and the API's open-match feed both need the
// identical definition — the worker to keep games running in order, the API to
// keep matches that cannot run yet out of the prediction list — so the
// predicate lives here rather than being pasted into each caller.
package seriesgate

// Blocking returns a SQL EXISTS body (to be wrapped in NOT EXISTS) that is true
// when the match identified by matchRef belongs to an active series that still
// has an earlier game without a result. matchRef is a column reference visible
// in the enclosing query, e.g. "j.match_id" or "m.match_id".
func Blocking(matchRef string) string {
	return `SELECT 1
			FROM series_games mine
			JOIN series waited ON waited.id = mine.series_id AND waited.status = 'active'
			JOIN series_games earlier
			  ON earlier.series_id = mine.series_id
			 AND earlier.game_num < mine.game_num
			WHERE mine.match_id = ` + matchRef + `
			  AND earlier.winner_id IS NULL`
}
