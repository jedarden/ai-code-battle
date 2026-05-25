/**
 * Unit tests for replay page error handling.
 * Tests the 404 vs generic error distinction in the URL load flow.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

declare global {
  // eslint-disable-next-line no-var
  var fetch: any;
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

    // Wait for async module loading
    await new Promise(resolve => setTimeout(resolve, 100));

    // Get the URL input and load button
    const urlInput = document.getElementById('url-input') as HTMLInputElement;
    const loadBtn = document.getElementById('load-url-btn') as HTMLButtonElement;

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

    await new Promise(resolve => setTimeout(resolve, 100));

    const urlInput = document.getElementById('url-input') as HTMLInputElement;
    const loadBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
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

    await new Promise(resolve => setTimeout(resolve, 100));

    const urlInput = document.getElementById('url-input') as HTMLInputElement;
    const loadBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
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

    await new Promise(resolve => setTimeout(resolve, 100));

    const urlInput = document.getElementById('url-input') as HTMLInputElement;
    const loadBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
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

    await new Promise(resolve => setTimeout(resolve, 100));

    let urlInput = document.getElementById('url-input') as HTMLInputElement;
    let loadBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
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

    await new Promise(resolve => setTimeout(resolve, 100));

    urlInput = document.getElementById('url-input') as HTMLInputElement;
    loadBtn = document.getElementById('load-url-btn') as HTMLButtonElement;
    urlInput.value = serverErrorUrl;
    loadBtn.click();

    await new Promise(resolve => setTimeout(resolve, 50));

    noReplayDiv = document.getElementById('no-replay') as HTMLElement;
    html = noReplayDiv.innerHTML;
    expect(html).toContain('Could not load this replay');
    expect(html).not.toContain('not available yet');
  });
});
