// i18n.js – Internationalisation helper for heCsTackForse
//
// Usage:
//   await initI18n();          // call once at page load
//   t('nav.challenges')        // look up a translation key
//   applyI18n()                // apply data-i18n attributes to the DOM
//   setLang('ru')              // switch language at runtime

let _locale = {};
let _lang = localStorage.getItem('ctf_lang') || 'en';

/**
 * Load a locale JSON from /static/locales/{lang}.json.
 * Falls back silently if the file is not found.
 */
async function loadLocale(lang) {
  try {
    const res = await fetch(`/static/locales/${lang}.json`);
    if (res.ok) {
      _locale = await res.json();
      _lang = lang;
      localStorage.setItem('ctf_lang', lang);
    }
  } catch (_) {
    // network error — keep existing locale
  }
}

/**
 * Initialise i18n: load the server-configured default language if none is
 * stored in localStorage, then apply translations to the current DOM.
 */
async function initI18n() {
  // Respect server-configured default language when user has no preference.
  if (!localStorage.getItem('ctf_lang')) {
    try {
      const cfg = await fetch('/api/config').then(r => r.json()).catch(() => ({}));
      if (cfg.language && cfg.language !== 'en') {
        _lang = cfg.language;
      }
    } catch (_) {}
  }
  await loadLocale(_lang);
  applyI18n();
  updateLangToggle();
  loadTheme();
}

/**
 * Load and apply a custom CSS theme from the server.
 * Injects a <style> element with CSS variable overrides into <head>.
 */
async function loadTheme() {
  try {
    const vars = await fetch('/api/theme').then(r => r.ok ? r.json() : null).catch(() => null);
    if (vars && typeof vars === 'object' && Object.keys(vars).length > 0) {
      const style = document.createElement('style');
      style.id = 'ctf-theme-override';
      style.textContent = ':root {' + Object.entries(vars).map(([k, v]) => `${k}:${v}`).join(';') + '}';
      const existing = document.getElementById('ctf-theme-override');
      if (existing) existing.remove();
      document.head.appendChild(style);
    }
  } catch (_) { /* ignore */ }
}

/**
 * Translate a key. Supports {placeholder} substitutions.
 * @param {string} key
 * @param {Object} [vars] – e.g. { n: 42 }
 * @returns {string}
 */
function t(key, vars) {
  let str = _locale[key] !== undefined ? _locale[key] : key;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      str = str.replace('{' + k + '}', String(v));
    }
  }
  return str;
}

/**
 * Walk the DOM and set textContent for every element with a data-i18n attribute,
 * and set placeholder for every element with data-i18n-placeholder.
 */
function applyI18n() {
  document.querySelectorAll('[data-i18n]').forEach(el => {
    el.textContent = t(el.getAttribute('data-i18n'));
  });
  document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
    el.placeholder = t(el.getAttribute('data-i18n-placeholder'));
  });
}

/**
 * Switch language at runtime, reload the locale and re-apply to the DOM.
 */
async function setLang(lang) {
  await loadLocale(lang);
  applyI18n();
  updateLangToggle();
}

/** Update the navbar language toggle button label. */
function updateLangToggle() {
  const el = document.getElementById('lang-toggle-label');
  if (el) el.textContent = _lang.toUpperCase();
}
