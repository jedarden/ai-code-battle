// Narrative Engine - generates weekly meta report blog posts from match data.
// Optionally enhances prose via the Anthropic API when ANTHROPIC_API_KEY is set.

import type {
  ExportData,
  ExportMatch,
  ExportBot,
  BlogPost,
  BlogWeekStats,
  BlogIndex,
  EvolutionLiveData,
} from './types.js';

// ---------------------------------------------------------------------------
// Week helpers
// ---------------------------------------------------------------------------

function startOfWeek(d: Date): Date {
  const day = d.getUTCDay(); // 0=Sun
  const diff = (day === 0 ? -6 : 1 - day); // Monday
  const out = new Date(d);
  out.setUTCDate(d.getUTCDate() + diff);
  out.setUTCHours(0, 0, 0, 0);
  return out;
}

function isoDate(d: Date): string {
  return d.toISOString().slice(0, 10);
}

function weekSlug(weekStart: Date): string {
  return `week-${isoDate(weekStart)}`;
}

// ---------------------------------------------------------------------------
// Stats extraction
// ---------------------------------------------------------------------------

function matchesInWeek(matches: ExportMatch[], weekStart: Date): ExportMatch[] {
  const start = weekStart.getTime();
  const end = start + 7 * 24 * 60 * 60 * 1000;
  return matches.filter(m => {
    if (!m.completed_at) return false;
    const t = new Date(m.completed_at).getTime();
    return t >= start && t < end;
  });
}

function computeWeekStats(
  weekMatches: ExportMatch[],
  bots: ExportBot[],
  evo: EvolutionLiveData | null,
): BlogWeekStats {
  const botMap = new Map<string, ExportBot>(bots.map(b => [b.id, b]));

  // Top bot by rating
  const sorted = [...bots].sort((a, b) => b.rating - a.rating);
  const topBot = sorted[0];

  // Match activity per bot
  const activityCount = new Map<string, number>();
  for (const m of weekMatches) {
    for (const p of m.participants) {
      activityCount.set(p.bot_id, (activityCount.get(p.bot_id) ?? 0) + 1);
    }
  }
  let mostActiveBot = topBot?.name ?? 'N/A';
  let mostActiveBotMatches = 0;
  for (const [id, count] of activityCount) {
    if (count > mostActiveBotMatches) {
      mostActiveBotMatches = count;
      mostActiveBot = botMap.get(id)?.name ?? id;
    }
  }

  // Biggest upset: lower-rated bot beats higher-rated by the largest margin
  let biggestUpset: string | null = null;
  let maxUpsetMargin = 0;
  for (const m of weekMatches) {
    if (!m.winner_id || m.participants.length < 2) continue;
    const winner = m.participants.find(p => p.bot_id === m.winner_id);
    if (!winner) continue;
    const loser = m.participants.find(p => p.bot_id !== m.winner_id);
    if (!loser) continue;
    const winnerBot = botMap.get(winner.bot_id);
    const loserBot = botMap.get(loser.bot_id);
    if (!winnerBot || !loserBot) continue;
    const margin = loserBot.rating - winnerBot.rating;
    if (margin > maxUpsetMargin) {
      maxUpsetMargin = margin;
      biggestUpset = `${winnerBot.name} defeated ${loserBot.name} (+${Math.round(margin)} rating gap)`;
    }
  }

  // Island leader from evolution data
  let islandLeader: string | null = null;
  if (evo) {
    let bestFitness = -Infinity;
    for (const [island, stat] of Object.entries(evo.islands)) {
      if (stat.best_fitness > bestFitness) {
        bestFitness = stat.best_fitness;
        islandLeader = island;
      }
    }
  }

  return {
    matches_played: weekMatches.length,
    top_bot: topBot?.name ?? 'N/A',
    top_bot_rating: Math.round(topBot?.rating ?? 0),
    biggest_upset: biggestUpset,
    most_active_bot: mostActiveBot,
    most_active_bot_matches: mostActiveBotMatches,
    island_leader: islandLeader,
  };
}

// ---------------------------------------------------------------------------
// Template-based narrative (used when no LLM key is available)
// ---------------------------------------------------------------------------

