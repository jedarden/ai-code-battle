// Event Ribbon Component Tests
// Test edge cases and proportional positioning

import { describe, it, expect, beforeEach } from 'vitest';
import { EventRibbon } from './event-ribbon';
import type { SignificantEvent } from '../extract-significant-events';

describe('EventRibbon', () => {
  let container: HTMLElement;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
  });

  describe('Edge case handling', () => {
    it('should display "No turns" when totalTurns is 0', () => {
      const events: SignificantEvent[] = [];
      const ribbon = new EventRibbon(container, { events, totalTurns: 0 });

      expect(container.querySelector('.ribbon-empty')?.textContent).toBe('No turns');
    });

    it('should handle negative totalTurns gracefully', () => {
      const events: SignificantEvent[] = [];
      const ribbon = new EventRibbon(container, { events, totalTurns: -5 });

      expect(container.querySelector('.ribbon-empty')?.textContent).toBe('No turns');
    });

    it('should handle single-turn game (totalTurns = 1)', () => {
      const events: SignificantEvent[] = [
        {
          type: 'combat',
          turn: 0,
          description: 'Combat event',
          emoji: '⚔️',
        },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 1 });

      const marker = container.querySelector('.ribbon-marker');
      expect(marker).toBeTruthy();
      // Events should be centered at 50% for single-turn games
      expect(marker?.getAttribute('style')).toContain('left: 50%');
    });

    it('should position events on turn 0 at far left (0%)', () => {
      const events: SignificantEvent[] = [
        {
          type: 'spawn_wave',
          turn: 0,
          description: 'Initial spawn',
          emoji: '🐣',
        },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      const marker = container.querySelector('.ribbon-marker');
      expect(marker?.getAttribute('style')).toContain('left: 0%');
    });

    it('should position events on final turn at far right (100%)', () => {
      const events: SignificantEvent[] = [
        {
          type: 'critical_moment',
          turn: 99,
          description: 'Game end',
          emoji: '🏆',
        },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      const marker = container.querySelector('.ribbon-marker');
      expect(marker?.getAttribute('style')).toContain('left: 99%');
    });

    it('should position events proportionally in middle turns', () => {
      const events: SignificantEvent[] = [
        { type: 'combat', turn: 50, description: 'Mid-game combat', emoji: '⚔️' },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      const marker = container.querySelector('.ribbon-marker');
      expect(marker?.getAttribute('style')).toContain('left: 50%');
    });
  });

  describe('Proportional positioning', () => {
    it('should correctly calculate left positions for multiple events', () => {
      const events: SignificantEvent[] = [
        { type: 'combat', turn: 0, description: 'Start', emoji: '⚔️' },
        { type: 'combat', turn: 25, description: 'Quarter', emoji: '⚔️' },
        { type: 'combat', turn: 50, description: 'Half', emoji: '⚔️' },
        { type: 'combat', turn: 75, description: 'Three-quarter', emoji: '⚔️' },
        { type: 'combat', turn: 100, description: 'End', emoji: '⚔️' },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      const markers = container.querySelectorAll('.ribbon-marker');
      expect(markers.length).toBe(5);

      // Check proportional positioning
      expect(markers[0]?.getAttribute('style')).toContain('left: 0%');
      expect(markers[1]?.getAttribute('style')).toContain('left: 25%');
      expect(markers[2]?.getAttribute('style')).toContain('left: 50%');
      expect(markers[3]?.getAttribute('style')).toContain('left: 75%');
      expect(markers[4]?.getAttribute('style')).toContain('left: 100%');
    });

    it('should handle multiple events on the same turn', () => {
      const events: SignificantEvent[] = [
        { type: 'combat', turn: 50, description: 'Combat 1', emoji: '⚔️' },
        { type: 'energy_milestone', turn: 50, description: 'Energy milestone', emoji: '💎' },
        { type: 'spawn_wave', turn: 50, description: 'Spawn wave', emoji: '🐣' },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      const markers = container.querySelectorAll('.ribbon-marker');
      expect(markers.length).toBe(3);

      // All should be at the same position
      markers.forEach(marker => {
        expect(marker?.getAttribute('style')).toContain('left: 50%');
      });
    });
  });

  describe('Event handling', () => {
    it('should call onEventClick when marker is clicked', () => {
      let clickedEvent: SignificantEvent | undefined;
      const events: SignificantEvent[] = [
        { type: 'combat', turn: 25, description: 'Test combat', emoji: '⚔️' },
      ];

      const ribbon = new EventRibbon(container, {
        events,
        totalTurns: 100,
        onEventClick: (event) => {
          clickedEvent = event;
        },
      });

      const marker = container.querySelector('.ribbon-marker');
      marker?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

      expect(clickedEvent).toEqual(events[0]);
    });

    it('should call onTurnClick when marker is clicked', () => {
      let clickedTurn: number | undefined;
      const events: SignificantEvent[] = [
        { type: 'combat', turn: 42, description: 'Test combat', emoji: '⚔️' },
      ];

      const ribbon = new EventRibbon(container, {
        events,
        totalTurns: 100,
        onTurnClick: (turn) => {
          clickedTurn = turn;
        },
      });

      const marker = container.querySelector('.ribbon-marker');
      marker?.dispatchEvent(new MouseEvent('click', { bubbles: true }));

      expect(clickedTurn).toBe(42);
    });

    it('should seek to correct turn when track is clicked', () => {
      let clickedTurn: number | undefined;
      const events: SignificantEvent[] = [];

      const ribbon = new EventRibbon(container, {
        events,
        totalTurns: 100,
        onTurnClick: (turn) => {
          clickedTurn = turn;
        },
      });

      const track = container.querySelector('.ribbon-track');
      expect(track).toBeTruthy();

      // Mock getBoundingClientRect for testing
      if (track) {
        const originalGetBoundingClientRect = track.getBoundingClientRect;
        track.getBoundingClientRect = () => ({
          left: 0,
          top: 0,
          width: 400,
          height: 32,
          right: 400,
          bottom: 32,
          x: 0,
          y: 0,
          toJSON: () => ({}),
        });

        // Simulate clicking at 25% of track width (100px / 400px = 25%)
        const clickEvent = new MouseEvent('click', {
          bubbles: true,
          clientX: 100,
        });

        track.dispatchEvent(clickEvent);

        // 100px / 400px = 25%, should seek to turn 25
        expect(clickedTurn).toBe(25);

        // Restore original method
        track.getBoundingClientRect = originalGetBoundingClientRect;
      }
    });
  });

  describe('UI rendering', () => {
    it('should render turn label with current and total turns', () => {
      const events: SignificantEvent[] = [];
      const ribbon = new EventRibbon(container, { events, totalTurns: 150 });

      const currentLabel = container.querySelector('#ribbon-current');
      const totalLabel = container.querySelector('#ribbon-total');

      expect(currentLabel?.textContent).toBe('0');
      expect(totalLabel?.textContent).toBe('150');
    });

    it('should update current turn label when setCurrentTurn is called', () => {
      const events: SignificantEvent[] = [];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      ribbon.setCurrentTurn(75);

      const currentLabel = container.querySelector('#ribbon-current');
      expect(currentLabel?.textContent).toBe('75');
    });

    it('should highlight active marker when current turn matches event turn', () => {
      const events: SignificantEvent[] = [
        { type: 'combat', turn: 50, description: 'Mid-game', emoji: '⚔️' },
        { type: 'combat', turn: 25, description: 'Early game', emoji: '⚔️' },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      ribbon.setCurrentTurn(50);

      const markers = container.querySelectorAll('.ribbon-marker');
      expect(markers[0]?.classList.contains('ribbon-marker-active')).toBe(true);
      expect(markers[1]?.classList.contains('ribbon-marker-active')).toBe(false);
    });
  });

  describe('Set events method', () => {
    it('should update ribbon when setEvents is called', () => {
      const ribbon = new EventRibbon(container);
      expect(container.querySelector('.ribbon-empty')).toBeTruthy();

      const events: SignificantEvent[] = [
        { type: 'combat', turn: 10, description: 'New event', emoji: '⚔️' },
      ];

      ribbon.setEvents(events, 50);

      expect(container.querySelector('.ribbon-marker')).toBeTruthy();
      expect(container.querySelector('#ribbon-total')?.textContent).toBe('50');
    });
  });

  describe('CSS styling', () => {
    it('should render ribbon with proper base classes', () => {
      const events: SignificantEvent[] = [
        { type: 'combat', turn: 10, description: 'Test', emoji: '⚔️' },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      expect(container.querySelector('.event-ribbon')).toBeTruthy();
      expect(container.querySelector('.ribbon-track')).toBeTruthy();
      expect(container.querySelector('.ribbon-progress')).toBeTruthy();
      expect(container.querySelector('.ribbon-turn-label')).toBeTruthy();
    });

    it('should apply event-type-specific styling', () => {
      const events: SignificantEvent[] = [
        { type: 'mass_death', turn: 10, description: 'Mass death', emoji: '💀' },
        { type: 'energy_milestone', turn: 20, description: 'Energy', emoji: '💎' },
      ];
      const ribbon = new EventRibbon(container, { events, totalTurns: 100 });

      const markers = container.querySelectorAll('.ribbon-marker');
      expect(markers[0]?.getAttribute('data-type')).toBe('mass_death');
      expect(markers[1]?.getAttribute('data-type')).toBe('energy_milestone');
    });
  });
});
