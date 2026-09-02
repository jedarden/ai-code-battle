/**
 * ShimmerAnimation Component
 *
 * A reusable shimmer animation component that produces a light gradient sweeping left-to-right.
 * This is the foundational animation component for all skeleton screens.
 *
 * @see §16.14 Skeleton screens
 * @see Acceptance criteria:
 *   - Light gradient sweeping left-to-right
 *   - Animation interval: 1.5s duration
 *   - Subtle and smooth gradient
 *   - Reusable across different skeleton shapes
 */

import React from 'react';

/**
 * Props for ShimmerAnimation component
 */
export interface ShimmerAnimationProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Width of the shimmer element (default: 100%) */
  width?: string | number;
  /** Height of the shimmer element (default: 100%) */
  height?: string | number;
  /** Border radius for the shimmer (default: var(--radius-sm)) */
  borderRadius?: string;
}

/**
 * ShimmerAnimation component
 *
 * Produces a light gradient that sweeps left-to-right at 1.5s intervals.
 * The gradient is subtle and smooth, using CSS variables for theme consistency.
 *
 * The animation works by creating a background gradient that is 200% wide,
 * then animating the background-position from -200% to 200% over 1.5 seconds.
 * This creates a smooth sweeping effect from left to right.
 *
 * @example
 * ```tsx
 * // Basic usage
 * <ShimmerAnimation width="100px" height="20px" />
 *
 * // As a circular avatar placeholder
 * <ShimmerAnimation width={40} height={40} borderRadius="50%" />
 *
 * // As a rectangular card placeholder
 * <ShimmerAnimation width="100%" height="120px" borderRadius="8px" />
 * ```
 */
export const ShimmerAnimation: React.FC<ShimmerAnimationProps> = ({
  width = '100%',
  height = '100%',
  borderRadius = 'var(--radius-sm)',
  style,
  className = '',
  ...rest
}) => {
  // Convert numeric width/height to pixel strings
  const widthStr = typeof width === 'number' ? `${width}px` : width;
  const heightStr = typeof height === 'number' ? `${height}px` : height;

  // Base styles with shimmer animation
  const baseStyle: React.CSSProperties = {
    width: widthStr,
    height: heightStr,
    borderRadius,
    // The shimmer gradient: light → medium → light, creating a sweeping effect
    background: 'linear-gradient(90deg, var(--bg-tertiary) 25%, var(--border) 37%, var(--bg-tertiary) 63%)',
    backgroundSize: '200% 100%',
    animation: 'skeleton-shimmer 1.5s ease-in-out infinite',
    ...style,
  };

  return (
    <div
      className={`shimmer-animation ${className}`.trim()}
      style={baseStyle}
      aria-hidden="true"
      role="presentation"
      {...rest}
    />
  );
};

export default ShimmerAnimation;
