/**
 * Tooltip Position Helper
 *
 * Pure geometry for placing a tooltip adjacent to a hovered anchor element
 * (e.g. an event-marker icon). Produces the top-left coordinates the Tooltip
 * component consumes verbatim through its `position` prop.
 *
 * This module reads nothing from the DOM: the caller measures the anchor
 * (e.g. `element.getBoundingClientRect()`) and the rendered tooltip, passes
 * both in, and gets back deterministic coordinates for the requested
 * placement. Deciding *which* placement fits the available space is the
 * caller's policy — see `positionTooltip` in event-ribbon.ts, which measures
 * the viewport and picks a side before delegating the arithmetic.
 *
 * @see §16.15 Tooltips and popovers
 */

import type { TooltipPosition } from './tooltip';

/**
 * Fixed gap in pixels between the tooltip's nearest edge and the anchor's
 * facing edge. Matches the gap used by the event ribbon's shared tooltip.
 */
export const TOOLTIP_OFFSET_PX = 12;

/**
 * Rectangle of the element the tooltip is anchored to, in the same
 * coordinate space the tooltip is positioned in. Structurally compatible
 * with `DOMRect`, so a live `getBoundingClientRect()` result can be passed
 * directly.
 */
export interface TooltipAnchorRect {
  /** Left edge of the anchor */
  x: number;
  /** Top edge of the anchor */
  y: number;
  width: number;
  height: number;
}

/** Rendered size of the tooltip, as reported by `getBoundingClientRect()`. */
export interface TooltipSize {
  width: number;
  height: number;
}

/**
 * Side of the anchor the tooltip is placed on.
 *
 * Placement is honored verbatim: `computeTooltipPosition` never flips to
 * another side and never clamps to the viewport, so identical inputs always
 * yield identical coordinates for a given placement. Callers that need
 * flip/fit behavior choose the placement themselves before calling.
 */
export type TooltipPlacement = 'above' | 'below' | 'left' | 'right';

/**
 * Compute tooltip top-left coordinates adjacent to an anchor element.
 *
 * Behavior by placement:
 * - `above` — tooltip's bottom edge sits `TOOLTIP_OFFSET_PX` above the
 *   anchor's top edge; horizontally centered on the anchor.
 * - `below` — tooltip's top edge sits `TOOLTIP_OFFSET_PX` below the
 *   anchor's bottom edge; horizontally centered on the anchor.
 * - `left` — tooltip's right edge sits `TOOLTIP_OFFSET_PX` left of the
 *   anchor's left edge; vertically centered on the anchor.
 * - `right` — tooltip's left edge sits `TOOLTIP_OFFSET_PX` right of the
 *   anchor's right edge; vertically centered on the anchor.
 *
 * In every case the tooltip rectangle is disjoint from the anchor rectangle
 * along the placement axis, so the two never overlap (the cross axis is
 * centered, which is expected to overlap — the tooltip points at the icon).
 * Coordinates may be fractional when the centered position falls on a half
 * pixel; subpixel `left`/`top` values are fine for the absolutely positioned
 * tooltip. Sizes are assumed non-negative.
 */
export function computeTooltipPosition(
  anchor: TooltipAnchorRect,
  tooltip: TooltipSize,
  placement: TooltipPlacement,
): TooltipPosition {
  switch (placement) {
    case 'above':
      return {
        x: anchor.x + anchor.width / 2 - tooltip.width / 2,
        y: anchor.y - tooltip.height - TOOLTIP_OFFSET_PX,
      };
    case 'below':
      return {
        x: anchor.x + anchor.width / 2 - tooltip.width / 2,
        y: anchor.y + anchor.height + TOOLTIP_OFFSET_PX,
      };
    case 'left':
      return {
        x: anchor.x - tooltip.width - TOOLTIP_OFFSET_PX,
        y: anchor.y + anchor.height / 2 - tooltip.height / 2,
      };
    case 'right':
      return {
        x: anchor.x + anchor.width + TOOLTIP_OFFSET_PX,
        y: anchor.y + anchor.height / 2 - tooltip.height / 2,
      };
  }
}
