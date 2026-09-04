/**
 * Unit tests for replay page error handling.
 * Tests the 404 vs generic error distinction in the URL load flow.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

declare global {
  // eslint-disable-next-line no-var
  var fetch: any;
}

/**
 * Wait for an element to appear in the DOM.
 * The replay page mounts real content only after the lazy-loaded viewer module
 * resolves plus a 150ms skeleton fade-out, so tests must wait for the element
 * rather than sleeping a fixed duration.
 */
async function waitForElement(id: string, timeoutMs = 2000): Promise<HTMLElement> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    const el = document.getElementById(id);
    if (el) return el;
    if (Date.now() > deadline) throw new Error(`#${id} did not appear within ${timeoutMs}ms`);
    await new Promise(resolve => setTimeout(resolve, 25));
  }
}

describe('replay.ts error handling (URL load button)', () => {
  beforeEach(() => {
    // Ensure matchMedia is mocked before any module loads
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    // Setup DOM environment
    document.body.innerHTML = '<div id="app"></div>';
    globalThis.fetch = vi.fn();

    // Mock canvas context to avoid ReplayViewer errors
    HTMLCanvasElement.prototype.getContext = vi.fn(() => ({
      fillRect: vi.fn(),
      clearRect: vi.fn(),
      getImageData: vi.fn(),
      putImageData: vi.fn(),
      createImageData: vi.fn(),
      setTransform: vi.fn(),
      resetTransform: vi.fn(),
      drawImage: vi.fn(),
      save: vi.fn(),
      fillText: vi.fn(),
      restore: vi.fn(),
      beginPath: vi.fn(),
      moveTo: vi.fn(),
      lineTo: vi.fn(),
      closePath: vi.fn(),
      stroke: vi.fn(),
      translate: vi.fn(),
      scale: vi.fn(),
      rotate: vi.fn(),
      arc: vi.fn(),
      fill: vi.fn(),
      measureText: vi.fn(() => ({ width: 0 })),
      transform: vi.fn(),
      rect: vi.fn(),
      clip: vi.fn(),
    })) as any;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    document.body.innerHTML = '';
  });

  it('should show friendly 404 message when replay not found', async () => {
    const testUrl = 'https://example.com/replays/missing.json';

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: async () => ({}),
    } as Response);

    // Import and initialize WITHOUT URL (to enable manual loading)
    const { renderReplayPage } = await import('./replay');
    renderReplayPage({});

    // Wait for the real content to mount (lazy import + 150ms skeleton fade)
    const urlInput = (await waitForElement('url-input')) as HTMLInputElement;
    const loadBtn = (await waitForElement('load-url-btn')) as HTMLButtonElement;

    expect(urlInput).toBeTruthy();
    expect(loadBtn).toBeTruthy();

    // Enter URL and click load
    urlInput.value = testUrl;
    loadBtn.click();

    // Wait for fetch
    await new Promise(resolve => setTimeout(resolve, 50));

    const noReplayDiv = document.getElementById('no-replay') as HTMLElement;
    expect(noReplayDiv).toBeTruthy();
    const html = noReplayDiv.innerHTML;
    expect(html).toContain('not available yet');
    expect(html).toContain('may not have been uploaded');
    expect(html).toContain(testUrl);
  });

  it('should show generic error message for non-404 HTTP errors', async () => {
    const testUrl = 'https://example.com/replays/error.json';

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({}),
    } as Response);

    const { renderReplayPage } = await import('./replay');
    renderReplayPage({});

    const urlInput = (await waitForElement('url-input')) as HTMLInputElement;
    const loadBtn = (await waitForElement('load-url-btn')) as HTMLButtonElement;
    urlInput.value = testUrl;
    loadBtn.click();

    await new Promise(resolve => setTimeout(resolve, 50));

    const noReplayDiv = document.getElementById('no-replay') as HTMLElement;
    expect(noReplayDiv).toBeTruthy();
    const html = noReplayDiv.innerHTML;
    expect(html).toContain('Could not load this replay');
    expect(html).toContain('HTTP 500');
    expect(html).toContain(testUrl);
  });

  it('should show parse error for invalid JSON', async () => {
    const testUrl = 'https://example.com/replays/bad.json';

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => {
        throw new SyntaxError('Unexpected token < in JSON');
      },
      headers: new Headers(),
      redirected: false,
      statusText: 'OK',
      type: 'basic',
      url: testUrl,
      clone: () => ({} as Response),
      body: null,
      bodyUsed: false,
      arrayBuffer: async () => new ArrayBuffer(0),
      blob: async () => ({} as Blob),
      formData: async () => ({} as FormData),
      text: async () => '',
    } as unknown as Response);

    const { renderReplayPage } = await import('./replay');
    renderReplayPage({});

    const urlInput = (await waitForElement('url-input')) as HTMLInputElement;
    const loadBtn = (await waitForElement('load-url-btn')) as HTMLButtonElement;
    urlInput.value = testUrl;
    loadBtn.click();

    await new Promise(resolve => setTimeout(resolve, 50));

    const noReplayDiv = document.getElementById('no-replay') as HTMLElement;
    expect(noReplayDiv).toBeTruthy();
    const html = noReplayDiv.innerHTML;
    // Check for parse-related message
    expect(html).toMatch(/parse|json/i);
  });

  it('should escape HTML in error messages', async () => {
    const testUrl = 'https://example.com/replays/<script>alert("xss")</script>.json';

    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: async () => ({}),
    } as Response);

    const { renderReplayPage } = await import('./replay');
    renderReplayPage({});

    const urlInput = (await waitForElement('url-input')) as HTMLInputElement;
    const loadBtn = (await waitForElement('load-url-btn')) as HTMLButtonElement;
    urlInput.value = testUrl;
    loadBtn.click();

    await new Promise(resolve => setTimeout(resolve, 50));

    const noReplayDiv = document.getElementById('no-replay') as HTMLElement;
    const html = noReplayDiv.innerHTML;
    expect(html).toContain('&lt;script&gt;');
    expect(html).not.toContain('<script>');
  });

  it('should distinguish 404 from network errors', async () => {
    const notFoundUrl = 'https://example.com/replays/404.json';
    const serverErrorUrl = 'https://example.com/replays/500.json';

    // Test 404 first
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: async () => ({}),
    } as Response);

    const { renderReplayPage } = await import('./replay');
    renderReplayPage({});

    let urlInput = (await waitForElement('url-input')) as HTMLInputElement;
    let loadBtn = (await waitForElement('load-url-btn')) as HTMLButtonElement;
    urlInput.value = notFoundUrl;
    loadBtn.click();

    await new Promise(resolve => setTimeout(resolve, 50));

    let noReplayDiv = document.getElementById('no-replay') as HTMLElement;
    let html = noReplayDiv.innerHTML;
    expect(html).toContain('not available yet');
    expect(html).not.toContain('Could not load this replay');

    // Reset and test 500 error
    document.body.innerHTML = '<div id="app"></div>';
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => ({}),
    } as Response);

    renderReplayPage({});

    urlInput = (await waitForElement('url-input')) as HTMLInputElement;
    loadBtn = (await waitForElement('load-url-btn')) as HTMLButtonElement;
    urlInput.value = serverErrorUrl;
    loadBtn.click();

    await new Promise(resolve => setTimeout(resolve, 50));

    noReplayDiv = document.getElementById('no-replay') as HTMLElement;
    html = noReplayDiv.innerHTML;
    expect(html).toContain('Could not load this replay');
    expect(html).not.toContain('not available yet');
  });

  it('should render skeleton immediately and replace it when content loads', async () => {
    const { renderReplayPage } = await import('./replay');
    renderReplayPage({});

    // Skeleton mounts synchronously, before the lazy-loaded viewer resolves
    const skeleton = document.querySelector('.skeleton-page');
    expect(skeleton).toBeTruthy();

    // Skeleton mirrors the real page structure at the same DOM positions:
    // canvas wrapper, then mobile controls, then event timeline inside
    // .replay-main, with the sidebar as the layout wrapper's last child.
    const skeletonMain = skeleton!.querySelector('.replay-main');
    expect(skeletonMain!.querySelector('.canvas-wrapper')).toBeTruthy();
    expect(skeletonMain!.querySelector('.mobile-replay-controls')).toBeTruthy();
    expect(skeletonMain!.querySelector('.mobile-event-timeline')).toBeTruthy();
    expect(skeleton!.querySelector('.replay-layout > .replay-sidebar')).toBeTruthy();

    // Real content replaces the skeleton once loading completes
    await waitForElement('url-input');
    expect(document.querySelector('.skeleton-page')).toBeFalsy();
    expect(document.querySelector('.replay-page')).toBeTruthy();

    // The live page takes the same positions back, so the 150ms skeleton
    // fade-out (§16.14 zero layout shift) leaves each section where its
    // placeholder sat.
    const liveMain = document.querySelector('.replay-page .replay-main');
    expect(liveMain!.querySelector('.canvas-wrapper')).toBeTruthy();
    expect(liveMain!.querySelector('.mobile-replay-controls')).toBeTruthy();
    expect(liveMain!.querySelector('.mobile-event-timeline')).toBeTruthy();
    expect(document.querySelector('.replay-page .replay-layout > .replay-sidebar')).toBeTruthy();
  });
});
