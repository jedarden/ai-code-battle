// Maps browsing page — view all maps, stats, and vote (§14.6)

import { fetchMapsIndex, submitMapVote, fetchMapVotes, type MapData } from '../api-types';

let mapsData: MapData[] = [];
let mapVotes: Map<string, { my_vote?: number; net_votes: number }> = new Map();

export async function renderMapsPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="maps-page">
      <h1 class="page-title">Maps</h1>
      <p class="page-subtitle">Browse all competitive maps, view engagement scores, and vote on your favorites</p>
      <div id="maps-content" class="loading">Loading maps...</div>
    </div>
  `;

  const content = document.getElementById('maps-content');
  if (!content) return;

  try {
    const data = await fetchMapsIndex();
    mapsData = data.maps;
    renderMapsGrid(content);
  } catch (err) {
    content.innerHTML = `
      <div class="error">
        <p>Failed to load maps.</p>
        <p class="hint">${err instanceof Error ? err.message : 'Unknown error'}</p>
      </div>
    `;
  }
}

function renderMapsGrid(container: HTMLElement): void {
  // Group maps by player count
  const byPlayerCount: Record<number, MapData[]> = {};
  for (const map of mapsData) {
    if (!byPlayerCount[map.player_count]) {
      byPlayerCount[map.player_count] = [];
    }
    byPlayerCount[map.player_count].push(map);
  }

  // Sort each group by engagement (descending)
  for (const count of Object.keys(byPlayerCount)) {
    byPlayerCount[Number(count)].sort((a, b) => b.engagement - a.engagement);
  }

  let html = '';

  for (const [count, maps] of Object.entries(byPlayerCount).sort(([a], [b]) => Number(a) - Number(b))) {
    html += `
      <section class="maps-section">
        <h2 class="maps-section-title">${count}-Player Maps</h2>
        <div class="maps-grid">
    `;

    for (const map of maps) {
      html += renderMapCard(map);
    }

    html += `
        </div>
      </section>
    `;
  }

  container.innerHTML = html;
  attachMapCardListeners();
}

function renderMapCard(map: MapData): string {
  const voteData = mapVotes.get(map.map_id);
  const netVotes = voteData?.net_votes ?? map.net_votes;
  const myVote = voteData?.my_vote;

  return `
    <div class="map-card" data-map-id="${map.map_id}">
      <div class="map-header">
        <span class="map-id">${escapeHtml(map.map_id)}</span>
        <span class="map-players">${map.player_count}P</span>
      </div>

      <div class="map-stats">
        <div class="map-stat">
          <span class="stat-label">Engagement</span>
          <span class="stat-value">${map.engagement.toFixed(1)}</span>
        </div>
        <div class="map-stat">
          <span class="stat-label">Wall Density</span>
          <span class="stat-value">${(map.wall_density * 100).toFixed(0)}%</span>
        </div>
        <div class="map-stat">
          <span class="stat-label">Energy</span>
          <span class="stat-value">${map.energy_count}</span>
        </div>
        <div class="map-stat">
          <span class="stat-label">Size</span>
          <span class="stat-value">${map.grid_width}×${map.grid_height}</span>
        </div>
      </div>

      <div class="map-votes">
        <button class="vote-btn ${myVote === 1 ? 'voted' : ''}" data-vote="1" aria-label="Upvote this map">
          👍
        </button>
        <span class="vote-count">${netVotes > 0 ? '+' : ''}${netVotes}</span>
        <button class="vote-btn ${myVote === -1 ? 'voted' : ''}" data-vote="-1" aria-label="Downvote this map">
          👎
        </button>
      </div>

      <div class="map-status map-status-${map.status}">${map.status}</div>
    </div>
  `;
}

function attachMapCardListeners(): void {
  // Vote buttons
  for (const card of document.querySelectorAll('.map-card')) {
    const mapId = card.getAttribute('data-map-id');
    if (!mapId) continue;

    for (const btn of card.querySelectorAll('.vote-btn')) {
      btn.addEventListener('click', async (e) => {
        e.preventDefault();
        const vote = (e.currentTarget as HTMLElement).getAttribute('data-vote');
        if (!vote) return;

        const voteNum = vote === '1' ? 1 : -1;
        await handleVote(mapId, voteNum, card as HTMLElement);
      });
    }
  }
}

async function handleVote(mapId: string, vote: 1 | -1, card: HTMLElement): Promise<void> {
  const voteData = mapVotes.get(mapId);

  // If clicking the same vote, remove it
  const newVote = voteData?.my_vote === vote ? 0 : vote;

  try {
    // Optimistic update
    const currentNet = voteData?.net_votes ?? (mapsData.find(m => m.map_id === mapId)?.net_votes ?? 0);
    const oldMyVote = voteData?.my_vote ?? 0;

    // Update net votes: subtract old vote, add new vote
    const newNet = currentNet - oldMyVote + newVote;

    mapVotes.set(mapId, { my_vote: newVote === 0 ? undefined : newVote, net_votes: newNet });

    // Re-render just this card
    const map = mapsData.find(m => m.map_id === mapId);
    if (map) {
      card.outerHTML = renderMapCard({ ...map, net_votes: newNet });
      attachMapCardListeners();
    }

    // Submit to API (if not removing vote)
    if (newVote !== 0) {
      await submitMapVote(mapId, newVote);
    } else {
      // To remove a vote, we'd need to vote the opposite direction and then vote again
      // For now, just submit the new vote
      await submitMapVote(mapId, vote);
    }

    // Refresh vote data from server to confirm
    const fresh = await fetchMapVotes(mapId);
    mapVotes.set(mapId, { my_vote: fresh.my_vote, net_votes: fresh.net_votes });

    // Re-render with confirmed data
    const freshMap = mapsData.find(m => m.map_id === mapId);
    if (freshMap) {
      const freshCard = document.querySelector(`[data-map-id="${mapId}"]`);
      if (freshCard) {
        freshCard.outerHTML = renderMapCard({ ...freshMap, net_votes: fresh.net_votes });
        attachMapCardListeners();
      }
    }
  } catch (err) {
    console.error('Failed to submit vote:', err);
    alert('Failed to submit vote. Please try again.');
  }
}

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// ─── Styles ────────────────────────────────────────────────────────────────────────

document.head.insertAdjacentHTML('beforeend', `
<style>
.maps-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-subtitle {
  color: var(--text-muted);
  margin-bottom: 24px;
}

