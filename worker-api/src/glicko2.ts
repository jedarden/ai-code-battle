// Glicko-2 Rating System Implementation
// Based on: http://www.glicko.net/glicko/glicko2.pdf

import type { Env, Bot, MatchParticipant } from './types';

// Glicko-2 constants
const SCALE = 173.7178; // Rating scale conversion factor
const TAU = 0.5; // System constant (constrains volatility change)
const DEFAULT_RATING = 1500;
const DEFAULT_RD = 350;
const DEFAULT_VOLATILITY = 0.06;

export interface Glicko2Rating {
  mu: number; // Mean rating (Glicko-2 scale)
  phi: number; // Rating deviation (Glicko-2 scale)
  sigma: number; // Volatility
}

/**
 * Convert rating to Glicko-2 scale
 */
export function toGlicko2(rating: number, rd: number): Glicko2Rating {
  return {
    mu: (rating - DEFAULT_RATING) / SCALE,
    phi: rd / SCALE,
    sigma: DEFAULT_VOLATILITY,
  };
}

/**
 * Convert from Glicko-2 scale to original scale
 */
export function fromGlicko2(g2: Glicko2Rating): { rating: number; rd: number } {
  return {
    rating: g2.mu * SCALE + DEFAULT_RATING,
    rd: g2.phi * SCALE,
  };
}

/**
 * Compute g(phi) function
 */
function g(phi: number): number {
  return 1 / Math.sqrt(1 + (3 * phi * phi) / (Math.PI * Math.PI));
}

/**
 * Compute E(mu, mu_j, phi_j) function
 */
function E(mu: number, mu_j: number, phi_j: number): number {
  return 1 / (1 + Math.exp(-g(phi_j) * (mu - mu_j)));
}

/**
 * Compute new rating deviation (Step 5/6)
 */
function computeNewPhi(phi: number, v: number): number {
  const phiSquared = phi * phi;
  const vInverse = 1 / v;
  return 1 / Math.sqrt(1 / phiSquared + vInverse);
}

/**
 * Iterative algorithm to compute new volatility (Step 5.4)
 */
function computeNewVolatility(
  sigma: number,
  phi: number,
  v: number,
  delta: number,
  tau: number = TAU
): number {
  let a = Math.log(sigma * sigma);
  const epsilon = 0.000001;

  const f = (x: number): number => {
    const expX = Math.exp(x);
    const tmp = phi * phi + v + expX;
    return (
      (expX * (delta * delta - phi * phi - v - expX)) / (2 * tmp * tmp) -
      (x - a) / (tau * tau)
    );
  };

  // Set initial bounds
  let A = a;
  let B: number;
  if (delta * delta > phi * phi + v) {
    B = Math.log(delta * delta - phi * phi - v);
  } else {
    let k = 1;
    while (f(a - k * tau) < 0) {
      k++;
    }
    B = a - k * tau;
  }

  // Illinois algorithm
  let fA = f(A);
  let fB = f(B);

  while (Math.abs(B - A) > epsilon) {
    const C = A + ((A - B) * fA) / (fB - fA);
    const fC = f(C);

    if (fC * fB <= 0) {
      A = B;
      fA = fB;
    } else {
      fA = fA / 2;
    }

    B = C;
    fB = fC;
  }

  return Math.exp(A / 2);
}

/**
 * Calculate rating updates for a bot after a match
 * @param bot The bot whose rating to update
 * @param opponents Array of opponent ratings and game outcomes (1=win, 0.5=draw, 0=loss)
 * @returns New rating values
 */
