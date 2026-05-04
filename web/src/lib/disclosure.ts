// Progressive feature revelation — §16.15
// Tracks which UI features a user has 'unlocked' based on engagement.
// Uses localStorage to persist XP and feature flags across sessions.

const STORAGE_KEY_XP = 'acb_viewer_xp';
const STORAGE_KEY_OVERRIDE = 'acb_disclosure_override';

// ─── Feature definitions ────────────────────────────────────────────────────────────

/**
 * All discoverable UI features with their XP threshold.
 * Features are revealed progressively as users gain engagement XP.
 */
export interface Feature {
  /** Feature key used for storage lookups */
  key: string;
  /** Display name for tooltips/notifications */
  name: string;
  /** XP level required to unlock */
  xpThreshold: number;
  /** Brief explanation shown on unlock */
  description: string;
}

export const FEATURES: Feature[] = [
  {
    key: 'event_timeline',
    name: 'Event Timeline',
    xpThreshold: 2,
    description: 'Key moments at a glance',
  },
  {
    key: 'view_mode_toggle',
    name: 'Territory View',
    xpThreshold: 5,
    description: 'See who controls the map',
  },
  {
    key: 'critical_moment_nav',
    name: 'Critical Moments',
    xpThreshold: 5,
    description: 'Jump to turning points in the match',
  },
  {
    key: 'follow_mode',
    name: 'Follow Bot',
    xpThreshold: 10,
    description: 'Camera tracks one player',
  },
  {
    key: 'fog_perspective',
    name: 'Fog of War View',
    xpThreshold: 10,
    description: 'See what each player sees',
  },
  {
    key: 'clip_export',
    name: 'Clip Export',
    xpThreshold: 10,
    description: 'Share replay moments as GIF or MP4',
  },
  {
    key: 'debug_telemetry',
    name: 'Debug Telemetry',
    xpThreshold: 20,
    description: 'View bot reasoning and targets',
  },
  {
    key: 'annotations',
    name: 'Community Annotations',
    xpThreshold: 20,
    description: 'Tactical feedback on any turn',
  },
  {
    key: 'director_mode',
    name: 'Director Mode',
    xpThreshold: 20,
    description: 'Auto-speed through uneventful turns',
  },
];

// Feature keys that unlock after specific user actions (not XP-based)
export const ACTION_FEATURES = {
  predictions: 'acb_feature_predictions',     // Unlocked after first replay watch
  sandbox: 'acb_feature_sandbox',             // Unlocked after registration/bot submission
  embed: 'acb_feature_embed',                 // Unlocked after sharing a replay
} as const;

// ─── XP tracking ─────────────────────────────────────────────────────────────────────

/**
 * Get the user's current engagement XP from localStorage.
 * Defaults to 0 for new users.
 */
export function getXP(): number {
  const stored = localStorage.getItem(STORAGE_KEY_XP);
  return stored ? Math.max(0, parseInt(stored, 10)) : 0;
}

/**
 * Set the user's XP to a specific value.
 * Use sparingly — prefer addXP() for engagement tracking.
 */
export function setXP(xp: number): void {
  localStorage.setItem(STORAGE_KEY_XP, String(Math.max(0, xp)));
}

/**
 * Increment XP by the given amount.
 * Called when user completes engagement actions (watching replays, etc.).
 */
export function addXP(amount: number): number {
  const current = getXP();
  const newXP = current + amount;
  setXP(newXP);
  return newXP;
}

/**
 * Reset XP to 0 (for testing or user preference).
 */
export function resetXP(): void {
  localStorage.removeItem(STORAGE_KEY_XP);
}

// ─── Feature revelation ─────────────────────────────────────────────────────────────

/**
 * Check if a specific XP-gated feature is revealed.
 * Returns true if the user's XP meets or exceeds the feature's threshold.
 */
export function isRevealed(featureKey: string): boolean {
  // Check override first (power user "show all" setting)
  if (isOverrideActive()) {
    return true;
  }

  const feature = FEATURES.find(f => f.key === featureKey);
  if (!feature) {
    // Unknown features default to revealed
    return true;
  }

  return getXP() >= feature.xpThreshold;
}

