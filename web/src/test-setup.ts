/**
 * Vitest setup file for browser API mocks.
 */
import { vi } from 'vitest';

// Mock matchMedia for accessibility features in replay.ts
// Must be set up before tests import modules that use it
const matchMediaMock = vi.fn().mockImplementation((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}));
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: matchMediaMock,
});
