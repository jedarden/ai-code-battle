// §16.15 Virtual list — renders only visible rows for large datasets.
// Keeps the DOM small even with 1000+ entries. Each row has a base height
// and only rows in the viewport (plus a buffer) are materialised.
// Expanded rows are tracked with their actual measured height so spacer
// calculations remain correct.

export interface VirtualListOptions<T> {
  items: T[];
  rowHeight: number;
  /** Extra rows rendered above/below viewport (default 5) */
  buffer?: number;
  /** Initial number of rows to show before requiring "Show more" */
  initialCount?: number;
  renderRow: (item: T, index: number) => string;
  renderExpanded?: (item: T, index: number) => string;
  containerClass?: string;
  ariaLabel?: string;
}

interface VirtualListState {
  scrollTop: number;
  viewportHeight: number;
  visibleStart: number;
  visibleEnd: number;
  expandedIndex: number | null;
  expandedHeight: number;
  showMoreCount: number;
}

export class VirtualList<T> {
  private opts: VirtualListOptions<T>;
  private state: VirtualListState;
  private scrollEl: HTMLElement | null = null;
  private spacerAbove: HTMLElement | null = null;
  private spacerBelow: HTMLElement | null = null;
  private rowContainer: HTMLElement | null = null;
  private showMoreEl: HTMLElement | null = null;
  private rafId = 0;
  private observer: ResizeObserver | null = null;

  constructor(opts: VirtualListOptions<T>) {
    this.opts = opts;
    const initial = opts.initialCount ?? opts.items.length;
    this.state = {
      scrollTop: 0,
      viewportHeight: 600,
      visibleStart: 0,
      visibleEnd: Math.min(initial, opts.items.length),
      expandedIndex: null,
      expandedHeight: 0,
      showMoreCount: initial,
    };
  }

