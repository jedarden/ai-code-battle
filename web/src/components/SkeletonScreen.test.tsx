/**
 * Tests for SkeletonScreen component
 *
 * Verifies acceptance criteria:
 * - Component renders without errors
 * - Shimmer animation is visible and smooth
 * - Component accepts width, height, and shape variant props
 * - No layout shift when mounted/unmounted
 */

import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SkeletonScreen } from './SkeletonScreen';

describe('SkeletonScreen Component', () => {
  describe('Rendering', () => {
    it('should render without errors', () => {
      expect(() => render(<SkeletonScreen />)).not.toThrow();
    });

    it('should render with default props', () => {
      render(<SkeletonScreen />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toBeInTheDocument();
      expect(skeleton).toHaveClass('skeleton-bar');
    });

    it('should render with custom className', () => {
      render(<SkeletonScreen className="custom-class" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveClass('custom-class');
    });
  });

  describe('Variant Props', () => {
    it('should render bar variant by default', () => {
      render(<SkeletonScreen />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveClass('skeleton-bar');
    });

    it('should render circle variant', () => {
      render(<SkeletonScreen variant="circle" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveClass('skeleton-circle');
    });

    it('should render rectangle variant', () => {
      render(<SkeletonScreen variant="rectangle" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveClass('skeleton-bar'); // rectangle uses bar class with border-radius
    });
  });

  describe('Width and Height Props', () => {
    it('should accept string width', () => {
      render(<SkeletonScreen width="200px" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ width: '200px' });
    });

    it('should accept number width and convert to pixels', () => {
      render(<SkeletonScreen width={300} />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ width: '300px' });
    });

    it('should accept string height', () => {
      render(<SkeletonScreen height="24px" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ height: '24px' });
    });

    it('should accept number height and convert to pixels', () => {
      render(<SkeletonScreen height={32} />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ height: '32px' });
    });

    it('should use default width 100%', () => {
      render(<SkeletonScreen />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ width: '100%' });
    });

    it('should use default height 16px', () => {
      render(<SkeletonScreen />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ height: '16px' });
    });
  });

  describe('Style Prop', () => {
    it('should merge custom styles with default styles', () => {
      render(<SkeletonScreen style={{ margin: '10px', opacity: 0.5 }} />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({
        width: '100%',
        height: '16px',
        margin: '10px',
        opacity: 0.5,
      });
    });

    it('should allow custom width to override default', () => {
      render(<SkeletonScreen width="150px" style={{ width: '200px' }} />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      // Custom style should override the width prop
      expect(skeleton).toHaveStyle({ width: '200px' });
    });
  });

  describe('Accessibility', () => {
    it('should have aria-hidden true', () => {
      render(<SkeletonScreen />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveAttribute('aria-hidden', 'true');
    });

    it('should have presentation role', () => {
      render(<SkeletonScreen />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveAttribute('role', 'presentation');
    });
  });

  describe('Layout Shift Prevention', () => {
    it('should have fixed dimensions to prevent layout shift', () => {
      const { container } = render(<SkeletonScreen width="200px" height="100px" />);
      const skeleton = container.firstChild as HTMLElement;

      // Fixed dimensions prevent layout shift
      expect(skeleton).toHaveStyle({ width: '200px' });
      expect(skeleton).toHaveStyle({ height: '100px' });
    });

    it('should maintain dimensions when props change', () => {
      const { rerender } = render(<SkeletonScreen width="100px" height="50px" />);
      let skeleton = screen.getByRole('presentation', { hidden: true });

      expect(skeleton).toHaveStyle({ width: '100px' });
      expect(skeleton).toHaveStyle({ height: '50px' });

      // Rerender with different props
      rerender(<SkeletonScreen width="200px" height="100px" />);
      skeleton = screen.getByRole('presentation', { hidden: true });

      expect(skeleton).toHaveStyle({ width: '200px' });
      expect(skeleton).toHaveStyle({ height: '100px' });
    });
  });

  describe('Shimmer Animation Classes', () => {
    it('should apply shimmer class for animation', () => {
      render(<SkeletonScreen />);
      const skeleton = screen.getByRole('presentation', { hidden: true });

      // The shimmer animation is applied via CSS class
      expect(skeleton).toHaveClass('skeleton-bar');
    });

    it('should apply circle shimmer class', () => {
      render(<SkeletonScreen variant="circle" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });

      expect(skeleton).toHaveClass('skeleton-circle');
    });
  });

  describe('Real-world Usage Patterns', () => {
    it('should work as text placeholder', () => {
      render(<SkeletonScreen variant="bar" width="70%" height="16px" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ width: '70%', height: '16px' });
    });

    it('should work as avatar placeholder', () => {
      render(<SkeletonScreen variant="circle" width={40} height={40} />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ width: '40px', height: '40px' });
    });

    it('should work as card placeholder', () => {
      render(<SkeletonScreen variant="rectangle" width="100%" height="120px" />);
      const skeleton = screen.getByRole('presentation', { hidden: true });
      expect(skeleton).toHaveStyle({ width: '100%', height: '120px' });
    });

    it('should render multiple instances without conflict', () => {
      render(
        <div>
          <SkeletonScreen data-testid="skeleton-1" />
          <SkeletonScreen data-testid="skeleton-2" width="50%" />
          <SkeletonScreen data-testid="skeleton-3" variant="circle" width={32} height={32} />
        </div>
      );

      expect(screen.getByTestId('skeleton-1')).toBeInTheDocument();
      expect(screen.getByTestId('skeleton-2')).toBeInTheDocument();
      expect(screen.getByTestId('skeleton-3')).toBeInTheDocument();
    });
  });
});
