// Registration page - form to register a new bot

import { registerBot, type RegisterResponse } from '../api-types';

interface FormState {
  submitting: boolean;
  result: RegisterResponse | null;
  error: string | null;
}

let state: FormState = {
  submitting: false,
  result: null,
  error: null,
};

export function renderRegisterPage(): void {
  const app = document.getElementById('app');
  if (!app) return;

  app.innerHTML = `
    <div class="register-page">
      <h1>Register Your Bot</h1>

      <div class="register-intro">
        <p>
          Register your bot to compete on the ladder. Your bot will be matched
          against other registered bots automatically.
        </p>
        <p>
          You'll receive an API key after registration. Keep it safe - you'll
          need it to manage your bot.
        </p>
      </div>

      <div id="register-form-container"></div>

      <div class="register-help">
        <h2>Requirements</h2>
        <ul>
          <li>Your bot must expose an HTTPS endpoint accessible from the internet</li>
          <li>The endpoint must respond to POST requests with game state JSON</li>
          <li>Response time must be under 3 seconds per turn</li>
          <li>See the <a href="#/compete/docs">Getting Started guide</a> for protocol details</li>
        </ul>
      </div>
    </div>
  `;

  renderForm();
}

function renderForm(): void {
  const container = document.getElementById('register-form-container');
  if (!container) return;

  if (state.result?.success) {
    container.innerHTML = `
      <div class="register-success">
        <h2>Registration Successful!</h2>
        <p>Your bot has been registered and will begin competing shortly.</p>

        <div class="api-key-display">
          <h3>Your API Key</h3>
          <p class="warning">Save this key now - it won't be shown again!</p>
          <code class="api-key">${escapeHtml(state.result.api_key || '')}</code>
          <button class="btn secondary" onclick="navigator.clipboard.writeText('${escapeHtml(state.result.api_key || '')}').then(() => alert('Copied!'))">
            Copy to Clipboard
          </button>
        </div>

        <div class="bot-info">
          <p><strong>Bot ID:</strong> ${escapeHtml(state.result.bot_id || '')}</p>
        </div>

        <div class="next-steps">
          <a href="#/bot/${encodeURIComponent(state.result.bot_id || '')}" class="btn primary">View Bot Profile</a>
          <a href="#/leaderboard" class="btn secondary">View Leaderboard</a>
        </div>
      </div>
    `;
    return;
  }

  container.innerHTML = `
    <form id="register-form" class="register-form">
      ${state.error ? `<div class="error-message">${escapeHtml(state.error)}</div>` : ''}

      <div class="form-group">
        <label for="bot-name">Bot Name</label>
        <input
          type="text"
          id="bot-name"
          name="name"
          placeholder="MyAwesomeBot"
          required
          pattern="[a-zA-Z0-9_-]+"
          minlength="3"
          maxlength="32"
          ${state.submitting ? 'disabled' : ''}
        >
        <span class="hint">3-32 characters, alphanumeric, dash, or underscore</span>
      </div>

      <div class="form-group">
        <label for="endpoint-url">Endpoint URL</label>
        <input
          type="url"
          id="endpoint-url"
          name="endpoint_url"
          placeholder="https://my-bot.example.com/move"
          required
          ${state.submitting ? 'disabled' : ''}
        >
        <span class="hint">HTTPS URL where your bot receives move requests</span>
      </div>

      <div class="form-group">
        <label for="owner-id">Owner ID</label>
        <input
          type="text"
          id="owner-id"
          name="owner_id"
          placeholder="your-email@example.com"
          required
          maxlength="64"
          ${state.submitting ? 'disabled' : ''}
        >
        <span class="hint">Your identifier for account management</span>
      </div>

      <button type="submit" class="btn primary" ${state.submitting ? 'disabled' : ''}>
        ${state.submitting ? 'Registering...' : 'Register Bot'}
      </button>
    </form>
  `;

  const form = document.getElementById('register-form') as HTMLFormElement;
  if (form) {
    form.addEventListener('submit', handleSubmit);
  }
}

async function handleSubmit(e: Event): Promise<void> {
  e.preventDefault();

  const form = e.target as HTMLFormElement;
  const formData = new FormData(form);

  const name = formData.get('name') as string;
  const endpointUrl = formData.get('endpoint_url') as string;
  const ownerId = formData.get('owner_id') as string;

  state = {
    ...state,
    submitting: true,
    error: null,
  };
  renderForm();

  try {
    const result = await registerBot({
      name,
      endpoint_url: endpointUrl,
      owner_id: ownerId,
    });

    state = {
      ...state,
      submitting: false,
      result,
      error: result.success ? null : result.error || 'Registration failed',
    };
    renderForm();
  } catch (err) {
    state = {
      ...state,
      submitting: false,
      error: `Network error: ${err}`,
    };
    renderForm();
  }
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
