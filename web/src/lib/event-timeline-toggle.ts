// Event timeline toggle — the replay page's KeyE shortcut primitive.
//
// The ribbon is the replay page's only event timeline
// (docs/event-ribbon-structure.md), so the shortcut's whole job is flipping
// its container between visible and display:none. This lives in lib/ rather
// than inline in replay.ts's keydown switch so the browser regression spec
// (web/layout-tests/event-ribbon-regression.spec.ts) can run the real toggle
// in Chromium — with the container id coming from the same constant the page
// template mounts — instead of asserting against a copy of both.

/** The container replay.ts mounts the ribbon into, and the E shortcut flips. */
export const EVENT_TIMELINE_CONTAINER_ID = 'mobile-timeline';

/**
 * Toggle the event timeline container's visibility: hidden (display:none)
 * becomes visible and visible becomes hidden, exactly as replay.ts's KeyE
 * case has always done. A missing container is a no-op, matching the guard
 * the inline version carried.
 */
export function toggleEventTimeline(doc: Document = document): void {
  const container = doc.getElementById(EVENT_TIMELINE_CONTAINER_ID);
  if (!container) return;
  const isHidden = container.style.display === 'none';
  container.style.display = isHidden ? '' : 'none';
}
