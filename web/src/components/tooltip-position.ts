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
 * caller's policy — the helpers here only answer narrow questions about a
 * placement (`placementOverflowsViewport`) or flip the choice across the
 * anchor when it overflows (`resolveTooltipPlacement`, which answers with the
 * effective placement and the coordinates to render for it, clamped to the
 * viewport when neither side of the anchor fits); see `positionTooltip` in
 * event-ribbon.ts for a caller that measures the viewport and picks a side.
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
 * flip behavior resolve the placement first — see
 * `resolveTooltipPlacement` and `placementOverflowsViewport` for detecting
 * an unfit placement.
 */
export type TooltipPlacement = 'above' | 'below' | 'left' | 'right';

/**
 * Viewport the tooltip must fit inside, in the same coordinate space as the
 * anchor rect and anchored at its top-left corner, so `window.innerWidth`
 * and `window.innerHeight` can be passed directly.
 */
export interface ViewportBounds {
  width: number;
  height: number;
}

/**
 * Report whether the tooltip rectangle implied by `placement` would extend
 * past any viewport edge.
 *
 * Evaluates exactly the coordinates `computeTooltipPosition` produces for the
 * same inputs, so a placement that reports `false` here is guaranteed to
 * render fully inside the viewport. A `true` means the placement alone does
 * not fit — `resolveTooltipPlacement` may still return coordinates for it
 * that do, by clamping them. A rectangle that merely touches an edge
 * (e.g. its right edge landing on `viewport.width`) does not count as
 * overflow — only a positive crossing does. All four edges are checked for
 * every placement, since the cross axis is centered and can overflow too.
 * Pure arithmetic; no DOM reads.
 */
export function placementOverflowsViewport(
  anchor: TooltipAnchorRect,
  tooltip: TooltipSize,
  placement: TooltipPlacement,
  viewport: ViewportBounds,
): boolean {
  const position = computeTooltipPosition(anchor, tooltip, placement);
  return (
    position.x < 0 ||
    position.y < 0 ||
    position.x + tooltip.width > viewport.width ||
    position.y + tooltip.height > viewport.height
  );
}

/**
 * The placement the resolver settled on, with the top-left coordinates
 * computed for it. `position` is always the one `computeTooltipPosition`
 * produces for the returned `placement`, so a caller that consumes both can
 * position the tooltip and its arrow without recomputing anything.
 */
export interface ResolvedTooltipPlacement {
  /** Effective placement — `preferred` unless it flipped across the anchor. */
  placement: TooltipPlacement;
  /**
   * Top-left coordinates for `placement`, adjacent to the anchor and clamped
   * to the viewport when the placement cannot fit without overflow. Exactly
   * `computeTooltipPosition`'s output unless that clamping moved it.
   */
  position: TooltipPosition;
}

/** The placement each placement flips to across the anchor. */
const FLIPPED: Record<TooltipPlacement, TooltipPlacement> = {
  above: 'below',
  below: 'above',
  left: 'right',
  right: 'left',
};

/**
 * Resolve the placement to use for `preferred` and the coordinates to render
 * it at, flipping the placement across the anchor when the preferred side
 * does not fit the viewport and clamping the coordinates to the viewport when
 * neither side does.
 *
 * `above` that would push the tooltip past the viewport top becomes `below`,
 * `below` that would push it past the viewport bottom becomes `above`, `left`
 * that would push it past the viewport left edge becomes `right`, and `right`
 * that would push it past the viewport right edge becomes `left`. An interior
 * anchor — one whose preferred side fits — keeps its preferred placement, so
 * nothing moves unless something actually overflows.
 *
 * Only the edge the preferred side faces decides the flip. That is
 * `placementOverflowsViewport` narrowed to the one edge a flip can repair: a
 * placement's other edges (its far edge on the same axis, and the centered
 * cross axis) are identical for the placement and its flip, so overflowing
 * them is not a reason to move the tooltip — that needs clamping instead.
 * Both sides can still lose, when the tooltip is larger than the viewport on
 * the placement axis; the flip then happens anyway, because the preferred
 * side is the one that already lost.
 *
 * The returned `position` is computed for the returned `placement` and then
 * passed through `clampTooltipPosition`, so it renders entirely inside the
 * viewport whenever the tooltip is no larger than the viewport on either
 * axis. A placement that fits is returned with exactly the coordinates
 * `computeTooltipPosition` produces — clamping only ever moves a position
 * that already overflowed, so adjacency to the anchor is preserved wherever
 * it is achievable.
 *
 * When both sides of an axis lose — a narrow viewport that leaves less room
 * beside the anchor than the tooltip needs, or a centered cross axis that
 * overflows — clamping pulls the tooltip inside the edge it crossed and can
 * set it down on top of the anchor. Overlapping the icon beats leaving the
 * viewport: a tooltip outside it is unreadable, and the overlap is
 * short-lived — the opaque tooltip covers the icon only while it is shown,
 * and the icon is back the moment the pointer moves on. Callers drawing an
 * arrow from the tooltip to the anchor clamp the arrow's offset the same way,
 * so it stays attached to a tooltip that has been pulled over its anchor.
 * A tooltip larger than the viewport on an axis cannot be pulled inside that
 * axis at all; it is pinned to that axis's origin (x = 0 or y = 0) and
 * necessarily crosses the far edge. Pure arithmetic; no DOM reads.
 */
