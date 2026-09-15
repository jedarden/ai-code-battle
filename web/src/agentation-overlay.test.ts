// Tests for the agentation feedback toolbar mount (workspace standard:
// "agentation goes on EVERY page" and "verify by mounting, never by
// grepping the tag").
//
// These pin the overlay module's contract — the mount div exists, the
// toolbar actually renders into the document (an empty mount div is the
// silent failure mode the standard warns about), and initAgentation is
// idempotent since both the app shell startup and page loaders may call it.

import { describe, it, expect, beforeEach } from 'vitest';
import { act } from '@testing-library/react';
import { initAgentation } from './agentation-overlay';

async function mount(): Promise<void> {
  await act(async () => {
    initAgentation();
  });
  // Let React commit the concurrent render (the toolbar lands via portal).
  await act(async () => {
    await new Promise(resolve => setTimeout(resolve, 0));
  });
}

beforeEach(() => {
  document.body.innerHTML = '';
});

describe('agentation overlay', () => {
  it('mounts #agentation-root on the page', async () => {
    await mount();
    expect(document.getElementById('agentation-root')).toBeInTheDocument();
  });

  it('renders the toolbar into the document — not just an empty mount div', async () => {
    await mount();
    expect(document.querySelector('[data-agentation-toolbar="true"]')).not.toBeNull();
  });

  it('is idempotent — a second call does not duplicate the mount', async () => {
    await mount();
    await mount();
    expect(document.querySelectorAll('#agentation-root')).toHaveLength(1);
    expect(document.querySelectorAll('[data-agentation-toolbar="true"]')).toHaveLength(1);
  });
});