export function updateRating(
  bot: Bot,
  opponents: Array<{
    rating: number;
    rd: number;
    score: number;
  }>
): { rating: number; rd: number; volatility: number } {
  if (opponents.length === 0) {
    // No games played - increase RD over time (rating decay)
    const phi = bot.rating_deviation / SCALE;
    const newPhi = Math.min(Math.sqrt(phi * phi + bot.rating_volatility * bot.rating_volatility), 350 / SCALE);
    return {
      rating: bot.rating,
      rd: newPhi * SCALE,
      volatility: bot.rating_volatility,
    };
  }

  // Convert to Glicko-2 scale
  const g2 = toGlicko2(bot.rating, bot.rating_deviation);
  g2.sigma = bot.rating_volatility;

  // Step 3: Compute v (variance of game outcomes)
  let vInverse = 0;
  for (const opp of opponents) {
    const oppG2 = toGlicko2(opp.rating, opp.rd);
    const gPhi = g(oppG2.phi);
    const eValue = E(g2.mu, oppG2.mu, oppG2.phi);
    vInverse += gPhi * gPhi * eValue * (1 - eValue);
  }
  const v = 1 / vInverse;

  // Step 4: Compute delta (rating improvement)
  let deltaSum = 0;
  for (const opp of opponents) {
    const oppG2 = toGlicko2(opp.rating, opp.rd);
    const gPhi = g(oppG2.phi);
    const eValue = E(g2.mu, oppG2.mu, oppG2.phi);
    deltaSum += gPhi * (opp.score - eValue);
  }
  const delta = v * deltaSum;

  // Step 5: Compute new volatility
  const newSigma = computeNewVolatility(g2.sigma, g2.phi, v, delta);

  // Step 6: Update phi
  const phiStar = Math.sqrt(g2.phi * g2.phi + newSigma * newSigma);

  // Step 7: Update phi and mu
  const newPhi = 1 / Math.sqrt(1 / (phiStar * phiStar) + 1 / v);
  const newMu = g2.mu + newPhi * newPhi * deltaSum;

  // Convert back
  const result = fromGlicko2({ mu: newMu, phi: newPhi, sigma: newSigma });

  return {
    rating: result.rating,
    rd: result.rd,
    volatility: newSigma,
  };
}

/**
 * Update ratings for all participants in a completed match
 */
export async function updateMatchRatings(
  env: Env,
  matchId: string,
  participants: MatchParticipant[],
  winnerId: string | null
): Promise<void> {
  // Get all bots involved
  const botIds = participants.map((p) => p.bot_id);
  const placeholders = botIds.map(() => '?').join(',');

  const bots = await env.DB.prepare(
    `SELECT * FROM bots WHERE id IN (${placeholders})`
  )
    .bind(...botIds)
    .all<Bot>();

  if (!bots.results || bots.results.length !== participants.length) {
    throw new Error('Could not find all participant bots');
  }

  const botMap = new Map(bots.results.map((b) => [b.id, b]));

  // Calculate new ratings for each participant
  const updates: Array<{
    botId: string;
    rating: number;
    rd: number;
    volatility: number;
    won: boolean;
  }> = [];

  for (const participant of participants) {
    const bot = botMap.get(participant.bot_id);
    if (!bot) continue;

    // Build opponent list
    const opponents = participants
      .filter((p) => p.bot_id !== participant.bot_id)
      .map((opp) => {
        const oppBot = botMap.get(opp.bot_id)!;
        // Score: 1 for win, 0.5 for draw (if no winner), 0 for loss
        let score = 0.5;
        if (winnerId === participant.bot_id) {
          score = 1;
        } else if (winnerId === opp.bot_id) {
          score = 0;
        }
        return {
          rating: oppBot.rating,
          rd: oppBot.rating_deviation,
          score,
        };
      });

    const newRating = updateRating(bot, opponents);
    const won = winnerId === participant.bot_id;

    updates.push({
      botId: participant.bot_id,
      rating: newRating.rating,
      rd: newRating.rd,
      volatility: newRating.volatility,
      won,
    });
  }

  // Apply updates in a batch
  const now = new Date().toISOString();

  for (const update of updates) {
    // Update bot rating
    await env.DB.prepare(
      `UPDATE bots SET
        rating = ?,
        rating_deviation = ?,
        rating_volatility = ?,
        matches_played = matches_played + 1,
        matches_won = matches_won + ?,
        updated_at = ?
      WHERE id = ?`
    )
      .bind(
        update.rating,
        update.rd,
        update.volatility,
        update.won ? 1 : 0,
        now,
        update.botId
      )
      .run();

    // Update participant with rating change
    await env.DB.prepare(
      `UPDATE match_participants SET
        rating_after = ?,
        rating_deviation_after = ?
      WHERE match_id = ? AND bot_id = ?`
    )
      .bind(update.rating, update.rd, matchId, update.botId)
      .run();

    // Record rating history
    await env.DB.prepare(
      `INSERT INTO rating_history (id, bot_id, match_id, rating_before, rating_after, rating_deviation, recorded_at)
       VALUES (?, ?, ?, ?, ?, ?, ?)`
    )
      .bind(
        crypto.randomUUID(),
        update.botId,
        matchId,
        botMap.get(update.botId)!.rating,
        update.rating,
        update.rd,
        now
      )
      .run();
  }
}