/**
 * Mark a feature as revealed by increasing XP to its threshold.
 * Returns the new XP level if it changed, or null if already revealed.
 */
export function reveal(featureKey: string): number | null {
  if (isRevealed(featureKey)) {
    return null;
  }

  const feature = FEATURES.find(f => f.key === featureKey);
  if (!feature) {
    return null;
  }

  return setXP(feature.xpThreshold);
}

/**
 * Get all features that should be visible at the current XP level.
 */
export function getVisibleFeatures(): Feature[] {
  const xp = getXP();
  const override = isOverrideActive();

  return FEATURES.filter(f => override || xp >= f.xpThreshold);
}

/**
 * Get features that are newly revealed since the last checkpoint.
 * Useful for showing unlock notifications.
 */
export function getNewlyRevealedFeatures(previousXP: number): Feature[] {
  const currentXP = getXP();
  return FEATURES.filter(
    f => previousXP < f.xpThreshold && currentXP >= f.xpThreshold
  );
}

// ─── Action-based features ───────────────────────────────────────────────────────────

/**
 * Check if an action-based feature (predictions, sandbox, embed) is unlocked.
 * These are stored as individual localStorage flags.
 */
export function isActionFeatureRevealed(feature: keyof typeof ACTION_FEATURES): boolean {
  const key = ACTION_FEATURES[feature];
  return localStorage.getItem(key) === 'true';
}

/**
 * Mark an action-based feature as revealed.
 */
export function revealActionFeature(feature: keyof typeof ACTION_FEATURES): void {
  localStorage.setItem(ACTION_FEATURES[feature], 'true');
}

/**
 * Reveal the 'predictions' feature after watching a replay.
 */
export function revealPredictions(): void {
  revealActionFeature('predictions');
}

/**
 * Reveal the 'sandbox' feature after bot registration/submission.
 */
export function revealSandbox(): void {
  revealActionFeature('sandbox');
}

/**
 * Reveal the 'embed' feature after sharing a replay.
 */
export function revealEmbed(): void {
  revealActionFeature('embed');
}

// ─── Power user override ────────────────────────────────────────────────────────────

/**
 * Enable 'show all controls' mode for power users.
 * When active, all features are revealed regardless of XP.
 */
export function setOverride(enabled: boolean): void {
  localStorage.setItem(STORAGE_KEY_OVERRIDE, enabled ? 'true' : 'false');
}

/**
 * Check if the override (show all controls) is active.
 */
export function isOverrideActive(): boolean {
  return localStorage.getItem(STORAGE_KEY_OVERRIDE) === 'true';
}

/**
 * Toggle the override state.
 */
export function toggleOverride(): boolean {
  const newState = !isOverrideActive();
  setOverride(newState);
  return newState;
}

// ─── Engagement tracking helpers ─────────────────────────────────────────────────────

/**
 * Record replay watch time and award XP if threshold met.
 * §16.15: Award 1 XP for watching a replay for >30 seconds.
 * Returns the new XP if awarded, or null if threshold not met.
 */
export function recordWatchTime(milliseconds: number): number | null {
  const THRESHOLD_MS = 30_000; // 30 seconds

  if (milliseconds >= THRESHOLD_MS) {
    return addXP(1);
  }

  return null;
}

/**
 * Check if the user has watched enough to qualify for engagement.
 * Used to prevent spamming XP from very short interactions.
 */
export function qualifiesForEngagement(milliseconds: number): boolean {
  return milliseconds >= 30_000;
}

// ─── Feature lookup ─────────────────────────────────────────────────────────────────

/**
 * Get a feature's metadata by key.
 */
export function getFeature(featureKey: string): Feature | undefined {
  return FEATURES.find(f => f.key === featureKey);
}

/**
 * Get all features sorted by XP threshold.
 */
export function getFeaturesSortedByThreshold(): Feature[] {
  return [...FEATURES].sort((a, b) => a.xpThreshold - b.xpThreshold);
}
