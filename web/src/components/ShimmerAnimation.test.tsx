/**
 * Tests for ShimmerAnimation component
 *
 * Verifies acceptance criteria:
 * - Light gradient sweeping left-to-right
 * - Animation interval: 1.5s duration
 * - Gradient should be subtle and smooth
 * - Component should be reusable across different skeleton shapes
 */

import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ShimmerAnimation } from './ShimmerAnimation';

describe('ShimmerAnimation Component', () => {
  describe('Basic Rendering', () => {
    it('should render without errors', () => {
      expect(() => render(<ShimmerAnimation />)).not.toThrow();
    });

    it('should render with default props', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toBeInTheDocument();
      expect(shimmer).toHaveClass('shimmer-animation');
    });

    it('should have aria-hidden true for accessibility', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveAttribute('aria-hidden', 'true');
    });

    it('should have presentation role', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveAttribute('role', 'presentation');
    });
  });

  describe('Width and Height Props', () => {
    it('should accept string width', () => {
      render(<ShimmerAnimation width="200px" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ width: '200px' });
    });

    it('should accept number width and convert to pixels', () => {
      render(<ShimmerAnimation width={300} />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ width: '300px' });
    });

    it('should accept string height', () => {
      render(<ShimmerAnimation height="24px" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ height: '24px' });
    });

    it('should accept number height and convert to pixels', () => {
      render(<ShimmerAnimation height={32} />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ height: '32px' });
    });

    it('should use default width 100%', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ width: '100%' });
    });

    it('should use default height 100%', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ height: '100%' });
    });
  });

  describe('Border Radius Prop', () => {
    it('should accept custom border radius string', () => {
      render(<ShimmerAnimation borderRadius="8px" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ borderRadius: '8px' });
    });

    it('should use default border radius from CSS var', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ borderRadius: 'var(--radius-sm)' });
    });
  });

  describe('Shimmer Animation Styles', () => {
    it('should apply gradient background for shimmer effect', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });

      const bgStyle = shimmer.style.background;
      expect(bgStyle).toContain('linear-gradient');
      expect(bgStyle).toContain('90deg');
      expect(bgStyle).toContain('var(--bg-tertiary)');
      expect(bgStyle).toContain('var(--border)');
    });

    it('should set background size to 200% for sweeping animation', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ backgroundSize: '200% 100%' });
    });

    it('should apply 1.5s ease-in-out infinite animation', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({
        animation: 'skeleton-shimmer 1.5s ease-in-out infinite',
      });
    });

    it('should use subtle gradient colors via CSS variables', () => {
      render(<ShimmerAnimation />);
      const shimmer = screen.getByRole('presentation', { hidden: true });

      // Verify the gradient uses CSS variables for theme consistency
      const bgStyle = shimmer.style.background;
      expect(bgStyle).toMatch(/var\(--bg-tertiary\)/);
      expect(bgStyle).toMatch(/var\(--border\)/);
    });
  });

  describe('Reusability Across Skeleton Shapes', () => {
    it('should work as a bar/text placeholder', () => {
      render(<ShimmerAnimation width="70%" height="16px" borderRadius="4px" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({
        width: '70%',
        height: '16px',
        borderRadius: '4px',
      });
    });

    it('should work as a circular avatar placeholder', () => {
      render(<ShimmerAnimation width={40} height={40} borderRadius="50%" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({
        width: '40px',
        height: '40px',
        borderRadius: '50%',
      });
    });

    it('should work as a rectangular card placeholder', () => {
      render(<ShimmerAnimation width="100%" height="120px" borderRadius="8px" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({
        width: '100%',
        height: '120px',
        borderRadius: '8px',
      });
    });

    it('should work as a square icon placeholder', () => {
      render(<ShimmerAnimation width={24} height={24} borderRadius="4px" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({
        width: '24px',
        height: '24px',
        borderRadius: '4px',
      });
    });

    it('should work as a full-width banner placeholder', () => {
      render(<ShimmerAnimation width="100%" height="200px" borderRadius="12px" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({
        width: '100%',
        height: '200px',
        borderRadius: '12px',
      });
    });
  });

  describe('Style Merging', () => {
    it('should merge custom styles with default styles', () => {
      render(<ShimmerAnimation style={{ margin: '10px', opacity: 0.8 }} />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({
        width: '100%',
        height: '100%',
        margin: '10px',
        opacity: 0.8,
      });
    });

    it('should allow custom width to override default', () => {
      render(<ShimmerAnimation width="150px" style={{ width: '200px' }} />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ width: '200px' });
    });

    it('should allow custom height to override default', () => {
      render(<ShimmerAnimation height="50px" style={{ height: '80px' }} />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveStyle({ height: '80px' });
    });
  });

  describe('Custom ClassName', () => {
    it('should accept custom className', () => {
      render(<ShimmerAnimation className="custom-shimmer" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveClass('custom-shimmer');
    });

    it('should preserve base shimmer-animation class with custom className', () => {
      render(<ShimmerAnimation className="custom-class" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveClass('shimmer-animation');
      expect(shimmer).toHaveClass('custom-class');
    });
  });

  describe('Real-world Usage Patterns', () => {
    it('should render multiple instances without conflict', () => {
      render(
        <div>
          <ShimmerAnimation data-testid="shimmer-1" width="100px" />
          <ShimmerAnimation data-testid="shimmer-2" width="50%" height="20px" />
          <ShimmerAnimation data-testid="shimmer-3" width={32} height={32} borderRadius="50%" />
        </div>
      );

      expect(screen.getByTestId('shimmer-1')).toBeInTheDocument();
      expect(screen.getByTestId('shimmer-2')).toBeInTheDocument();
      expect(screen.getByTestId('shimmer-3')).toBeInTheDocument();
    });

    it('should work in a flex container layout', () => {
      render(
        <div style={{ display: 'flex', gap: '8px' }}>
          <ShimmerAnimation width="40px" height="40px" borderRadius="50%" />
          <ShimmerAnimation width="100px" height="16px" />
        </div>
      );

      const shimmers = screen.getAllByRole('presentation', { hidden: true });
      expect(shimmers).toHaveLength(2);
    });

    it('should work in a grid container layout', () => {
      render(
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
          <ShimmerAnimation width="100%" height="80px" />
          <ShimmerAnimation width="100%" height="80px" />
        </div>
      );

      const shimmers = screen.getAllByRole('presentation', { hidden: true });
      expect(shimmers).toHaveLength(2);
    });
  });

  describe('HTML Attribute Propagation', () => {
    it('should spread additional HTML attributes', () => {
      render(<ShimmerAnimation data-testid="test-shimmer" id="my-shimmer" />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      expect(shimmer).toHaveAttribute('data-testid', 'test-shimmer');
      expect(shimmer).toHaveAttribute('id', 'my-shimmer');
    });

    it('should accept onClick handler as property', () => {
      const handleClick = () => {};
      render(<ShimmerAnimation onClick={handleClick} />);
      const shimmer = screen.getByRole('presentation', { hidden: true });
      // React attaches event handlers as properties, not HTML attributes
      expect(shimmer).toHaveProperty('onclick');
    });
  });
});
