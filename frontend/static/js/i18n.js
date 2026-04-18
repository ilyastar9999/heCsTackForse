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
 * Load and apply a custom theme from the server.
 * Supports theme name, CSS variables, custom CSS, theme CSS bundle and custom JS.
 */
async function loadTheme() {
  try {
    const raw = await fetch('/api/theme').then(r => r.ok ? r.json() : null).catch(() => null);
    const theme = normalizeThemeConfig(raw);

    const oldVars = document.getElementById('ctf-theme-override');
    if (oldVars) oldVars.remove();
    const oldCss = document.getElementById('ctf-theme-custom-css');
    if (oldCss) oldCss.remove();
    const oldLink = document.getElementById('ctf-theme-bundle-css');
    if (oldLink) oldLink.remove();
    const oldJs = document.getElementById('ctf-theme-custom-js');
    if (oldJs) oldJs.remove();

    document.documentElement.setAttribute('data-theme', theme.name || 'default');

    if (theme.name && theme.name !== 'default') {
      const link = document.createElement('link');
      link.id = 'ctf-theme-bundle-css';
      link.rel = 'stylesheet';
      link.href = `/static/themes/${encodeURIComponent(theme.name)}/theme.css`;
      document.head.appendChild(link);
    }

    const vars = theme.vars || {};
    if (Object.keys(vars).length > 0) {
      const style = document.createElement('style');
      style.id = 'ctf-theme-override';
      style.textContent = ':root {' + Object.entries(vars).map(([k, v]) => `${k}:${v}`).join(';') + '}';
      document.head.appendChild(style);
    }

    if (theme.custom_css) {
      const customCss = document.createElement('style');
      customCss.id = 'ctf-theme-custom-css';
      customCss.textContent = theme.custom_css;
      document.head.appendChild(customCss);
    }

    if (theme.custom_js) {
      const script = document.createElement('script');
      script.id = 'ctf-theme-custom-js';
      script.text = theme.custom_js;
      document.head.appendChild(script);
    }
  } catch (_) { /* ignore */ }
}

function normalizeThemeConfig(raw) {
  if (!raw || typeof raw !== 'object') {
    return { name: 'default', vars: {}, custom_css: '', custom_js: '' };
  }
  const hasExtendedKeys = ('vars' in raw) || ('name' in raw) || ('custom_css' in raw) || ('custom_js' in raw);
  if (!hasExtendedKeys) {
    return { name: 'default', vars: raw, custom_css: '', custom_js: '' };
  }
  return {
    name: typeof raw.name === 'string' && raw.name ? raw.name : 'default',
    vars: raw.vars && typeof raw.vars === 'object' ? raw.vars : {},
    custom_css: typeof raw.custom_css === 'string' ? raw.custom_css : '',
    custom_js: typeof raw.custom_js === 'string' ? raw.custom_js : '',
  };
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

function formatProfileLabel(username) {
  return `${t('nav.profile')} (${username})`;
}
