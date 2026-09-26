// Page-side bootstrap for agentation-mount.spec.ts, which bundles this file
// with esbuild and injects it into the fixture page via addScriptTag.
//
// It mounts the REAL Agentation toolbar from the npm package (the same
// component initAgentation renders in the app, mounted into the same
// #agentation-root element id) in demo mode, which drives the toolbar's own
// identify pipeline (querySelector → identifyElement → annotation state)
// against the fixture's #demo-target element.
//
// The identification helpers are exposed for the spec's direct assertions.
// They live in the page — every call goes through page.evaluate; functions
// cannot cross into Node. The demo annotation itself is observed through the
// toolbar's own persistence (loadAnnotations over the localStorage store the
// toolbar writes), not through a callback: the demo path sets annotation
// state internally, so onAnnotationAdd never sees it.

import { createElement } from 'react';
import { createRoot } from 'react-dom/client';
import { Agentation, identifyElement, getElementPath, loadAnnotations } from 'agentation';

const container = document.createElement('div');
container.id = 'agentation-root';
document.body.appendChild(container);

(window as unknown as Record<string, unknown>).__agentation = {
  identifyElement,
  getElementPath,
  loadAnnotations,
};

createRoot(container).render(
  createElement(Agentation, {
    enableDemoMode: true,
    demoDelay: 100,
    demoAnnotations: [
      { selector: '#demo-target', comment: 'probe annotation from agentation-mount.spec' },
    ],
  })
);
