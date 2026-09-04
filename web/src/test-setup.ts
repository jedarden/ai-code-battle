/**
 * Vitest setup file for browser API mocks.
 */
import { vi } from 'vitest';
// Arms the jest-dom matchers (toBeInTheDocument / toHaveClass / toHaveStyle)
// for every component test — without this import they are undefined at
// runtime, so any suite using them fails to even collect.
import '@testing-library/jest-dom';

// Ensure jsdom DOM is properly initialized
if (typeof document !== 'undefined' && !document.body) {
  document.body = document.createElement('body');
}

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
