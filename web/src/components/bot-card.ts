// Bot profile card renderer — Canvas-rendered PNG with OG tags
// §14.10: Auto-generated visual cards summarizing a bot's identity, stats, and character

import type { BotProfile } from '../api-types';

// Card dimensions (social sharing optimized)
const CARD_WIDTH = 800;
const CARD_HEIGHT = 450;

// Color palette
const COLORS = {
  bg: '#0f172a',
  bgSecondary: '#1e293b',
  text: '#f8fafc',
  textMuted: '#94a3b8',
  accent: '#3b82f6',
  success: '#22c55e',
  warning: '#eab308',
  border: '#334155',
  player1: '#3b82f6',
  player2: '#ef4444',
  player3: '#22c55e',
  player4: '#eab308',
  player5: '#a855f7',
  player6: '#06b6d4',
};

// Archetype labels (simplified for card display)
const ARCHETYPE_LABELS: Record<string, string> = {
  aggressive: 'AGGRESSIVE RUSHER',
  defensive: 'DEFENSIVE TURTLE',
  economic: 'ENERGY HOARDER',
  balanced: 'BALANCED FIGHTER',
  swarm: 'FORMATION SWARM',
  hunter: 'TARGET HUNTER',
  gatherer: 'RESOURCE GATHERER',
  unknown: 'STRATEGY UNKNOWN',
};

/**
 * Render a bot profile card to an offscreen canvas and return as PNG blob
 */
export async function renderBotCard(
  profile: BotProfile,
  rank?: number,
  _rivals?: { name: string; record: string }[],
): Promise<Blob> {
  const canvas = document.createElement('canvas');
  canvas.width = CARD_WIDTH;
  canvas.height = CARD_HEIGHT;
  const ctx = canvas.getContext('2d')!;

  // Background
  ctx.fillStyle = COLORS.bg;
  ctx.fillRect(0, 0, CARD_WIDTH, CARD_HEIGHT);

  // Decorative gradient header
  const gradient = ctx.createLinearGradient(0, 0, CARD_WIDTH, 0);
  gradient.addColorStop(0, 'rgba(59, 130, 246, 0.15)');
  gradient.addColorStop(1, 'rgba(168, 85, 247, 0.15)');
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, CARD_WIDTH, 80);

  // Top section: Bot name and rank
  drawHeader(ctx, profile, rank);

  // Middle section: Main stats box
  drawStatsBox(ctx, profile);

  // Bottom section: Win rates and signature
  drawWinRates(ctx, profile);

  // Footer: Platform URL
  drawFooter(ctx);

  // Convert to PNG blob
  return new Promise((resolve) => {
    canvas.toBlob((blob) => {
      resolve(blob!);
    }, 'image/png');
  });
}

/**
 * Draw the header section with bot name, rank, and rating
 */
function drawHeader(ctx: CanvasRenderingContext2D, profile: BotProfile, rank?: number): void {
  const y = 25;
  const leftX = 20;

  // Bot name
  ctx.fillStyle = COLORS.text;
  ctx.font = 'bold 28px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';
  ctx.textAlign = 'left';
  ctx.textBaseline = 'middle';
  ctx.fillText(truncateText(profile.name, 30), leftX, y);

  // Rank badge
  if (rank !== undefined) {
    const rankText = `#${rank}`;
    const rankWidth = ctx.measureText(rankText).width + 16;
    ctx.fillStyle = COLORS.accent;
    roundRect(ctx, CARD_WIDTH - 20 - rankWidth, y - 14, rankWidth, 28, 6);
    ctx.fillStyle = '#fff';
    ctx.font = 'bold 16px -apple-system, sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText(rankText, CARD_WIDTH - 20 - rankWidth / 2, y);
  }

  // Owner and rating
  ctx.fillStyle = COLORS.textMuted;
  ctx.font = '14px -apple-system, sans-serif';
  ctx.textAlign = 'left';
  ctx.fillText(`by ${truncateText(profile.owner_id, 25)}`, leftX, y + 30);

  const ratingText = `Rating: ${Math.round(profile.rating)}`;
  ctx.textAlign = 'right';
  ctx.fillText(ratingText, CARD_WIDTH - 20, y + 30);

  // Evolved badge
  if (profile.evolved) {
    const evolvedText = 'EVOLVED';
    const evolvedWidth = ctx.measureText(evolvedText).width + 12;
    ctx.fillStyle = 'rgba(168, 85, 247, 0.3)';
    ctx.strokeStyle = COLORS.player5;
    ctx.lineWidth = 1;
    roundRect(ctx, leftX, y + 42, evolvedWidth, 20, 4, true, true);
    ctx.fillStyle = COLORS.player5;
    ctx.font = 'bold 11px -apple-system, sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText(evolvedText, leftX + evolvedWidth / 2, y + 52);
  }
}

/**
 * Draw the main stats box with archetype and season info
 */
