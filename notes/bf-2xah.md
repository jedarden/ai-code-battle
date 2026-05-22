# Bot Registration to Matchmaking Integration

## Task
Verify that POST /api/register is wired up to active matchmaking for third-party bots.

## Analysis

### Registration Flow (cmd/acb-api/server.go:215-216)
```go
INSERT INTO bots (bot_id, name, owner, endpoint_url, shared_secret, status, debug_public)
VALUES ($1, $2, $3, $4, $5, 'active', $6)
```
Bots registered via the API are immediately set to `status = 'active'`.

### Matchmaking Query (cmd/acb-matchmaker/tickers.go:163)
```sql
WHERE b.status = 'active'
  AND (b.cooldown_until IS NULL OR b.cooldown_until < NOW())
```
The matchmaker queries for all active bots (not on cooldown) as eligible candidates.

## Conclusion
The integration is **already complete**. No code changes are required:
- Third-party bots register → get `status = 'active'`
- Matchmaker includes all active bots in the pool
- Bots immediately become eligible for pairing

The "deferred" note in docs/plan/plan.md (lines 79-84, 311) refers to the Go API service not being deployed for v1, not to missing integration code. When the API deploys, third-party bots will work immediately.
