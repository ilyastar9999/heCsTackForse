// api.js – shared API helpers

const BASE = '';

async function apiFetch(path, opts = {}) {
  const token = localStorage.getItem('token');
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
  if (token) headers['Authorization'] = 'Bearer ' + token;
  const res = await fetch(BASE + path, { ...opts, headers });
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
