/**
 * Unit tests for the tooltip positioning helper (§16.15)
 * Verifies adjacency, centering, non-overlap, determinism, viewport
 * overflow detection for all four placements, the flip across the
 * anchor on overflow on both axes, and the clamping that keeps the
 * tooltip inside the viewport when neither placement fits.
 */

import { describe, it, expect } from 'vitest';
import {
  clampTooltipPosition,
  computeTooltipPosition,
  placementCrossesFacingEdge,
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
function tooltipRect(position: { x: number; y: number }, size: TooltipSize = TOOLTIP) {
  return {
    left: position.x,
    top: position.y,
    right: position.x + size.width,
    bottom: position.y + size.height,
  };
}

/** Axis-aligned rectangle intersection between a tooltip rect and an anchor. */
function overlapsAnchor(
  position: { x: number; y: number },
  anchor: TooltipAnchorRect = ANCHOR,
  size: TooltipSize = TOOLTIP,
): boolean {
  const rect = tooltipRect(position, size);
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

describe('placementCrossesFacingEdge', () => {
  it('should report no crossing for every placement of an interior anchor', () => {
    // Interior on all four axes, which the fixture ANCHOR is not — it has less
    // room to its left than the tooltip needs, so 'left' crosses there.
    const centered: TooltipAnchorRect = { x: 400, y: 300, width: 24, height: 24 };
    for (const placement of PLACEMENTS) {
      expect(placementCrossesFacingEdge(centered, TOOLTIP, placement, VIEWPORT)).toBe(
        false,
      );
    }
  });

  it('should report the crossing each placement flips on', () => {
    // Same anchors as the flip tests on resolveTooltipPlacement: each one has
    // room on the far side of its axis and none on the side it faces.
    const nearTop: TooltipAnchorRect = { x: 100, y: 30, width: 24, height: 24 };
    const nearBottom: TooltipAnchorRect = { x: 100, y: 730, width: 24, height: 24 };
    const nearLeft: TooltipAnchorRect = { x: 8, y: 200, width: 24, height: 24 };
    const nearRight: TooltipAnchorRect = { x: 990, y: 200, width: 24, height: 24 };

    expect(placementCrossesFacingEdge(nearTop, TOOLTIP, 'above', VIEWPORT)).toBe(true);
    expect(placementCrossesFacingEdge(nearBottom, TOOLTIP, 'below', VIEWPORT)).toBe(true);
    expect(placementCrossesFacingEdge(nearLeft, TOOLTIP, 'left', VIEWPORT)).toBe(true);
    expect(placementCrossesFacingEdge(nearRight, TOOLTIP, 'right', VIEWPORT)).toBe(true);
  });

  it('should not count a rectangle that exactly touches the facing edge', () => {
    // x = 828 + 24 + 12 = 864, so the right edge lands exactly on 1024 — the
    // same boundary placementOverflowsViewport accepts.
    const touching: TooltipAnchorRect = { x: 828, y: 200, width: 24, height: 24 };
    expect(placementCrossesFacingEdge(touching, TOOLTIP, 'right', VIEWPORT)).toBe(false);
  });

  it('should ignore the cross axis, unlike placementOverflowsViewport', () => {
    // The anchor that makes placementOverflowsViewport report 'above' as
    // overflowing (centered rect [932,1092] spills past the right edge) still
    // clears the top edge it faces — and that spill is clamping's problem, not
    // a reason to move the tooltip, which is exactly what lets a caller pick
    // this side in a viewport too narrow for any side to center cleanly.
    const nearRight: TooltipAnchorRect = { x: 1000, y: 200, width: 24, height: 24 };
    expect(placementOverflowsViewport(nearRight, TOOLTIP, 'above', VIEWPORT)).toBe(true);
    expect(placementCrossesFacingEdge(nearRight, TOOLTIP, 'above', VIEWPORT)).toBe(false);
  });

  it('should be the exact predicate resolveTooltipPlacement flips on', () => {
    // Whatever the predicate reports as crossing is the placement the resolver
    // refuses to keep, for every placement and anchor — the two cannot drift.
    const anchors: TooltipAnchorRect[] = [
      ANCHOR,
      { x: 0, y: 0, width: 24, height: 24 },
      { x: 1000, y: 740, width: 24, height: 24 },
      { x: 8, y: 30, width: 24, height: 24 },
    ];
    for (const anchor of anchors) {
      for (const placement of PLACEMENTS) {
        const crosses = placementCrossesFacingEdge(anchor, TOOLTIP, placement, VIEWPORT);
        const { placement: resolved } = resolveTooltipPlacement(
          anchor,
          TOOLTIP,
          placement,
          VIEWPORT,
        );
        expect(resolved === placement).toBe(!crosses);
      }
    }
  });
});

describe('clampTooltipPosition', () => {
  it('should return a position that already fits unchanged', () => {
    // Rect [32,192] x [140,188] sits fully inside 1024x768.
    const inside = { x: 32, y: 140 };
    expect(clampTooltipPosition(inside, TOOLTIP, VIEWPORT)).toEqual(inside);
  });

  it('should pull a negative x back to the viewport left edge', () => {
    // -72 is exactly the 'left' placement for ANCHOR: 100 - 160 - 12.
    const clamped = clampTooltipPosition({ x: -72, y: 200 }, TOOLTIP, VIEWPORT);
    expect(clamped.x).toBe(0);
    expect(clamped.y).toBe(200);
  });

  it('should pull a right overflow back to viewport.width - tooltip.width', () => {
    // 932 + 160 = 1092, past the 1024 right edge by 68.
    const clamped = clampTooltipPosition({ x: 932, y: 200 }, TOOLTIP, VIEWPORT);
    expect(clamped.x).toBe(VIEWPORT.width - TOOLTIP.width);
    expect(clamped.y).toBe(200);
  });

  it('should pull a negative y back to the viewport top edge', () => {
    // -30 is exactly the 'above' placement for the nearTop anchor: 30 - 48 - 12.
    const clamped = clampTooltipPosition({ x: 200, y: -30 }, TOOLTIP, VIEWPORT);
    expect(clamped.y).toBe(0);
    expect(clamped.x).toBe(200);
  });

  it('should pull a bottom overflow back to viewport.height - tooltip.height', () => {
    // 766 + 48 = 814, past the 768 bottom edge by 46.
    const clamped = clampTooltipPosition({ x: 200, y: 766 }, TOOLTIP, VIEWPORT);
    expect(clamped.y).toBe(VIEWPORT.height - TOOLTIP.height);
    expect(clamped.x).toBe(200);
  });

  it('should clamp each axis independently', () => {
    // Both axes overflow here, and neither clamp may disturb the other.
    const clamped = clampTooltipPosition({ x: 932, y: -30 }, TOOLTIP, VIEWPORT);
    expect(clamped).toEqual({ x: VIEWPORT.width - TOOLTIP.width, y: 0 });
  });

  it('should leave a rectangle that exactly touches the viewport edges', () => {
    // Rect [0,160] x [720,768]: top-left on the origin, bottom edge on 768.
    const touching = { x: 0, y: VIEWPORT.height - TOOLTIP.height };
    expect(clampTooltipPosition(touching, TOOLTIP, VIEWPORT)).toEqual(touching);
  });

  it('should pin x to 0 when the tooltip is wider than the viewport', () => {
    // The clamp range [0, -176] is empty, so 0 wins and the tooltip
    // necessarily crosses the right edge.
    const wide: TooltipSize = { width: 1200, height: 48 };
    const clamped = clampTooltipPosition({ x: 136, y: 200 }, wide, VIEWPORT);
    expect(clamped.x).toBe(0);
    expect(clamped.y).toBe(200);
  });

  it('should pin y to 0 when the tooltip is taller than the viewport', () => {
    const tall: TooltipSize = { width: 160, height: 400 };
    const cramped: ViewportBounds = { width: 1024, height: 200 };
    const clamped = clampTooltipPosition({ x: 32, y: 236 }, tall, cramped);
    expect(clamped.y).toBe(0);
    expect(clamped.x).toBe(32);
  });

  it('should land the clamped rectangle inside the viewport', () => {
    for (const position of [
      { x: -72, y: 200 },
      { x: 932, y: -30 },
      { x: 500, y: 766 },
    ]) {
      const rect = tooltipRect(clampTooltipPosition(position, TOOLTIP, VIEWPORT));
      expect(rect.left).toBeGreaterThanOrEqual(0);
      expect(rect.top).toBeGreaterThanOrEqual(0);
      expect(rect.right).toBeLessThanOrEqual(VIEWPORT.width);
      expect(rect.bottom).toBeLessThanOrEqual(VIEWPORT.height);
    }
  });

  it('should set the clamped tooltip down on top of the anchor when necessary', () => {
    // A 300px tooltip on a 320px viewport leaves 20px beside an icon whose
    // left edge is at 40, so clamping drags the rectangle over the icon.
    // Overlap is the accepted cost of keeping the tooltip readable.
    const narrow: ViewportBounds = { width: 320, height: 480 };
    const wide: TooltipSize = { width: 300, height: 48 };
    const icon: TooltipAnchorRect = { x: 40, y: 100, width: 24, height: 24 };
    const clamped = clampTooltipPosition({ x: 76, y: 88 }, wide, narrow);

    expect(clamped.x).toBe(narrow.width - wide.width);
    const overlaps =
      clamped.x < icon.x + icon.width &&
      clamped.x + wide.width > icon.x &&
      clamped.y < icon.y + icon.height &&
      clamped.y + wide.height > icon.y;
    expect(overlaps).toBe(true);
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

  describe('minimum viewport width', () => {
    // A phone-width viewport beside an icon near its left edge: the 300px
    // tooltip leaves 20px of room to the icon's right and none to its left,
    // so neither horizontal placement fits and only clamping is left.
    const NARROW: ViewportBounds = { width: 320, height: 480 };
    const wideTooltip: TooltipSize = { width: 300, height: 48 };
    const leftIcon: TooltipAnchorRect = { x: 40, y: 100, width: 24, height: 24 };

    /** Resolve `preferred` for the narrow viewport and the left-edge icon. */
    function resolvedNarrow(preferred: TooltipPlacement) {
      return resolveTooltipPlacement(leftIcon, wideTooltip, preferred, NARROW);
    }

    it('should clamp both horizontal placements inside the viewport', () => {
      // 'right' lands at x = 40 + 24 + 12 = 76, crossing the 320 right edge,
      // so it flips to 'left' at x = 40 - 300 - 12 = -272, which clamps to 0.
      const flipped = resolvedNarrow('right');
      expect(flipped.placement).toBe('left');
      expect(flipped.position).toEqual({ x: 0, y: 88 });

      // 'left' crosses the left edge immediately, flips to 'right' at x = 76,
      // and that clamps to the 20px the viewport can give it.
      const clamped = resolvedNarrow('left');
      expect(clamped.placement).toBe('right');
      expect(clamped.position).toEqual({ x: NARROW.width - wideTooltip.width, y: 88 });
    });

    it('should keep the clamped rectangle entirely within the viewport', () => {
      for (const preferred of ['above', 'below', 'left', 'right'] as const) {
        const { position } = resolvedNarrow(preferred);
        const rect = tooltipRect(position, wideTooltip);
        expect(rect.left).toBeGreaterThanOrEqual(0);
        expect(rect.top).toBeGreaterThanOrEqual(0);
        expect(rect.right).toBeLessThanOrEqual(NARROW.width);
        expect(rect.bottom).toBeLessThanOrEqual(NARROW.height);
      }
    });

    it('should overlap the anchor rather than leave the viewport', () => {
      // Both horizontal placements are clamped over the icon: the rectangle
      // spanning [0,300] or [20,320] covers the icon's [40,64]. Overlap is
      // the accepted cost — outside the viewport the tooltip is unreadable.
      for (const preferred of ['left', 'right'] as const) {
        const { position } = resolvedNarrow(preferred);
        expect(overlapsAnchor(position, leftIcon, wideTooltip)).toBe(true);
      }

      // The vertical placements clamp horizontally too (their centered x is
      // 40 + 12 - 150 = -98) but land clear of the icon vertically, so
      // clamping alone does not imply overlap.
      expect(overlapsAnchor(resolvedNarrow('above').position, leftIcon, wideTooltip)).toBe(
        false,
      );
    });

    it('should report overflow for a placement whose clamped coordinates fit', () => {
      // The predicate evaluates the placement's own coordinates, not the
      // resolver's clamped ones, so it still says 'left' does not fit even
      // though the resolver returns in-viewport coordinates for it.
      expect(placementOverflowsViewport(leftIcon, wideTooltip, 'left', NARROW)).toBe(
        true,
      );
      expect(resolvedNarrow('left').placement).toBe('right');
    });

    it('should clamp a kept placement whose centered cross axis overflows', () => {
      // nearRight 'above' clears the top edge (y = 140) and keeps its
      // placement, but its centered x = 932 crosses the 1024 right edge. The
      // flip cannot repair that, so the clamp pulls x back instead.
      const { placement, position } = resolved(nearRight, 'above');
      expect(placement).toBe('above');
      expect(position.x).toBe(VIEWPORT.width - TOOLTIP.width);
      expect(position.y).toBe(140);
      // The clamp slides the tooltip sideways along the cross axis only, so
      // it stays TOOLTIP_OFFSET_PX clear of the anchor on the placement axis
      // — overlap here needs a clamp on the axis the tooltip is placed on,
      // as in the narrow-viewport cases above.
      expect(overlapsAnchor(position, nearRight)).toBe(false);
    });

    it('should clamp the vertical axis when neither above nor below fits', () => {
      // A 120px tooltip on a 200px-tall viewport beside an anchor whose
      // bottom edge is at 114: 'above' needs 132px of headroom, 'below' 132px
      // of floor, and neither is there.
      const short: ViewportBounds = { width: 1024, height: 200 };
      const midTooltip: TooltipSize = { width: 160, height: 120 };
      const midAnchor: TooltipAnchorRect = { x: 100, y: 90, width: 24, height: 24 };

      // 'above' flips to 'below' at y = 90 + 24 + 12 = 126, clamped to the
      // 80px the viewport can give it.
      const flipped = resolveTooltipPlacement(midAnchor, midTooltip, 'above', short);
      expect(flipped.placement).toBe('below');
      expect(flipped.position.y).toBe(short.height - midTooltip.height);
      expect(flipped.position.x).toBe(32);

      // 'below' flips to 'above' at y = 90 - 120 - 12 = -42, clamped to 0.
      const clamped = resolveTooltipPlacement(midAnchor, midTooltip, 'below', short);
      expect(clamped.placement).toBe('above');
      expect(clamped.position.y).toBe(0);
      expect(overlapsAnchor(clamped.position, midAnchor, midTooltip)).toBe(true);
    });

    it('should pin a tooltip larger than the viewport to the viewport origin', () => {
      // Neither axis can be pulled inside a viewport smaller than the
      // tooltip; the clamp pins the leading edges to 0 and the far edges
      // necessarily cross.
      const wide: TooltipSize = { width: 1200, height: 48 };
      const wideResult = resolveTooltipPlacement(ANCHOR, wide, 'left', VIEWPORT);
      expect(wideResult.placement).toBe('right');
      expect(wideResult.position.x).toBe(0);

      const tall: TooltipSize = { width: 160, height: 400 };
      const cramped: ViewportBounds = { width: 1024, height: 200 };
      const tallResult = resolveTooltipPlacement(ANCHOR, tall, 'above', cramped);
      expect(tallResult.placement).toBe('below');
      expect(tallResult.position.y).toBe(0);
    });

    it('should leave the coordinates untouched when the placement fits', () => {
      // Clamping only ever moves a position that already overflowed, so the
      // interior anchors and the repaired flips keep the exact coordinates
      // computeTooltipPosition produces — adjacency is untouched.
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
        const { placement, position } = resolved(anchor, preferred);
        expect(position).toEqual(computeTooltipPosition(anchor, TOOLTIP, placement));
        expect(overlapsAnchor(position, anchor)).toBe(false);
      }
    });
  });
});