export function resolveTooltipPlacement(
  anchor: TooltipAnchorRect,
  tooltip: TooltipSize,
  preferred: TooltipPlacement,
  viewport: ViewportBounds,
): ResolvedTooltipPlacement {
  const placement = crossesFacingEdge(anchor, tooltip, preferred, viewport)
    ? FLIPPED[preferred]
    : preferred;
  return {
    placement,
    position: clampTooltipPosition(
      computeTooltipPosition(anchor, tooltip, placement),
      tooltip,
      viewport,
    ),
  };
}

/**
 * Whether `placement` pushes the tooltip past the viewport edge it faces: the
 * top edge for `above`, the bottom edge for `below`, the left edge for `left`,
 * the right edge for `right`. Touching the edge is not enough — only a
 * positive crossing counts, matching `placementOverflowsViewport`. The
 * coordinates come from `computeTooltipPosition`, so this predicate can never
 * disagree with the position that actually renders for the same inputs.
 */
function crossesFacingEdge(
  anchor: TooltipAnchorRect,
  tooltip: TooltipSize,
  placement: TooltipPlacement,
  viewport: ViewportBounds,
): boolean {
  const { x, y } = computeTooltipPosition(anchor, tooltip, placement);
  switch (placement) {
    case 'above':
      return y < 0;
    case 'below':
      return y + tooltip.height > viewport.height;
    case 'left':
      return x < 0;
    case 'right':
      return x + tooltip.width > viewport.width;
  }
}

/**
 * Clamp a tooltip position so the tooltip rectangle stays inside `viewport`.
 *
 * Each axis is clamped independently: `x` into `[0, viewport.width -
 * tooltip.width]` and `y` into `[0, viewport.height - tooltip.height]`, so
 * the tooltip neither crosses the viewport's leading (left/top) edge nor
 * trails past its far (right/bottom) edge. A position that already fits is
 * returned unchanged — a rectangle that merely touches an edge counts as
 * fitting, matching `placementOverflowsViewport`, which treats only a
 * positive crossing as overflow.
 *
 * Clamping is the move left for a placement nothing else can fix: the caller
 * has already flipped across the anchor by the time this runs, so both sides
 * have lost. Pulling the tooltip inside can set it down on top of the anchor,
 * which the unclamped placements never do — overlap with the anchor is the
 * accepted cost of keeping the tooltip readable. A tooltip larger than the
 * viewport on an axis makes that axis's range empty; the position is pinned
 * to `0` there and the tooltip necessarily crosses the far edge. Pure
 * arithmetic; no DOM reads.
 */
export function clampTooltipPosition(
  position: TooltipPosition,
  tooltip: TooltipSize,
  viewport: ViewportBounds,
): TooltipPosition {
  return {
    x: Math.max(0, Math.min(position.x, viewport.width - tooltip.width)),
    y: Math.max(0, Math.min(position.y, viewport.height - tooltip.height)),
  };
}

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
 *
 * The viewport is deliberately not consulted: these coordinates are honored
 * verbatim, overflow included. To find out whether a placement fits before
 * committing to it, see `placementOverflowsViewport`, which evaluates exactly
 * these coordinates; to pull an unfit position back inside, see
 * `clampTooltipPosition`.
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
