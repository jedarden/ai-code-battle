// §16.15 Lazy section — defers rendering of below-the-fold content until
// it scrolls into view using IntersectionObserver. Shows a minimal
// placeholder until revealed.

export interface LazySectionOptions {
  /** Placeholder HTML shown until the section is visible */
  placeholder?: string;
  /** Margin around the root to trigger early (default "200px") */
  rootMargin?: string;
  /** Threshold for triggering (default 0) */
  threshold?: number;
}

const DEFAULT_PLACEHOLDER = '<div class="lazy-placeholder" style="min-height:60px"></div>';

/**
 * Wraps a block of HTML in a lazy-loaded container. The container starts
 * with a lightweight placeholder and swaps in the real content when it
 * enters the viewport (with a configurable margin for preloading).
 *
 * Returns the outer HTML string (safe to use in innerHTML assignments).
 */
export function lazySection(
  id: string,
  contentHtml: string,
  opts: LazySectionOptions = {}
): string {
  const placeholder = opts.placeholder ?? DEFAULT_PLACEHOLDER;
  return `<div class="lazy-section" data-lazy-id="${id}" data-lazy-content="${escapeAttr(contentHtml)}">${placeholder}</div>`;
}

/**
 * Activates all lazy sections within the given root element.
 * Call after mounting HTML (e.g., after innerHTML assignment).
 */
export function initLazySections(
  root: HTMLElement,
  opts: Pick<LazySectionOptions, 'rootMargin' | 'threshold'> = {}
): () => void {
  const rootMargin = opts.rootMargin ?? '200px';
  const threshold = opts.threshold ?? 0;

  const sections = root.querySelectorAll<HTMLElement>('.lazy-section');
  if (sections.length === 0) return () => {};

  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        const el = entry.target as HTMLElement;
        reveal(el);
        observer.unobserve(el);
      }
    },
    { rootMargin, threshold }
  );

  sections.forEach((s) => observer.observe(s));
  return () => observer.disconnect();
}

/** Immediately reveal a lazy section (also used by IntersectionObserver). */
export function revealLazySection(root: HTMLElement, id: string): void {
  const el = root.querySelector<HTMLElement>(`[data-lazy-id="${id}"]`);
  if (el) reveal(el);
}

function reveal(el: HTMLElement): void {
  const content = el.getAttribute('data-lazy-content');
  if (content === null) return;
  // Remove the data attribute to avoid re-revealing
  el.removeAttribute('data-lazy-content');
  el.classList.add('lazy-section-revealed');
  el.innerHTML = decodeAttr(content);
}

function escapeAttr(html: string): string {
  return html
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function decodeAttr(encoded: string): string {
  const el = document.createElement('textarea');
  el.innerHTML = encoded;
  return el.value;
}
