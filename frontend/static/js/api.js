// api.js – shared API helpers
// Authentication relies on the HttpOnly cookie set by the server on login.
// The token is NOT stored in localStorage to prevent XSS-based token theft.

const BASE = '';

async function apiFetch(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
  const res = await fetch(BASE + path, { ...opts, headers, credentials: 'same-origin' });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return data;
}

async function getMe() {
  try {
    return await apiFetch('/api/auth/me');
  } catch {
    return null;
  }
}

// Namespace object used by ad.html and other pages
const api = {
  async me() { return getMe(); },
  async get(path) {
    try { return await apiFetch(path); } catch { return null; }
  },
  async post(path, body) {
    try { return await apiFetch(path, { method: 'POST', body: JSON.stringify(body) }); } catch(e) { return { error: e.message }; }
  },
  async put(path, body) {
    try { return await apiFetch(path, { method: 'PUT', body: JSON.stringify(body) }); } catch(e) { return { error: e.message }; }
  },
  async del(path) {
    try { return await apiFetch(path, { method: 'DELETE' }); } catch(e) { return { error: e.message }; }
  },
};
