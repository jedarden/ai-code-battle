/**
 * Tooltip Component
 *
 * A reusable tooltip component with configurable positioning and visibility.
 * Renders absolutely positioned content with basic styling for hover or manual display.
 *
 * @see §16.15 Tooltips and popovers
 */

import React from 'react';

/**
 * Position coordinates for tooltip placement
 */
export interface TooltipPosition {
  /** X coordinate in pixels */
  x: number;
  /** Y coordinate in pixels */
  y: number;
}

/**
 * Props for Tooltip component
 */
export interface TooltipProps {
  /** Content to display in the tooltip */
  content: string | React.ReactNode;
  /** Position coordinates for tooltip placement */
  position: TooltipPosition;
  /** Whether the tooltip is visible */
  visible: boolean;
  /** Optional additional CSS classes */
  className?: string;
  /** Optional z-index value (defaults to --z-tooltip variable) */
  zIndex?: number;
}

/**
 * Tooltip component with absolute positioning
 *
 * The tooltip renders with fixed positioning based on provided coordinates.
 * It uses CSS variables for consistent styling and supports rich content via React nodes.
 *
 * @example
 * ```tsx
 * const [position, setPosition] = useState({ x: 100, y: 200 });
 * const [visible, setVisible] = useState(false);
 *
 * <Tooltip
 *   content="Helpful information"
 *   position={position}
 *   visible={visible}
 * />
 *
 * // With rich content
 * <Tooltip
 *   position={position}
 *   visible={visible}
 *   content={
 *     <div>
 *       <strong>Title</strong>
 *       <p>Description text</p>
 *     </div>
 *   }
 * />
 * ```
 */
export const Tooltip: React.FC<TooltipProps> = ({
  content,
  position,
  visible,
  className = '',
  zIndex,
}) => {
  if (!visible) {
    return null;
  }

  const tooltipStyle: React.CSSProperties = {
    position: 'absolute',
    left: `${position.x}px`,
    top: `${position.y}px`,
    zIndex: zIndex ?? 'var(--z-tooltip, 1000)',
  };

  return (
    <div
      className={`tooltip ${className}`.trim()}
      style={tooltipStyle}
      role="tooltip"
      aria-live="polite"
    >
      <div className="tooltip-content">
        {content}
      </div>
      {/* Arrow pointer */}
      <div className="tooltip-arrow" />
    </div>
  );
};

export default Tooltip;
