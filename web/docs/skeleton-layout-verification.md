# Skeleton loading and layout verification

The §16.14 skeleton transition uses a 150ms opacity-only fade. The loading
placeholder is painted before the page data request completes, remains in the
same box while the request is pending, and is replaced by the page content with
the `.fade-in` animation. Network coverage exercises fast (150ms), slow
(2000ms), and disconnected requests for the leaderboard, bot profile, and
replay routes.

Rendered Chromium parity checks compare the skeleton and settled content at
375px, 768px, and 1280px. They cover the page/header regions, content rows or
sections, replay canvas, mobile controls, sidebar, and timeline. The replay
placeholder reserves the `#no-replay` message's 24px line plus its 120px of
vertical padding, so the canvas stack and everything below it remain stationary
when the content arrives.

There are no remaining skeleton-to-content layout-shift issues in these three
pages. A replay may resize its canvas later when an actual map is loaded; that is
post-load visualization sizing, not the skeleton swap measured here.
