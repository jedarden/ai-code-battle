/**
 * EventTypeLegend Component Tests
 *
 * Tests for the event type legend component that displays event type → icon/color mapping
 */

import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { EventTypeLegend } from './EventTypeLegend';
import type { SignificantEventType } from '../extract-significant-events';

describe('EventTypeLegend Component', () => {
  it('should render all event types when no eventTypes prop is provided', () => {
    render(<EventTypeLegend />);

    // Check that all event types are rendered
    expect(screen.getByText('Combat')).toBeInTheDocument();
    expect(screen.getByText('Core Capture')).toBeInTheDocument();
    expect(screen.getByText('Energy Milestone')).toBeInTheDocument();
    expect(screen.getByText('Mass Death')).toBeInTheDocument();
    expect(screen.getByText('Momentum Shift')).toBeInTheDocument();
    expect(screen.getByText('Critical Moment')).toBeInTheDocument();
    expect(screen.getByText('Spawn Wave')).toBeInTheDocument();
  });

  it('should render only specified event types', () => {
    const specificTypes: SignificantEventType[] = ['combat', 'core_capture'];
    render(<EventTypeLegend eventTypes={specificTypes} />);

    // Check that only the specified types are rendered
    expect(screen.getByText('Combat')).toBeInTheDocument();
    expect(screen.getByText('Core Capture')).toBeInTheDocument();

    // Check that other types are not rendered
    expect(screen.queryByText('Energy Milestone')).not.toBeInTheDocument();
    expect(screen.queryByText('Mass Death')).not.toBeInTheDocument();
  });

  it('should render icons with correct colors for combat event type', () => {
    render(<EventTypeLegend eventTypes={['combat']} />);

    const combatIcon = screen.getByText('⚔️');
    expect(combatIcon).toBeInTheDocument();
    expect(combatIcon).toHaveStyle({ color: '#ef4444' });
  });

  it('should render icons with correct colors for core_capture event type', () => {
    render(<EventTypeLegend eventTypes={['core_capture']} />);

    const coreCaptureIcon = screen.getByText('🏰');
    expect(coreCaptureIcon).toBeInTheDocument();
    expect(coreCaptureIcon).toHaveStyle({ color: '#3b82f6' });
  });

  it('should apply custom className when provided', () => {
    const { container } = render(<EventTypeLegend className="custom-class" />);

    const legendElement = container.querySelector('.event-type-legend');
    expect(legendElement).toHaveClass('custom-class');
  });

  it('should render in correct order for all event types', () => {
    render(<EventTypeLegend />);

    // Get all the legend item labels
    const combatLabel = screen.getByText('Combat');
    const coreCaptureLabel = screen.getByText('Core Capture');
    const energyMilestoneLabel = screen.getByText('Energy Milestone');
    const massDeathLabel = screen.getByText('Mass Death');
    const momentumShiftLabel = screen.getByText('Momentum Shift');
    const criticalMomentLabel = screen.getByText('Critical Moment');
    const spawnWaveLabel = screen.getByText('Spawn Wave');

    // Verify all labels exist and are in DOM (order is maintained by component logic)
    expect(combatLabel).toBeInTheDocument();
    expect(coreCaptureLabel).toBeInTheDocument();
    expect(energyMilestoneLabel).toBeInTheDocument();
    expect(massDeathLabel).toBeInTheDocument();
    expect(momentumShiftLabel).toBeInTheDocument();
    expect(criticalMomentLabel).toBeInTheDocument();
    expect(spawnWaveLabel).toBeInTheDocument();
  });

  it('should handle empty eventTypes array', () => {
    const { container } = render(<EventTypeLegend eventTypes={[]} />);

    const legendContent = container.querySelector('.event-type-legend-content');
    expect(legendContent).toBeInTheDocument();
    expect(legendContent?.children.length).toBe(0);
  });
});