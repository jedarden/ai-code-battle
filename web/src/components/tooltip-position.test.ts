/**
 * Unit tests for the tooltip positioning helper (§16.15)
 * Verifies adjacency, centering, non-overlap, determinism, viewport
 * overflow detection for all four placements, and the flip across the
 * anchor on overflow on both axes.
 */

import { describe, it, expect } from 'vitest';
import {
  computeTooltipPosition,
  placementOverflowsViewport,
  resolveTooltipPlacement,
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

/** Axis-aligned rectangle intersection between a tooltip rect and an anchor. */
function overlapsAnchor(
  position: { x: number; y: number },
  anchor: TooltipAnchorRect = ANCHOR,
): boolean {
  const rect = tooltipRect(position);
  return (
    rect.left < anchor.x + anchor.width &&
    rect.right > anchor.x &&
    rect.top < anchor.y + anchor.height &&
    rect.bottom > anchor.y
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

describe('resolveTooltipPlacement', () => {
  // nearTop/nearBottom sit at the horizontal interior so only the vertical
  // axis is in play; nearLeft/nearRight/midViewport sit at the vertical
  // interior so only the horizontal axis is.
  const nearTop: TooltipAnchorRect = { x: 100, y: 30, width: 24, height: 24 };
  const nearBottom: TooltipAnchorRect = { x: 100, y: 730, width: 24, height: 24 };
  const nearLeft: TooltipAnchorRect = { x: 100, y: 200, width: 24, height: 24 };
  const nearRight: TooltipAnchorRect = { x: 990, y: 200, width: 24, height: 24 };
  const midViewport: TooltipAnchorRect = { x: 500, y: 200, width: 24, height: 24 };

  /** Resolve for `anchor`, giving the placement it picked and where it lands. */
  function resolved(anchor: TooltipAnchorRect, preferred: TooltipPlacement) {
    return resolveTooltipPlacement(anchor, TOOLTIP, preferred, VIEWPORT);
  }

  it('should return the placement with the coordinates computed for it', () => {
    // The position must be the one the returned placement produces — a flip
    // that kept the preferred side's coordinates would leave the tooltip
    // off-viewport while reporting a fitting placement.
    const flipped = resolved(nearTop, 'above');
    expect(flipped.placement).toBe('below');
    expect(flipped.position).toEqual(computeTooltipPosition(nearTop, TOOLTIP, 'below'));

    const kept = resolved(midViewport, 'left');
    expect(kept.placement).toBe('left');
    expect(kept.position).toEqual(computeTooltipPosition(midViewport, TOOLTIP, 'left'));
  });

  it('should flip above to below when the tooltip crosses the viewport top', () => {
    // y = 30 - 48 - 12 = -30, past the top edge.
    expect(resolved(nearTop, 'above').placement).toBe('below');
  });

  it('should flip below to above when the tooltip crosses the viewport bottom', () => {
    // y = 730 + 24 + 12 = 766, so the bottom edge lands at 814 > 768.
    expect(resolved(nearBottom, 'below').placement).toBe('above');
  });

  it('should flip left to right when the tooltip crosses the viewport left edge', () => {
    // x = 100 - 160 - 12 = -72, past the left edge.
    expect(resolved(nearLeft, 'left').placement).toBe('right');
  });

  it('should flip right to left when the tooltip crosses the viewport right edge', () => {
    // x = 990 + 24 + 12 = 1026, so the right edge lands at 1186 > 1024.
    expect(resolved(nearRight, 'right').placement).toBe('left');
  });

  it('should keep the preferred placement for an interior anchor', () => {
    // above spans [140,188] and below [236,284], both inside 768; left spans
    // [328,488] and right [536,696], both inside 1024.
    for (const preferred of PLACEMENTS) {
      expect(resolved(midViewport, preferred).placement).toBe(preferred);
    }
  });

  it('should report the preferred placement coordinates when nothing flips', () => {
    for (const preferred of PLACEMENTS) {
      const { placement, position } = resolved(midViewport, preferred);
      expect(placement).toBe(preferred);
      expect(position).toEqual(computeTooltipPosition(midViewport, TOOLTIP, placement));
    }
  });

  it('should keep the flipped placement TOOLTIP_OFFSET_PX adjacent to the anchor', () => {
    const flippedUp = resolved(nearTop, 'above');
    expect(flippedUp.position.y).toBe(nearTop.y + nearTop.height + TOOLTIP_OFFSET_PX);

    const flippedDown = resolved(nearBottom, 'below');
    expect(flippedDown.position.y).toBe(
      nearBottom.y - TOOLTIP.height - TOOLTIP_OFFSET_PX,
    );

    const flippedRight = resolved(nearLeft, 'left');
    expect(flippedRight.position.x).toBe(
      nearLeft.x + nearLeft.width + TOOLTIP_OFFSET_PX,
    );

    const flippedLeft = resolved(nearRight, 'right');
    expect(flippedLeft.position.x).toBe(
      nearRight.x - TOOLTIP.width - TOOLTIP_OFFSET_PX,
    );
  });

  it('should keep the returned coordinates clear of the anchor, flipped or not', () => {
    for (const [anchor, preferred] of [
      [midViewport, 'above'],
      [midViewport, 'below'],
      [midViewport, 'left'],
      [midViewport, 'right'],
      [nearTop, 'above'],
      [nearBottom, 'below'],
      [nearLeft, 'left'],
      [nearRight, 'right'],
    ] as const) {
      const { position } = resolved(anchor, preferred);
      expect(overlapsAnchor(position, anchor)).toBe(false);
    }
  });

  it('should land the returned coordinates inside the viewport', () => {
    for (const [anchor, preferred] of [
      [nearTop, 'above'],
      [nearBottom, 'below'],
      [nearLeft, 'left'],
      [nearRight, 'right'],
    ] as const) {
      const { placement, position } = resolved(anchor, preferred);
      expect(placementOverflowsViewport(anchor, TOOLTIP, placement, VIEWPORT)).toBe(
        false,
      );
      const rect = tooltipRect(position);
      expect(rect.left).toBeGreaterThanOrEqual(0);
      expect(rect.top).toBeGreaterThanOrEqual(0);
      expect(rect.right).toBeLessThanOrEqual(VIEWPORT.width);
      expect(rect.bottom).toBeLessThanOrEqual(VIEWPORT.height);
    }
  });

  it('should not flip a placement that only overflows the cross axis', () => {
    // 'above' clears the top edge (y = 140) but its centered rect spans
    // [922,1082], past the 1024 right edge. Moving the tooltip to the other
    // side cannot repair horizontal overflow, so above is kept.
    expect(placementOverflowsViewport(nearRight, TOOLTIP, 'above', VIEWPORT)).toBe(
      true,
    );
    expect(resolved(nearRight, 'above').placement).toBe('above');

    // Symmetrically, 'left' clears the left edge (x = 228) but its centered
    // rect spans [-12,36], past the 0 top edge, and moving it to the right
    // side would not lift it back in.
    const highAnchor: TooltipAnchorRect = { x: 400, y: 0, width: 24, height: 24 };
    expect(placementOverflowsViewport(highAnchor, TOOLTIP, 'left', VIEWPORT)).toBe(
      true,
    );
    expect(resolved(highAnchor, 'left').placement).toBe('left');
  });

  it('should not flip when the tooltip exactly touches the viewport edge', () => {
    // above y = 60 - 48 - 12 = 0, top edge on the boundary.
    const touchingTop: TooltipAnchorRect = { x: 100, y: 60, width: 24, height: 24 };
    expect(resolved(touchingTop, 'above').placement).toBe('above');

    // below y = 684 + 24 + 12 = 720, bottom edge exactly on 768.
    const touchingBottom: TooltipAnchorRect = { x: 100, y: 684, width: 24, height: 24 };
    expect(resolved(touchingBottom, 'below').placement).toBe('below');

    // left x = 172 - 160 - 12 = 0, left edge on the boundary.
    const touchingLeft: TooltipAnchorRect = { x: 172, y: 200, width: 24, height: 24 };
    expect(resolved(touchingLeft, 'left').placement).toBe('left');

    // right x = 828 + 24 + 12 = 864, right edge exactly on 1024.
    const touchingRight: TooltipAnchorRect = { x: 828, y: 200, width: 24, height: 24 };
    expect(resolved(touchingRight, 'right').placement).toBe('right');
  });

  it('should still flip when neither side fits, taking the non-preferred side', () => {
    // A 400px tooltip cannot fit a 200px viewport on either side; the flip
    // happens anyway because the preferred side is the one that already lost.
    const tall: TooltipSize = { width: 160, height: 400 };
    const cramped: ViewportBounds = { width: 1024, height: 200 };
    expect(resolveTooltipPlacement(ANCHOR, tall, 'above', cramped).placement).toBe(
      'below',
    );
    expect(resolveTooltipPlacement(ANCHOR, tall, 'below', cramped).placement).toBe(
      'above',
    );

    // Same on the horizontal axis, with a tooltip wider than the viewport.
    const wide: TooltipSize = { width: 1200, height: 48 };
    expect(resolveTooltipPlacement(ANCHOR, wide, 'left', VIEWPORT).placement).toBe(
      'right',
    );
    expect(resolveTooltipPlacement(ANCHOR, wide, 'right', VIEWPORT).placement).toBe(
      'left',
    );
  });
});
