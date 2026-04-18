// notifications.js

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

async function loadNotifications() {
  const container = document.getElementById('notifications-list');
  try {
    const notifications = await apiFetch('/api/notifications');
    if (!notifications || !notifications.length) {
      container.innerHTML = `<p class="text-muted text-center py-5" data-i18n="notifications.empty">${t('notifications.empty')}</p>`;
      return;
    }
    // Sort newest first
    const sorted = [...notifications].sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
    container.innerHTML = sorted.map(n => `
      <div class="notification-card">
        <div class="notif-title">${escHtml(n.title)}</div>
        <div class="notif-date">${new Date(n.created_at).toLocaleString()}</div>
        <div class="notif-content">${escHtml(n.content)}</div>
      </div>
    `).join('');
  } catch (err) {
    container.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}
