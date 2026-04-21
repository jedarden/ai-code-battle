// Blog page - displays meta reports and chronicles
import { router } from '../router';

interface BlogEntry {
  slug: string;
  title: string;
  published_at: string;
  type: 'meta-report' | 'chronicle';
  summary: string;
  tags: string[];
}

interface BlogPost extends BlogEntry {
  body_markdown: string;
}

interface BlogIndex {
  updated_at: string;
  posts: BlogEntry[];
}

let cachedIndex: BlogIndex | null = null;

export async function renderBlogPage(): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="blog-page">
      <h1 class="page-title">Blog</h1>
      <p class="page-subtitle">Meta reports, chronicles, and stories from the arena</p>

      <div class="blog-layout">
        <div class="blog-main">
          <div class="blog-filters">
            <button class="filter-btn active" data-filter="all">All</button>
            <button class="filter-btn" data-filter="meta-report">Meta Reports</button>
            <button class="filter-btn" data-filter="chronicle">Chronicles</button>
          </div>

          <div id="blog-list" class="blog-list">
            <div class="loading">Loading posts...</div>
          </div>
        </div>

        <aside class="blog-sidebar">
          <div class="panel">
            <h2>Recent Tags</h2>
            <div id="tag-cloud" class="tag-cloud">
              <span class="loading-text">Loading...</span>
            </div>
          </div>

          <div class="panel">
            <h2>Subscribe</h2>
            <p class="sidebar-text">New posts are published weekly. Check back for the latest meta analysis and stories.</p>
          </div>
        </aside>
      </div>
    </div>

    <style>
      .blog-page .page-title {
        margin-bottom: 8px;
      }

      .blog-page .page-subtitle {
        color: var(--text-muted);
        margin-bottom: 24px;
      }

      .blog-layout {
        display: flex;
        gap: 24px;
      }

      .blog-main {
        flex: 1;
        min-width: 0;
      }

      .blog-filters {
        display: flex;
        gap: 8px;
        margin-bottom: 20px;
      }

      .filter-btn {
        background-color: var(--bg-secondary);
        border: 1px solid var(--border);
        color: var(--text-muted);
        padding: 8px 16px;
        border-radius: 6px;
        cursor: pointer;
        transition: all 0.2s;
      }

      .filter-btn:hover {
        color: var(--text-primary);
        border-color: var(--accent);
      }

      .filter-btn.active {
        background-color: var(--accent);
        color: white;
        border-color: var(--accent);
      }

      .blog-list {
        display: flex;
        flex-direction: column;
        gap: 16px;
      }

      .blog-card {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 20px;
        cursor: pointer;
        transition: transform 0.2s, box-shadow 0.2s;
      }

      .blog-card:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
      }

      .blog-card-meta {
        display: flex;
        align-items: center;
        gap: 12px;
        margin-bottom: 8px;
      }

      .blog-card-type {
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        padding: 4px 8px;
        border-radius: 4px;
        font-weight: 600;
      }

      .blog-card-type.meta-report {
        background-color: rgba(59, 130, 246, 0.2);
        color: #60a5fa;
      }

      .blog-card-type.chronicle {
        background-color: rgba(16, 185, 129, 0.2);
        color: #34d399;
      }

      .blog-card-date {
        color: var(--text-muted);
        font-size: 0.875rem;
      }

      .blog-card-title {
        font-size: 1.25rem;
        color: var(--text-primary);
        margin-bottom: 8px;
      }

      .blog-card-summary {
        color: var(--text-secondary);
        font-size: 0.875rem;
        line-height: 1.5;
      }

      .blog-card-tags {
        display: flex;
        gap: 8px;
        margin-top: 12px;
        flex-wrap: wrap;
      }

      .blog-tag {
        font-size: 0.75rem;
        color: var(--text-muted);
        background-color: var(--bg-tertiary);
        padding: 2px 8px;
        border-radius: 4px;
      }

      .blog-sidebar {
        width: 280px;
        flex-shrink: 0;
      }

      .blog-sidebar .panel {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 16px;
        margin-bottom: 16px;
      }

      .blog-sidebar .panel h2 {
        font-size: 0.875rem;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
        margin-bottom: 12px;
      }

      .tag-cloud {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
      }

      .tag-item {
        font-size: 0.75rem;
        color: var(--text-secondary);
        background-color: var(--bg-tertiary);
        padding: 4px 10px;
        border-radius: 4px;
        cursor: pointer;
        transition: background-color 0.2s;
      }

      .tag-item:hover {
        background-color: var(--accent);
        color: white;
      }

      .sidebar-text {
        color: var(--text-muted);
        font-size: 0.875rem;
        line-height: 1.5;
      }

      .loading, .loading-text {
        color: var(--text-muted);
        text-align: center;
        padding: 40px 20px;
      }

      .empty-state {
        color: var(--text-muted);
        text-align: center;
        padding: 60px 20px;
      }

      @media (max-width: 900px) {
        .blog-layout {
          flex-direction: column;
        }

        .blog-sidebar {
          width: 100%;
        }
      }
    </style>
  `;

  // Load blog index
  await loadBlogIndex();

  // Setup filter handlers
  setupFilterHandlers();
}

async function loadBlogIndex(): Promise<void> {
  const listEl = document.getElementById('blog-list');
  const tagCloudEl = document.getElementById('tag-cloud');

  if (!listEl || !tagCloudEl) return;

  try {
    // Try to fetch from local data directory
    const response = await fetch('data/blog/index.json');
    if (!response.ok) {
      throw new Error('Blog index not found');
    }
    cachedIndex = await response.json() as BlogIndex;

    renderBlogList(cachedIndex.posts);
    renderTagCloud(cachedIndex.posts);
  } catch {
    // Show placeholder if no blog data yet
    listEl.innerHTML = `
      <div class="empty-state">
        <p>No blog posts yet.</p>
        <p style="margin-top: 8px; font-size: 0.875rem;">Weekly meta reports and chronicles will appear here once matches are running.</p>
      </div>
    `;
    tagCloudEl.innerHTML = '<span class="sidebar-text">No tags yet</span>';
  }
}

function renderBlogList(posts: BlogEntry[], filter: string = 'all'): void {
  const listEl = document.getElementById('blog-list');
  if (!listEl) return;

  const filtered = filter === 'all'
    ? posts
    : posts.filter(p => p.type === filter);

  if (filtered.length === 0) {
    listEl.innerHTML = `
      <div class="empty-state">
        <p>No posts found for this filter.</p>
      </div>
    `;
    return;
  }

  listEl.innerHTML = filtered.map(post => `
    <div class="blog-card" data-slug="${post.slug}">
      <div class="blog-card-meta">
        <span class="blog-card-type ${post.type}">${formatPostType(post.type)}</span>
        <span class="blog-card-date">${formatDate(post.published_at)}</span>
      </div>
      <h3 class="blog-card-title">${escapeHtml(post.title)}</h3>
      <p class="blog-card-summary">${escapeHtml(post.summary)}</p>
      <div class="blog-card-tags">
        ${post.tags.slice(0, 4).map(tag => `<span class="blog-tag">${escapeHtml(tag)}</span>`).join('')}
      </div>
    </div>
  `).join('');

  // Add click handlers
  listEl.querySelectorAll('.blog-card').forEach(card => {
    card.addEventListener('click', () => {
      const slug = card.getAttribute('data-slug');
      if (slug) {
        router.navigate(`/blog/${slug}`);
      }
    });
  });
}

function renderTagCloud(posts: BlogEntry[]): void {
  const tagCloudEl = document.getElementById('tag-cloud');
  if (!tagCloudEl) return;

  // Count tag occurrences
  const tagCounts = new Map<string, number>();
  posts.forEach(post => {
    post.tags.forEach(tag => {
      tagCounts.set(tag, (tagCounts.get(tag) || 0) + 1);
    });
  });

  // Sort by count and take top 10
  const sortedTags = Array.from(tagCounts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 10);

  if (sortedTags.length === 0) {
    tagCloudEl.innerHTML = '<span class="sidebar-text">No tags yet</span>';
    return;
  }

  tagCloudEl.innerHTML = sortedTags.map(([tag, count]) =>
    `<span class="tag-item" data-tag="${escapeHtml(tag)}">${escapeHtml(tag)} (${count})</span>`
  ).join('');
}

function setupFilterHandlers(): void {
  const filterBtns = document.querySelectorAll('.filter-btn');

  filterBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      // Update active state
      filterBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');

      // Filter posts
      const filter = btn.getAttribute('data-filter') || 'all';
      if (cachedIndex) {
        renderBlogList(cachedIndex.posts, filter);
      }
    });
  });
}

// Individual blog post page
export async function renderBlogPostPage(params: Record<string, string>): Promise<void> {
  const app = document.getElementById('app');
  if (!app) return;

  const slug = params.slug;
  if (!slug) {
    router.navigate('/blog');
    return;
  }

  app.innerHTML = `
    <div class="blog-post-page">
      <a href="#/blog" class="back-link">← Back to Blog</a>
      <div id="post-content" class="post-content">
        <div class="loading">Loading post...</div>
      </div>
    </div>

    <style>
      .blog-post-page {
        max-width: 800px;
      }

      .back-link {
        display: inline-block;
        color: var(--accent);
        text-decoration: none;
        margin-bottom: 20px;
        font-size: 0.875rem;
      }

      .back-link:hover {
        text-decoration: underline;
      }

      .post-content {
        background-color: var(--bg-secondary);
        border-radius: 8px;
        padding: 32px;
      }

      .post-header {
        margin-bottom: 24px;
        padding-bottom: 20px;
        border-bottom: 1px solid var(--border);
      }

      .post-type-badge {
        display: inline-block;
        font-size: 0.75rem;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        padding: 4px 10px;
        border-radius: 4px;
        font-weight: 600;
        margin-bottom: 12px;
      }

      .post-type-badge.meta-report {
        background-color: rgba(59, 130, 246, 0.2);
        color: #60a5fa;
      }

      .post-type-badge.chronicle {
        background-color: rgba(16, 185, 129, 0.2);
        color: #34d399;
      }

      .post-title {
        font-size: 2rem;
        color: var(--text-primary);
        margin-bottom: 12px;
        line-height: 1.3;
      }

      .post-date {
        color: var(--text-muted);
        font-size: 0.875rem;
      }

      .post-body {
        color: var(--text-secondary);
        line-height: 1.7;
      }

      .post-body h2 {
        color: var(--text-primary);
        margin: 32px 0 16px;
        font-size: 1.5rem;
      }

      .post-body h3 {
        color: var(--text-primary);
        margin: 24px 0 12px;
        font-size: 1.25rem;
      }

      .post-body p {
        margin-bottom: 16px;
      }

      .post-body ul, .post-body ol {
        margin-bottom: 16px;
        padding-left: 24px;
      }

      .post-body li {
        margin-bottom: 8px;
      }

      .post-body table {
        width: 100%;
        border-collapse: collapse;
        margin-bottom: 16px;
      }

      .post-body th, .post-body td {
        padding: 10px 12px;
        text-align: left;
        border-bottom: 1px solid var(--border);
      }

      .post-body th {
        color: var(--text-primary);
        font-weight: 600;
      }

      .post-body hr {
        border: none;
        border-top: 1px solid var(--border);
        margin: 32px 0;
      }

      .post-body code {
        background-color: var(--bg-tertiary);
        padding: 2px 6px;
        border-radius: 4px;
        font-family: 'Fira Code', monospace;
        font-size: 0.875em;
      }

      .post-body strong {
        color: var(--text-primary);
      }

      .post-tags {
        margin-top: 32px;
        padding-top: 20px;
        border-top: 1px solid var(--border);
        display: flex;
        gap: 8px;
        flex-wrap: wrap;
      }

      .post-tag {
        font-size: 0.75rem;
        color: var(--text-muted);
        background-color: var(--bg-tertiary);
        padding: 4px 10px;
        border-radius: 4px;
      }

      .loading {
        color: var(--text-muted);
        text-align: center;
        padding: 60px 20px;
      }

      .not-found {
        color: var(--text-muted);
        text-align: center;
        padding: 60px 20px;
      }
    </style>
  `;

  try {
    const response = await fetch(`data/blog/posts/${slug}.json`);
    if (!response.ok) {
      throw new Error('Post not found');
    }
    const post = await response.json() as BlogPost;
    renderPost(post);
  } catch {
    const contentEl = document.getElementById('post-content');
    if (contentEl) {
      contentEl.innerHTML = `
        <div class="not-found">
          <p>Post not found.</p>
          <a href="#/blog" class="back-link" style="margin-top: 16px; display: inline-block;">← Back to Blog</a>
        </div>
      `;
    }
  }
}

function renderPost(post: BlogPost): void {
  const contentEl = document.getElementById('post-content');
  if (!contentEl) return;

  contentEl.innerHTML = `
    <div class="post-header">
      <span class="post-type-badge ${post.type}">${formatPostType(post.type)}</span>
      <h1 class="post-title">${escapeHtml(post.title)}</h1>
      <div class="post-date">${formatDate(post.published_at)}</div>
    </div>
    <div class="post-body">
      ${markdownToHtml(post.body_markdown)}
    </div>
    <div class="post-tags">
      ${post.tags.map(tag => `<span class="post-tag">${escapeHtml(tag)}</span>`).join('')}
    </div>
  `;
}

function formatPostType(type: string): string {
  switch (type) {
    case 'meta-report':
      return 'Meta Report';
    case 'chronicle':
      return 'Chronicle';
    default:
      return type;
  }
}

function formatDate(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    });
  } catch {
    return dateStr;
  }
}

function escapeHtml(text: string): string {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Simple markdown to HTML converter for basic formatting
function markdownToHtml(md: string): string {
  let html = md;

  // Headers
  html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');

  // Bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');

  // Italic
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>');

  // Code (inline)
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

  // Horizontal rule
  html = html.replace(/^---$/gm, '<hr>');

  // Unordered lists
  html = html.replace(/^- (.+)$/gm, '<li>$1</li>');
  html = html.replace(/(<li>.*<\/li>\n?)+/g, '<ul>$&</ul>');

  // Ordered lists
  html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');

  // Tables (basic)
  const tableRegex = /\|(.+)\|\n\|[-|\s]+\|\n((?:\|.+\|\n?)+)/g;
  html = html.replace(tableRegex, (_, header, body) => {
    const headers = header.split('|').filter((h: string) => h.trim()).map((h: string) => `<th>${h.trim()}</th>`).join('');
    const rows = body.trim().split('\n').map((row: string) => {
      const cells = row.split('|').filter((c: string) => c.trim()).map((c: string) => `<td>${c.trim()}</td>`).join('');
      return `<tr>${cells}</tr>`;
    }).join('');
    return `<table><thead><tr>${headers}</tr></thead><tbody>${rows}</tbody></table>`;
  });

  // Paragraphs (must be last)
  html = html.split('\n\n').map(para => {
    para = para.trim();
    if (!para) return '';
    if (para.startsWith('<')) return para;
    return `<p>${para}</p>`;
  }).join('\n');

  return html;
}
