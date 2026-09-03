/**
 * EventTypeLegend Component
 *
 * A React component that displays event type → icon/color mapping
 * Shows icon, type name, and associated color for each event type
 *
 * @see SignificantEventType from extract-significant-events.ts
 */

import React from 'react';
import type { SignificantEventType } from '../extract-significant-events';
import { EVENT_TYPE_REGISTRY, getEventTypeDescriptor, type EventTypeDescriptor } from './event-type-registry';

/**
 * Event type configuration with icon, color, and label.
 * Sourced from the event type registry — the same single source the ribbon
 * markers, tooltip and legend read — so this key can never drift from what
 * the ribbon actually shows.
 */
interface EventTypeConfig extends EventTypeDescriptor {
  type: SignificantEventType;
}

/**
 * Props for EventTypeLegend component
 */
export interface EventTypeLegendProps {
  /** Optional array of event types to display (shows all if not provided) */
  eventTypes?: SignificantEventType[];
  /** Optional additional CSS classes */
  className?: string;
}

/**
 * EventTypeLegend component displays event type mappings with icons and colors
 *
 * @example
 * ```tsx
 * // Display all event types
 * <EventTypeLegend />
 *
 * // Display specific event types
 * <EventTypeLegend eventTypes={['combat', 'core_capture']} />
 * ```
 */
export const EventTypeLegend: React.FC<EventTypeLegendProps> = ({
  eventTypes,
  className = ''
}) => {
  // Use all event types if not provided
  const typesToDisplay = eventTypes || (Object.keys(EVENT_TYPE_REGISTRY) as SignificantEventType[]);

  // Resolve through the registry's own lookup rather than indexing it
  // directly: a value outside the union — reachable from unvalidated data via
  // a plain-JS caller — then falls back to the unknown-type descriptor instead
  // of producing an entry with no icon, name or color
  const eventConfigs: EventTypeConfig[] = typesToDisplay.map(type => ({
    type,
    ...getEventTypeDescriptor(type)
  }));

  return (
    <div className={`event-type-legend ${className}`.trim()}>
      <div className="event-type-legend-content">
        {eventConfigs.map(({ type, name, icon, color }) => (
          <div key={type} className="event-type-legend-item">
            <span
              className="event-type-legend-icon"
              style={{ color }}
              aria-hidden="true"
            >
              {icon}
            </span>
            <span className="event-type-legend-label">
              {name}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default EventTypeLegend;