.maps-section {
  margin-bottom: 32px;
}

.maps-section-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 16px;
}

.maps-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.map-card {
  background-color: var(--bg-secondary);
  border-radius: 8px;
  padding: 16px;
  border: 1px solid var(--border);
  transition: transform 0.2s, box-shadow 0.2s;
}

.map-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.map-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.map-id {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.875rem;
}

.map-players {
  background-color: var(--accent);
  color: white;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
}

.map-stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-bottom: 12px;
}

.map-stat {
  display: flex;
  flex-direction: column;
}

.stat-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.stat-value {
  font-size: 0.875rem;
  color: var(--text-primary);
  font-weight: 600;
}

.map-votes {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 0;
  margin-bottom: 8px;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}

.vote-btn {
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.vote-btn:hover {
  background-color: var(--bg-tertiary);
}

.vote-btn.voted {
  background-color: var(--accent);
  color: white;
}

.vote-count {
  font-weight: 600;
  color: var(--text-primary);
  min-width: 40px;
  text-align: center;
}

.map-status {
  font-size: 0.75rem;
  padding: 4px 8px;
  border-radius: 4px;
  text-align: center;
  text-transform: uppercase;
  font-weight: 600;
  letter-spacing: 0.05em;
}

.map-status-active {
  background-color: rgba(34, 197, 94, 0.2);
  color: var(--success);
}

.map-status-retired {
  background-color: rgba(148, 163, 184, 0.2);
  color: var(--text-muted);
}

.map-status-unfair {
  background-color: rgba(239, 68, 68, 0.2);
  color: var(--error);
}

@media (max-width: 640px) {
  .maps-grid {
    grid-template-columns: 1fr;
  }
}
</style>
`);
