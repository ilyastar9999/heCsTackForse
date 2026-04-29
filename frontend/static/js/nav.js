let hecstackPublicConfigPromise = null;

async function getPublicConfig() {
  if (!hecstackPublicConfigPromise) {
    hecstackPublicConfigPromise = apiFetch('/api/config').catch(() => ({}));
  }
  return hecstackPublicConfigPromise;
}

async function applyModeAwareNav() {
  const cfg = await getPublicConfig();
  const notificationsItem = document.getElementById('nav-notifications-li');
  if (notificationsItem) notificationsItem.style.display = '';
  return cfg;
}
