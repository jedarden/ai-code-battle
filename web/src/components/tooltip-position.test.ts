/**
 * Unit tests for the tooltip positioning helper (§16.15)
 * Verifies adjacency, centering, non-overlap, determinism, and viewport
 * overflow detection for all four placements.
 */

import { describe, it, expect } from 'vitest';
import {
  computeTooltipPosition,
  placementOverflowsViewport,
  TOOLTIP_OFFSET_PX,
  type TooltipAnchorRect,
  type TooltipPlacement,
  type TooltipSize,
  type ViewportBounds,
} from './tooltip-position';

const ANCHOR: TooltipAnchorRect = { x: 100, y: 200, width: 24, height: 24 };
const TOOLTIP: TooltipSize = { width: 160, height: 48 };
const PLACEMENTS: TooltipPlacement[] = ['above', 'below', 'left', 'right'];
const VIEWPORT: ViewportBounds = { width: 1024, height: 768 };

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

describe('placementOverflowsViewport', () => {
  describe('above', () => {
    it('should report no overflow when the tooltip fits above the anchor', () => {
      // Rect [32,192] x [140,188] sits fully inside 1024x768.
      expect(placementOverflowsViewport(ANCHOR, TOOLTIP, 'above', VIEWPORT)).toBe(
        false,
      );
    });

    it('should report overflow when the tooltip crosses the top edge', () => {
      const nearTop: TooltipAnchorRect = { x: 100, y: 30, width: 24, height: 24 };
      // y = 30 - 48 - 12 = -30, so the tooltip's top edge crosses y=0.
      expect(placementOverflowsViewport(nearTop, TOOLTIP, 'above', VIEWPORT)).toBe(
        true,
      );
    });
  });

  describe('below', () => {
    it('should report no overflow when the tooltip fits below the anchor', () => {
      // Rect [32,192] x [236,284] sits fully inside 1024x768.
      expect(placementOverflowsViewport(ANCHOR, TOOLTIP, 'below', VIEWPORT)).toBe(
        false,
      );
    });

    it('should report overflow when the tooltip crosses the bottom edge', () => {
      const nearBottom: TooltipAnchorRect = {
        x: 100,
        y: 730,
        width: 24,
        height: 24,
      };
      // y = 730 + 24 + 12 = 766, so the bottom edge lands at 814 > 768.
      expect(
        placementOverflowsViewport(nearBottom, TOOLTIP, 'below', VIEWPORT),
      ).toBe(true);
    });
  });

  describe('left', () => {
    it('should report no overflow when the tooltip fits left of the anchor', () => {
      // x = 500 - 160 - 12 = 328, so the rect [328,488] clears the left edge.
      const midViewport: TooltipAnchorRect = {
        x: 500,
        y: 200,
        width: 24,
        height: 24,
      };
      expect(
        placementOverflowsViewport(midViewport, TOOLTIP, 'left', VIEWPORT),
      ).toBe(false);
    });

    it('should report overflow when the tooltip crosses the left edge', () => {
      // x = 100 - 160 - 12 = -72, so the tooltip's left edge crosses x=0.
      expect(placementOverflowsViewport(ANCHOR, TOOLTIP, 'left', VIEWPORT)).toBe(
        true,
      );
    });
  });

  describe('right', () => {
    it('should report no overflow when the tooltip fits right of the anchor', () => {
      // x = 100 + 24 + 12 = 136, so the rect [136,296] clears the right edge.
      expect(placementOverflowsViewport(ANCHOR, TOOLTIP, 'right', VIEWPORT)).toBe(
        false,
      );
    });

    it('should report overflow when the tooltip crosses the right edge', () => {
      const nearRight: TooltipAnchorRect = {
        x: 990,
        y: 200,
        width: 24,
        height: 24,
      };
      // x = 990 + 24 + 12 = 1026, so the right edge lands at 1186 > 1024.
      expect(placementOverflowsViewport(nearRight, TOOLTIP, 'right', VIEWPORT)).toBe(
        true,
      );
    });
  });

  describe('edge handling', () => {
    it('should not count a rectangle that exactly touches the viewport edge', () => {
      // x = 828 + 24 + 12 = 864, so the right edge lands exactly on 1024.
      const touching: TooltipAnchorRect = { x: 828, y: 200, width: 24, height: 24 };
      expect(placementOverflowsViewport(touching, TOOLTIP, 'right', VIEWPORT)).toBe(
        false,
      );
    });

    it('should check the cross axis even when the placement axis fits', () => {
      // 'above' clears the top edge (y = 140), but the centered rect spans
      // [932,1092] horizontally, past the 1024 right edge.
      const nearRight: TooltipAnchorRect = { x: 1000, y: 200, width: 24, height: 24 };
      expect(placementOverflowsViewport(nearRight, TOOLTIP, 'above', VIEWPORT)).toBe(
        true,
      );
    });
  });
});