function templateNarrative(weekStart: Date, stats: BlogWeekStats): { title: string; summary: string; body_html: string } {
  const weekLabel = isoDate(weekStart);
  const title = `Meta Report: Week of ${weekLabel}`;

  const summary =
    `This week ${stats.matches_played} matches were played. ` +
    `${stats.top_bot} leads the leaderboard at ${stats.top_bot_rating} rating. ` +
    (stats.biggest_upset
      ? `The biggest upset saw ${stats.biggest_upset}. `
      : '') +
    `${stats.most_active_bot} was the most active with ${stats.most_active_bot_matches} matches.`;

  const upsetSection = stats.biggest_upset
    ? `<h3>Biggest Upset</h3>
       <p>${stats.biggest_upset}.</p>`
    : '';

  const evoSection = stats.island_leader
    ? `<h3>Evolution Observatory</h3>
       <p>Island <strong>${stats.island_leader}</strong> leads the evolution pipeline this week.</p>`
    : '';

  const body_html = `
<h2>Overview</h2>
<p>
  The week of <strong>${weekLabel}</strong> produced <strong>${stats.matches_played}</strong> completed matches
  on the AI Code Battle platform.
</p>

<h3>Leaderboard Snapshot</h3>
<p>
  <strong>${stats.top_bot}</strong> holds the top position with a rating of
  <strong>${stats.top_bot_rating}</strong>. The competition remains fierce as bots jockey
  for position in the weekly rankings.
</p>

<h3>Most Active Competitor</h3>
<p>
  <strong>${stats.most_active_bot}</strong> played the most matches this week
  (<strong>${stats.most_active_bot_matches}</strong> games), demonstrating consistent
  availability and aggressive scheduling.
</p>

${upsetSection}

${evoSection}

<h3>What to Watch</h3>
<p>
  With the meta always shifting, next week promises fresh rivalries and strategy evolution.
  Keep an eye on the <a href="#/evolution">Evolution Dashboard</a> for emerging program
  lineages and the <a href="#/rivalries">Rivalries</a> page for head-to-head trends.
</p>
`.trim();

  return { title, summary, body_html };
}

// ---------------------------------------------------------------------------
// LLM-enhanced narrative (Anthropic API)
// ---------------------------------------------------------------------------

async function llmNarrative(
  weekStart: Date,
  stats: BlogWeekStats,
  templateResult: { title: string; summary: string; body_html: string },
): Promise<{ title: string; summary: string; body_html: string }> {
  const apiKey = process.env.ANTHROPIC_API_KEY;
  if (!apiKey) return templateResult;

  const prompt = `You are a sports journalist covering an AI bot programming competition.
Write a short, engaging weekly meta report for the week of ${isoDate(weekStart)}.

Statistics:
- Matches played: ${stats.matches_played}
- Top bot: ${stats.top_bot} (rating: ${stats.top_bot_rating})
- Most active bot: ${stats.most_active_bot} (${stats.most_active_bot_matches} matches)
- Biggest upset: ${stats.biggest_upset ?? 'none this week'}
- Evolution island leader: ${stats.island_leader ?? 'data not available'}

Write:
1. A catchy title (one line, no markdown)
2. A one-paragraph summary (plain text, 2-3 sentences)
3. Full HTML body content (use <h2>, <h3>, <p> tags; no <html>/<body>/<head>)

Format your response as JSON with keys: title, summary, body_html`;

  try {
    const res = await fetch('https://api.anthropic.com/v1/messages', {
      method: 'POST',
      headers: {
        'x-api-key': apiKey,
        'anthropic-version': '2023-06-01',
        'content-type': 'application/json',
      },
      body: JSON.stringify({
        model: 'claude-haiku-4-5-20251001',
        max_tokens: 1024,
        messages: [{ role: 'user', content: prompt }],
      }),
    });

    if (!res.ok) {
      console.warn(`LLM API returned ${res.status}, falling back to template narrative`);
      return templateResult;
    }

    const json = await res.json() as { content: Array<{ text: string }> };
    const text = json.content[0]?.text ?? '';

    // Extract JSON from response (may be wrapped in markdown code fences)
    const jsonMatch = text.match(/\{[\s\S]*\}/);
    if (!jsonMatch) {
      console.warn('LLM response did not contain JSON, using template');
      return templateResult;
    }

    const parsed = JSON.parse(jsonMatch[0]) as { title?: string; summary?: string; body_html?: string };
    return {
      title: parsed.title ?? templateResult.title,
      summary: parsed.summary ?? templateResult.summary,
      body_html: parsed.body_html ?? templateResult.body_html,
    };
  } catch (err) {
    console.warn('LLM narrative failed, using template:', err);
    return templateResult;
  }
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export async function generateWeeklyPost(
  data: ExportData,
  evo: EvolutionLiveData | null,
  weekStart?: Date,
): Promise<BlogPost> {
  const now = new Date();
  const week = weekStart ?? startOfWeek(now);

  const weekMatches = matchesInWeek(data.matches, week);
  const stats = computeWeekStats(weekMatches, data.bots, evo);

  const template = templateNarrative(week, stats);
  const narrative = await llmNarrative(week, stats, template);

  return {
    slug: weekSlug(week),
    title: narrative.title,
    published_at: now.toISOString(),
    week_start: isoDate(week),
    summary: narrative.summary,
    body_html: narrative.body_html,
    stats,
  };
}

export function buildBlogIndex(posts: BlogPost[]): BlogIndex {
  return {
    updated_at: new Date().toISOString(),
    posts: posts.sort((a, b) => b.week_start.localeCompare(a.week_start)),
  };
}

/**
 * Compute the start-of-week dates for the last N weeks.
 */
export function lastNWeekStarts(n: number, from?: Date): Date[] {
  const base = startOfWeek(from ?? new Date());
  const weeks: Date[] = [];
  for (let i = 0; i < n; i++) {
    const d = new Date(base);
    d.setUTCDate(base.getUTCDate() - i * 7);
    weeks.push(d);
  }
  return weeks;
}
