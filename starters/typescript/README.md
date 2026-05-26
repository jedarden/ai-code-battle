# AI Code Battle - TypeScript Starter Bot

A minimal TypeScript bot for AI Code Battle using Fastify framework with full type definitions.

## Quick Start

1. Copy this bot to your own repository
2. Edit `src/strategy.ts` to implement your bot's logic
3. Install: `npm install`
4. Build: `npm run build`
5. Run: `SHARED_SECRET=test npm start`
6. Test: `curl http://localhost:8080/health` should return "OK"

## Strategy Interface

Edit `src/strategy.ts`:

```typescript
export function computeMoves(state: VisibleState): Move[] {
    return state.bots
        .filter(bot => bot.owner === state.you.id)
        .map(bot => ({
            position: bot.position,
            direction: "" // Hold position
        }));
}
```

## Deployment

Build Docker image and push to registry, then register at https://ai-code-battle.pages.dev/#/register
