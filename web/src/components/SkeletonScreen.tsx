/**
 * SkeletonScreen Component
 *
 * A reusable skeleton loading component with shimmer animation for placeholder content.
 * Zero layout shift by design - uses fixed dimensions.
 *
 * @see §16.14 Skeleton screens
 */

import React from 'react';

/**
 * Supported skeleton shape variants
 */
export type SkeletonVariant = 'bar' | 'circle' | 'rectangle';

/**
 * Props for SkeletonScreen component
 */
export interface SkeletonScreenProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Shape variant: bar (default), circle, or rectangle */
  variant?: SkeletonVariant;
  /** Width of the skeleton element (default: 100%) */
  width?: string | number;
  /** Height of the skeleton element (default: 16px) */
  height?: string | number;
}

/**
 * SkeletonScreen component with shimmer animation
 *
 * The shimmer effect is a light gradient that sweeps left-to-right at 1.5s intervals.
 * Fixed dimensions prevent layout shift when the component mounts/unmounts.
 *
 * @example
 * ```tsx
 * <SkeletonScreen variant="bar" width="200px" height="16px" />
 * <SkeletonScreen variant="circle" width="40px" height="40px" />
 * <SkeletonScreen variant="rectangle" width="100%" height="100px" />
 * ```
 */
export const SkeletonScreen: React.FC<SkeletonScreenProps> = ({
  variant = 'bar',
  width = '100%',
  height = '16px',
  style,
  className = '',
  ...rest
}) => {
  // Convert numeric width/height to pixel strings
  const widthStr = typeof width === 'number' ? `${width}px` : width;
  const heightStr = typeof height === 'number' ? `${height}px` : height;

  // Base styles that prevent layout shift
  const baseStyle: React.CSSProperties = {
    width: widthStr,
    height: heightStr,
    ...style,
  };

  // Variant-specific class names
  // Rectangle uses skeleton-bar class with border-radius added via style
  const variantClass = variant === 'rectangle' ? 'skeleton-bar' : `skeleton-${variant}`;

  // Add border-radius for rectangle variant
  const finalStyle = variant === 'rectangle'
    ? { ...baseStyle, borderRadius: 'var(--radius-md)' }
    : baseStyle;

  return (
    <div
      className={`${variantClass} ${className}`.trim()}
      style={finalStyle}
      aria-hidden="true"
      role="presentation"
      {...rest}
    />
  );
};

export default SkeletonScreen;