function drawStatsBox(ctx: CanvasRenderingContext2D, profile: BotProfile): void {
  const boxX = 20;
  const boxY = 90;
  const boxW = CARD_WIDTH - 40;
  const boxH = 140;

  // Box background
  ctx.fillStyle = COLORS.bgSecondary;
  roundRect(ctx, boxX, boxY, boxW, boxH, 12);

  // Border
  ctx.strokeStyle = COLORS.border;
  ctx.lineWidth = 1;
  roundRect(ctx, boxX, boxY, boxW, boxH, 12, false, true);

  // Archetype section
  const archetype = inferArchetype(profile);
  ctx.fillStyle = COLORS.text;
  ctx.font = 'bold 14px -apple-system, sans-serif';
  ctx.textAlign = 'left';
  ctx.fillText('Archetype:', boxX + 16, boxY + 24);

  ctx.fillStyle = COLORS.accent;
  ctx.font = 'bold 13px -apple-system, sans-serif';
  ctx.fillText(ARCHETYPE_LABELS[archetype] || ARCHETYPE_LABELS.unknown, boxX + 16, boxY + 44);

  // Season info (use current date as proxy for season)
  const seasonInfo = `${new Date().getFullYear()} Season · ${profile.matches_played} games`;
  ctx.fillStyle = COLORS.textMuted;
  ctx.font = '12px -apple-system, sans-serif';
  ctx.fillText(seasonInfo, boxX + 16, boxY + 64);

  // Stats row
  const statsY = boxY + 90;
  const statSpacing = (boxW - 32) / 3;

  // Matches
  drawStat(ctx, boxX + 16, statsY, profile.matches_played.toString(), 'Matches', COLORS.text);

  // Win rate
  const winRate = profile.win_rate.toFixed(1) + '%';
  drawStat(ctx, boxX + 16 + statSpacing, statsY, winRate, 'Win Rate', COLORS.success);

  // Rating deviation
  const rd = '±' + Math.round(profile.rating_deviation);
  drawStat(ctx, boxX + 16 + statSpacing * 2, statsY, rd, 'Uncertainty', COLORS.textMuted);
}

/**
 * Draw win rate bars and signature/rival info
 */
function drawWinRates(ctx: CanvasRenderingContext2D, profile: BotProfile): void {
  const startY = 250;
  const leftX = 20;
  const rightX = CARD_WIDTH / 2 + 10;
  const colWidth = CARD_WIDTH / 2 - 30;

  // Win rate bar (calculated from matches_won / matches_played)
  const winRate = profile.matches_won / Math.max(1, profile.matches_played);
  const barWidth = Math.round(winRate * 100);
  void barWidth; // Mark as intentionally unused for now

  // Label
  ctx.fillStyle = COLORS.text;
  ctx.font = 'bold 13px -apple-system, sans-serif';
  ctx.textAlign = 'left';
  ctx.fillText('Win Rate', leftX, startY);

  // Bar background
  const barY = startY + 12;
  ctx.fillStyle = COLORS.bgSecondary;
  roundRect(ctx, leftX, barY, colWidth, 20, 4);

  // Bar fill
  const fillWidth = (colWidth * barWidth) / 100;
  ctx.fillStyle = winRate >= 0.5 ? COLORS.success : COLORS.warning;
  roundRect(ctx, leftX, barY, fillWidth, 20, 4);

  // Percentage text
  ctx.fillStyle = '#fff';
  ctx.font = 'bold 12px -apple-system, sans-serif';
  ctx.textAlign = 'center';
  ctx.fillText(`${barWidth}%`, leftX + colWidth / 2, barY + 26);

  // Stats icons (kills, energy, captures - placeholder since not in profile)
  const statsY = barY + 40;
  ctx.fillStyle = COLORS.textMuted;
  ctx.font = '11px -apple-system, sans-serif';
  ctx.textAlign = 'left';
  ctx.fillText('⚔️ ' + profile.matches_won + ' wins', leftX, statsY);
  ctx.fillText('💎 ' + Math.round(profile.matches_played * profile.win_rate / 100) + ' efficiency', leftX, statsY + 18);

  // Right column: Signature move
  ctx.fillStyle = COLORS.text;
  ctx.font = 'bold 13px -apple-system, sans-serif';
  ctx.fillText('Signature', rightX, startY);

  const signature = generateSignature(profile);
  ctx.fillStyle = COLORS.textMuted;
  ctx.font = '11px -apple-system, sans-serif';
  wrapText(ctx, signature, rightX, startY + 16, colWidth, 14);

  // Rival info (if available)
  if (profile.recent_matches.length > 0) {
    const rivalName = profile.recent_matches[0]?.participants.find(p => p.bot_id !== profile.id)?.name;
    if (rivalName) {
      ctx.fillStyle = COLORS.text;
      ctx.font = 'bold 13px -apple-system, sans-serif';
      ctx.fillText('Recent Rival', rightX, startY + 60);

      ctx.fillStyle = COLORS.textMuted;
      ctx.font = '11px -apple-system, sans-serif';
      ctx.fillText(`vs ${truncateText(rivalName, 20)}`, rightX, startY + 76);
    }
  }
}

/**
 * Draw footer with platform URL
 */
