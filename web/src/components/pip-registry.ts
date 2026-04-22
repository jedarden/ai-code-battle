// Lightweight registry that bridges the replay page and the router.
// The replay page registers its active viewer; the router checks before navigating away.

export interface ActiveReplay {
  matchId: string;
  canvas: HTMLCanvasElement;
  canvasWrapper: HTMLElement;
  getScoreText: () => string;
  getTurn: () => number;
  getTotalTurns: () => number;
  getIsPlaying: () => boolean;
  togglePlay: () => void;
  pause: () => void;
}

let activeReplay: ActiveReplay | null = null;

export function setActiveReplay(replay: ActiveReplay | null): void {
  activeReplay = replay;
}

export function getActiveReplay(): ActiveReplay | null {
  return activeReplay;
}
