/**
 * Measurement helpers for the rendered-layout harness. Everything here reads
 * geometry out of the headless browser; nothing re-derives numbers from CSS
 * text — that stays the job of the jsdom-side guards in
 * src/components/skeleton.test.ts. Parity children should build their
 * skeleton-vs-live assertions on these helpers rather than on fresh
 * page.evaluate() calls, so the shape of a measurement stays comparable.
 */

import type { Page } from '@playwright/test';

/**
 * A serialized DOMRect plus the computed display. DOMRect instances do not
 * survive page.evaluate() serialization, so the fields are copied out as a
 * plain object; `display` is included because a hidden element measures all
 * zeros and parity checks need to tell "collapsed by a display rule" apart
 * from "missing from the page" (measure() throws for the latter).
 */
export interface RectSnapshot {
  x: number;
  y: number;
  width: number;
  height: number;
  top: number;
  right: number;
  bottom: number;
  left: number;
  display: string;
}

/** Measures the first element matching `selector`, in CSS pixels. */
export async function measure(page: Page, selector: string): Promise<RectSnapshot> {
  return page.evaluate((sel: string): RectSnapshot => {
    const el = document.querySelector(sel);
    if (!el) throw new Error(`measure(): no element matches "${sel}"`);
    const rect = el.getBoundingClientRect();
    return {
      x: rect.x,
      y: rect.y,
      width: rect.width,
      height: rect.height,
      top: rect.top,
      right: rect.right,
      bottom: rect.bottom,
      left: rect.left,
      display: getComputedStyle(el).display,
    };
  }, selector);
}

/** Measures every element matching `selector`, in document order. */
export async function measureAll(page: Page, selector: string): Promise<RectSnapshot[]> {
  return page.evaluate((sel: string): RectSnapshot[] => {
    return Array.from(document.querySelectorAll(sel), (el) => {
      const rect = el.getBoundingClientRect();
      return {
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height,
        top: rect.top,
        right: rect.right,
        bottom: rect.bottom,
        left: rect.left,
        display: getComputedStyle(el).display,
      };
    });
  }, selector);
}

/**
 * Reads the named computed styles off the first element matching `selector`,
 * keyed by property name exactly as handed in (longhand or shorthand, e.g.
 * "gap", "padding-left", "min-height"). Lengths come back resolved to px in
 * Chromium, so a declaration like `padding: var(--space-sm) var(--space-md)`
 * reads as "8px"/"16px" — the laid-out value, not the var() text. Parity
 * assertions use this to compare what the two rows' shared rule *resolved to*
 * against the geometry measure()/measureAll() read off the same elements.
 */
export async function readStyles(
  page: Page,
  selector: string,
  properties: string[]
): Promise<Record<string, string>> {
  return page.evaluate(
    ({ sel, props }): Record<string, string> => {
      const el = document.querySelector(sel);
      if (!el) throw new Error(`readStyles(): no element matches "${sel}"`);
      const computed = getComputedStyle(el);
      const out: Record<string, string> = {};
      for (const prop of props) out[prop] = computed.getPropertyValue(prop);
      return out;
    },
    { sel: selector, props: properties }
  );
}