function drawFooter(ctx: CanvasRenderingContext2D): void {
  const footerY = CARD_HEIGHT - 24;

  ctx.fillStyle = COLORS.textMuted;
  ctx.font = '12px -apple-system, sans-serif';
  ctx.textAlign = 'center';
  ctx.fillText('aicodebattle.com', CARD_WIDTH / 2, footerY);

  // Decorative line
  ctx.strokeStyle = COLORS.border;
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(CARD_WIDTH / 2 - 50, footerY - 10);
  ctx.lineTo(CARD_WIDTH / 2 + 50, footerY - 10);
  ctx.stroke();
}

/**
 * Draw a single stat with value and label
 */
function drawStat(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  value: string,
  label: string,
  color: string,
): void {
  ctx.fillStyle = color;
  ctx.font = 'bold 20px -apple-system, sans-serif';
  ctx.textAlign = 'center';
  ctx.fillText(value, x, y);

  ctx.fillStyle = COLORS.textMuted;
  ctx.font = '11px -apple-system, sans-serif';
  ctx.fillText(label, x, y + 16);
}

/**
 * Infer archetype from bot stats (simplified heuristic)
 */
function inferArchetype(profile: BotProfile): string {
  const wr = profile.win_rate;
  const matches = profile.matches_played;

  // High win rate with fewer matches → aggressive/hunter
  if (wr > 60 && matches < 100) return 'aggressive';
  // Moderate win rate with many matches → balanced
  if (wr >= 45 && wr <= 55 && matches > 100) return 'balanced';
  // Lower win rate → defensive/gatherer
  if (wr < 45) return 'defensive';
  // High win rate with many matches → swarm
  if (wr > 55 && matches > 100) return 'swarm';

  return 'balanced';
}

/**
 * Generate a signature move description from bot stats
 */
function generateSignature(profile: BotProfile): string {
  const wr = profile.win_rate;

  if (profile.evolved) {
    return `LLM-evolved strategy combining adaptive positioning with dynamic resource allocation. Generation ${profile.generation ?? '?'}, island ${profile.island ?? '?'}.`;
  }

  if (wr > 60) {
    return 'Dominates through aggressive positioning and efficient eliminations. Favors early confrontation over economic buildup.';
  }
  if (wr < 40) {
    return 'Focuses on defensive perimeter control and resource accumulation. Prefers long-term economic advantage over early aggression.';
  }
  return 'Balanced approach mixing territorial control with opportunistic engagements. Adapts strategy based on opponent positioning.';
}

/**
 * Helper: Draw a rounded rectangle
 */
function roundRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
  fill = true,
  stroke = false,
): void {
  ctx.beginPath();
  ctx.roundRect(x, y, w, h, r);
  if (fill) ctx.fill();
  if (stroke) ctx.stroke();
}

/**
 * Helper: Truncate text to fit width
 */
function truncateText(text: string, maxWidth: number): string {
  if (text.length <= maxWidth) return text;
  return text.substring(0, maxWidth - 3) + '...';
}

/**
 * Helper: Wrap text to fit width
 */
function wrapText(
  ctx: CanvasRenderingContext2D,
  text: string,
  x: number,
  y: number,
  maxWidth: number,
  lineHeight: number,
): void {
  const words = text.split(' ');
  let line = '';
  let currentY = y;

  for (const word of words) {
    const testLine = line + word + ' ';
    const metrics = ctx.measureText(testLine);

    if (metrics.width > maxWidth && line !== '') {
      ctx.fillText(line.trim(), x, currentY);
      line = word + ' ';
      currentY += lineHeight;
    } else {
      line = testLine;
    }
  }
  ctx.fillText(line.trim(), x, currentY);
}

/**
 * Generate a shareable bot card URL
 */
export function getBotCardURL(botId: string): string {
  return `/r2/cards/${botId}.png`;
}

/**
 * Download a bot card as PNG
 */
export async function downloadBotCard(
  profile: BotProfile,
  rank?: number,
  _rivals?: { name: string; record: string }[],
): Promise<void> {
  const blob = await renderBotCard(profile, rank, _rivals);
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${profile.name.replace(/\s+/g, '_')}_card.png`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/**
 * Render bot card to a canvas element for preview
 */
export async function renderBotCardToCanvas(
  canvas: HTMLCanvasElement,
  profile: BotProfile,
  rank?: number,
  _rivals?: { name: string; record: string }[],
): Promise<void> {
  canvas.width = CARD_WIDTH;
  canvas.height = CARD_HEIGHT;
  const ctx = canvas.getContext('2d')!;

  // Background
  ctx.fillStyle = COLORS.bg;
  ctx.fillRect(0, 0, CARD_WIDTH, CARD_HEIGHT);

  // Decorative gradient header
  const gradient = ctx.createLinearGradient(0, 0, CARD_WIDTH, 0);
  gradient.addColorStop(0, 'rgba(59, 130, 246, 0.15)');
  gradient.addColorStop(1, 'rgba(168, 85, 247, 0.15)');
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, CARD_WIDTH, 80);

  drawHeader(ctx, profile, rank);
  drawStatsBox(ctx, profile);
  drawWinRates(ctx, profile);
  drawFooter(ctx);
}
