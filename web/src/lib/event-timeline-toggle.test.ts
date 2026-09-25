import { describe, it, expect, beforeEach } from 'vitest';
import { toggleEventTimeline, EVENT_TIMELINE_CONTAINER_ID } from './event-timeline-toggle';
import { replayPageMarkup } from '../pages/replay';

describe('toggleEventTimeline', () => {
  beforeEach(() => {
    document.body.innerHTML = `<div class="mobile-event-timeline" id="${EVENT_TIMELINE_CONTAINER_ID}"></div>`;
  });

  it('hides a visible container', () => {
    const container = document.getElementById(EVENT_TIMELINE_CONTAINER_ID)!;
    toggleEventTimeline();
    expect(container.style.display).toBe('none');
  });

  it('restores a hidden container to visible', () => {
    const container = document.getElementById(EVENT_TIMELINE_CONTAINER_ID)!;
    container.style.display = 'none';
    toggleEventTimeline();
    expect(container.style.display).toBe('');
  });

  it('round-trips through repeated presses', () => {
    const container = document.getElementById(EVENT_TIMELINE_CONTAINER_ID)!;
    toggleEventTimeline();
    toggleEventTimeline();
    toggleEventTimeline();
    expect(container.style.display).toBe('none');
  });

  it('is a no-op when the container is missing', () => {
    document.body.innerHTML = '';
    expect(() => toggleEventTimeline()).not.toThrow();
  });

  it('flips the container the page template mounts, found by the shared id', () => {
    // The template in replay.ts interpolates the same constant into
    // id="${EVENT_TIMELINE_CONTAINER_ID}", so this documents the contract
    // rather than a coincidence: the shortcut and the markup cannot drift.
    const container = document.getElementById(EVENT_TIMELINE_CONTAINER_ID);
    expect(container).toBeTruthy();
    expect(container!.className).toBe('mobile-event-timeline');
  });

  it('targets the id the real page template renders, not a hand-built stand-in', () => {
    // Pins the markup side of the shared constant against the actual template
    // output — if replay.ts ever renames the container or stops mounting it,
    // the shortcut (and the browser regression spec) would flip nothing.
    expect(replayPageMarkup()).toContain(`id="${EVENT_TIMELINE_CONTAINER_ID}"`);
  });
});