  /** Mount the virtual list into the given container. */
  mount(container: HTMLElement): void {
    container.innerHTML = '';
    container.classList.add(this.opts.containerClass ?? 'virtual-list');

    // Scrollable viewport
    const scrollEl = document.createElement('div');
    scrollEl.className = 'virtual-list-scroll';
    scrollEl.setAttribute('role', 'list');
    if (this.opts.ariaLabel) scrollEl.setAttribute('aria-label', this.opts.ariaLabel);
    scrollEl.tabIndex = 0;
    this.scrollEl = scrollEl;

    // Spacers for total height
    this.spacerAbove = document.createElement('div');
    this.spacerBelow = document.createElement('div');

    // Actual rendered rows
    this.rowContainer = document.createElement('div');
    this.rowContainer.className = 'virtual-list-rows';

    scrollEl.appendChild(this.spacerAbove);
    scrollEl.appendChild(this.rowContainer);
    scrollEl.appendChild(this.spacerBelow);
    container.appendChild(scrollEl);

    // "Show more" button
    const showMoreEl = document.createElement('button');
    showMoreEl.className = 'virtual-list-show-more btn secondary';
    showMoreEl.type = 'button';
    this.showMoreEl = showMoreEl;
    this.updateShowMore();
    container.appendChild(this.showMoreEl);

    // Bind events
    scrollEl.addEventListener('scroll', () => this.onScroll(), { passive: true });
    this.showMoreEl.addEventListener('click', () => this.onShowMore());

    // Keyboard: Enter/Space on a row toggles expand
    scrollEl.addEventListener('keydown', (e) => this.onKeyDown(e));

    // Track viewport height
    this.observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        this.state.viewportHeight = entry.contentRect.height;
        this.render();
      }
    });
    this.observer.observe(scrollEl);

    // Initial render
    this.state.viewportHeight = scrollEl.clientHeight || 600;
    this.render();
  }

  destroy(): void {
    if (this.rafId) cancelAnimationFrame(this.rafId);
    this.observer?.disconnect();
  }

  private onScroll(): void {
    if (this.rafId) return;
    this.rafId = requestAnimationFrame(() => {
      this.rafId = 0;
      if (!this.scrollEl) return;
      this.state.scrollTop = this.scrollEl.scrollTop;
      this.computeWindow();
      this.render();
    });
  }

  /** Compute the height of a row at the given index, accounting for expansion. */
  private rowHeightAt(idx: number): number {
    if (this.state.expandedIndex === idx && this.state.expandedHeight > 0) {
      return this.opts.rowHeight + this.state.expandedHeight;
    }
    return this.opts.rowHeight;
  }

  /** Compute the total height of rows from index 0 up to (but not including) `end`. */
  private totalHeight(end: number): number {
    const { rowHeight } = this.opts;
    if (this.state.expandedIndex === null || this.state.expandedHeight === 0) {
      // Fast path: no expanded row, all same height
      return end * rowHeight;
    }
    let h = 0;
    for (let i = 0; i < end; i++) {
      h += this.rowHeightAt(i);
    }
    return h;
  }

  /** Find the row index that contains the given scroll offset. */
  private indexAtOffset(offset: number): number {
    const { rowHeight, items } = this.opts;
    const maxIdx = Math.min(this.state.showMoreCount, items.length);
    if (this.state.expandedIndex === null || this.state.expandedHeight === 0) {
      // Fast path: uniform row heights
      return Math.floor(offset / rowHeight);
    }
    let acc = 0;
    for (let i = 0; i < maxIdx; i++) {
      acc += this.rowHeightAt(i);
      if (acc > offset) return i;
    }
    return maxIdx;
  }

  private computeWindow(): void {
    const buffer = this.opts.buffer ?? 5;
    const maxIdx = this.state.showMoreCount;

    const rawStart = this.indexAtOffset(this.state.scrollTop) - buffer;
    const rawEnd = this.indexAtOffset(this.state.scrollTop + this.state.viewportHeight) + buffer;

    this.state.visibleStart = Math.max(0, rawStart);
    this.state.visibleEnd = Math.min(maxIdx, rawEnd);
  }

  private render(): void {
    if (!this.rowContainer || !this.spacerAbove || !this.spacerBelow) return;
    const { items, renderRow, renderExpanded } = this.opts;
    const { visibleStart, visibleEnd, expandedIndex, showMoreCount } = this.state;

    const frags: string[] = [];
    for (let i = visibleStart; i < visibleEnd; i++) {
      const item = items[i];
      if (!item) continue;
      const isExpanded = expandedIndex === i;
      frags.push(`<div class="virtual-list-row${isExpanded ? ' expanded' : ''}" data-idx="${i}" role="listitem" tabindex="0">`);
      frags.push(renderRow(item, i));
      if (isExpanded && renderExpanded) {
        frags.push(`<div class="virtual-list-expanded">${renderExpanded(item, i)}</div>`);
      }
      frags.push('</div>');
    }

    this.rowContainer.innerHTML = frags.join('');

    // Spacer above = total height of all rows before visibleStart
    this.spacerAbove.style.height = `${this.totalHeight(visibleStart)}px`;

    // Spacer below = total height of all rows after visibleEnd up to showMoreCount
    let belowHeight = 0;
    for (let i = visibleEnd; i < showMoreCount; i++) {
      belowHeight += this.rowHeightAt(i);
    }
    this.spacerBelow.style.height = `${Math.max(0, belowHeight)}px`;

    // Wire expand toggles
    this.rowContainer.querySelectorAll<HTMLElement>('.virtual-list-row').forEach(row => {
      row.addEventListener('click', (e) => {
        if ((e.target as HTMLElement).closest('a, button')) return;
        const idx = Number(row.dataset.idx);
        this.toggleExpand(idx);
      });
    });

    // Measure expanded row height after DOM update
    if (expandedIndex !== null) {
      const expandedRow = this.rowContainer.querySelector(`[data-idx="${expandedIndex}"]`);
      const expandedEl = expandedRow?.querySelector<HTMLElement>('.virtual-list-expanded');
      if (expandedEl) {
        const measured = expandedEl.offsetHeight;
        if (measured !== this.state.expandedHeight) {
          this.state.expandedHeight = measured;
          // Recalculate spacers with the new height
          this.spacerAbove.style.height = `${this.totalHeight(visibleStart)}px`;
          let belowHeight2 = 0;
          for (let i = visibleEnd; i < showMoreCount; i++) {
            belowHeight2 += this.rowHeightAt(i);
          }
          this.spacerBelow.style.height = `${Math.max(0, belowHeight2)}px`;
        }
      }
    }
  }

  private toggleExpand(idx: number): void {
    const prevExpanded = this.state.expandedIndex;
    if (prevExpanded === idx) {
      // Collapse current
      this.state.expandedIndex = null;
      this.state.expandedHeight = 0;
    } else {
      // Expand new (height measured after render)
      this.state.expandedIndex = idx;
      this.state.expandedHeight = 0;
    }
    this.render();
  }

  private onKeyDown(e: KeyboardEvent): void {
    const row = (e.target as HTMLElement).closest('.virtual-list-row') as HTMLElement | null;
    if (!row) return;

    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      const idx = Number(row.dataset.idx);
      this.toggleExpand(idx);
    }
  }

  private onShowMore(): void {
    const batchSize = 100;
    this.state.showMoreCount = Math.min(
      this.state.showMoreCount + batchSize,
      this.opts.items.length
    );
    this.computeWindow();
    this.render();
    this.updateShowMore();
  }

  private updateShowMore(): void {
    if (!this.showMoreEl) return;
    const remaining = this.opts.items.length - this.state.showMoreCount;
    if (remaining <= 0) {
      this.showMoreEl.style.display = 'none';
      return;
    }
    this.showMoreEl.style.display = '';
    const next = Math.min(100, remaining);
    this.showMoreEl.textContent = `Show ${next} more (${remaining} remaining)`;
    this.showMoreEl.setAttribute('aria-label', `Show ${next} more entries, ${remaining} remaining`);
  }
}
