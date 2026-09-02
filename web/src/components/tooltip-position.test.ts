/**
 * Unit tests for the tooltip positioning helper (§16.15)
 * Verifies adjacency, centering, non-overlap, and determinism for all four
 * placements.
 */

import { describe, it, expect } from 'vitest';
import {
  computeTooltipPosition,
  TOOLTIP_OFFSET_PX,
  type TooltipAnchorRect,
  type TooltipPlacement,
  type TooltipSize,
} from './tooltip-position';

const ANCHOR: TooltipAnchorRect = { x: 100, y: 200, width: 24, height: 24 };
const TOOLTIP: TooltipSize = { width: 160, height: 48 };
const PLACEMENTS: TooltipPlacement[] = ['above', 'below', 'left', 'right'];

/** Tooltip rectangle implied by a computed position and the tooltip size. */
function tooltipRect(position: { x: number; y: number }) {
  return {
    left: position.x,
    top: position.y,
    right: position.x + TOOLTIP.width,
    bottom: position.y + TOOLTIP.height,
  };
}

/** Axis-aligned rectangle intersection with the anchor. */
function overlapsAnchor(position: { x: number; y: number }): boolean {
  const rect = tooltipRect(position);
  return (
    rect.left < ANCHOR.x + ANCHOR.width &&
    rect.right > ANCHOR.x &&
    rect.top < ANCHOR.y + ANCHOR.height &&
    rect.bottom > ANCHOR.y
  );
}

describe('computeTooltipPosition', () => {
  describe('above', () => {
    it('should place the tooltip bottom edge TOOLTIP_OFFSET_PX above the anchor top', () => {
      const { y } = computeTooltipPosition(ANCHOR, TOOLTIP, 'above');
      expect(y).toBe(ANCHOR.y - TOOLTIP.height - TOOLTIP_OFFSET_PX);
    });

    it('should center horizontally on the anchor', () => {
      const { x } = computeTooltipPosition(ANCHOR, TOOLTIP, 'above');
      expect(x).toBe(ANCHOR.x + ANCHOR.width / 2 - TOOLTIP.width / 2);
    });
  });

  describe('below', () => {
    it('should place the tooltip top edge TOOLTIP_OFFSET_PX below the anchor bottom', () => {
      const { y } = computeTooltipPosition(ANCHOR, TOOLTIP, 'below');
      expect(y).toBe(ANCHOR.y + ANCHOR.height + TOOLTIP_OFFSET_PX);
    });

    it('should center horizontally on the anchor', () => {
      const { x } = computeTooltipPosition(ANCHOR, TOOLTIP, 'below');
      expect(x).toBe(ANCHOR.x + ANCHOR.width / 2 - TOOLTIP.width / 2);
    });
  });

  describe('left', () => {
    it('should place the tooltip right edge TOOLTIP_OFFSET_PX left of the anchor left', () => {
      const { x } = computeTooltipPosition(ANCHOR, TOOLTIP, 'left');
      expect(x).toBe(ANCHOR.x - TOOLTIP.width - TOOLTIP_OFFSET_PX);
    });

    it('should center vertically on the anchor', () => {
      const { y } = computeTooltipPosition(ANCHOR, TOOLTIP, 'left');
      expect(y).toBe(ANCHOR.y + ANCHOR.height / 2 - TOOLTIP.height / 2);
    });
  });

  describe('right', () => {
    it('should place the tooltip left edge TOOLTIP_OFFSET_PX right of the anchor right', () => {
      const { x } = computeTooltipPosition(ANCHOR, TOOLTIP, 'right');
      expect(x).toBe(ANCHOR.x + ANCHOR.width + TOOLTIP_OFFSET_PX);
    });

    it('should center vertically on the anchor', () => {
      const { y } = computeTooltipPosition(ANCHOR, TOOLTIP, 'right');
      expect(y).toBe(ANCHOR.y + ANCHOR.height / 2 - TOOLTIP.height / 2);
    });
  });

  describe('all placements', () => {
    it('should never overlap the anchor rect', () => {
      for (const placement of PLACEMENTS) {
        const position = computeTooltipPosition(ANCHOR, TOOLTIP, placement);
        expect(overlapsAnchor(position)).toBe(false);
      }
    });

    it('should remain adjacent to the anchor across a zero-size anchor', () => {
      const point: TooltipAnchorRect = { x: 50, y: 50, width: 0, height: 0 };
      for (const placement of PLACEMENTS) {
        const position = computeTooltipPosition(point, TOOLTIP, placement);
        const rect = tooltipRect(position);
        // Every placement keeps at least TOOLTIP_OFFSET_PX of clearance
        // along its axis, so the tooltip never covers the anchor point.
        const clearance = Math.min(
          Math.abs(rect.left - point.x),
          Math.abs(rect.right - point.x),
          Math.abs(rect.top - point.y),
          Math.abs(rect.bottom - point.y),
        );
        expect(clearance).toBeGreaterThanOrEqual(TOOLTIP_OFFSET_PX);
      }
    });

    it('should be deterministic for identical inputs', () => {
      for (const placement of PLACEMENTS) {
        expect(computeTooltipPosition(ANCHOR, TOOLTIP, placement)).toEqual(
          computeTooltipPosition(ANCHOR, TOOLTIP, placement),
        );
      }
    });

    it('should accept a DOMRect-shaped anchor without alteration', () => {
      // getBoundingClientRect() returns extra edges alongside x/y/width/height;
      // the helper must use only the structural fields it declares.
      const domRect = {
        x: ANCHOR.x,
        y: ANCHOR.y,
        width: ANCHOR.width,
        height: ANCHOR.height,
        top: ANCHOR.y,
        left: ANCHOR.x,
        right: ANCHOR.x + ANCHOR.width,
        bottom: ANCHOR.y + ANCHOR.height,
        toJSON: () => ({}),
      };
      expect(computeTooltipPosition(domRect, TOOLTIP, 'above')).toEqual(
        computeTooltipPosition(ANCHOR, TOOLTIP, 'above'),
      );
    });
  });
});
