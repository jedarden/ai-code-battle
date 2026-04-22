// Seasonal accent color shifts — §16.18
// Subtle background hue tint that changes per season theme.
// Most users won't consciously notice; the platform just *feels* different.

export interface SeasonTheme {
  bgPrimary: string;
  accentShift: string;
}

const THEMES: Record<string, SeasonTheme> = {
  labyrinth:   { bgPrimary: '#1e1a2e', accentShift: 'hsl(270, 15%, 10%)' },
  energy_rush: { bgPrimary: '#1a2e1e', accentShift: 'hsl(140, 15%, 10%)' },
  fog_of_war:  { bgPrimary: '#1a1a3e', accentShift: 'hsl(220, 20%, 12%)' },
  colosseum:   { bgPrimary: '#2e1a1a', accentShift: 'hsl(0, 15%, 10%)' },
};

const DEFAULT_BG = '#0f172a';

/**
 * Apply a seasonal background tint based on the theme name.
 * Falls back to the default dark bg for unknown/missing themes.
 */
export function applySeasonTheme(theme: string | null | undefined): void {
  const root = document.documentElement;
  const t = theme ? THEMES[toKey(theme)] : null;

  root.style.setProperty('--bg-primary', t ? t.bgPrimary : DEFAULT_BG);
  root.style.setProperty('--season-accent', t ? t.accentShift : 'transparent');

  // Update body background to match
  document.body.style.backgroundColor = t ? t.bgPrimary : DEFAULT_BG;
}

function toKey(theme: string): string {
  return theme.toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_|_$/g, '');
}

/**
 * Return the seasonal CSS background for a given theme.
 * Used inline where CSS variables aren't available (e.g. iframe embeds).
 */
export function seasonBg(theme: string | null | undefined): string {
  if (!theme) return DEFAULT_BG;
  const t = THEMES[toKey(theme)];
  return t ? t.bgPrimary : DEFAULT_BG;
}
