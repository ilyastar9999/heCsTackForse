// admin-user.js – Admin user detail/edit page

let _userId = null;
let _userFields = [];
let _userFieldValues = {};

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

// Extract user ID from URL path: /admin/users/123
function getUserIdFromPath() {
  const parts = window.location.pathname.split('/').filter(Boolean);
  return parts[parts.length - 1];
}

async function initAdminUser() {
  _userId = getUserIdFromPath();
  if (!_userId || isNaN(Number(_userId))) {
    document.getElementById('user-loading').innerHTML =
      '<div class="alert alert-danger">Invalid user ID in URL.</div>';
    return;
  }

  try {
    const [user, fields] = await Promise.all([
      apiFetch('/api/admin/users/' + _userId),
      apiFetch('/api/admin/userfields').catch(() => []),
    ]);

    _userFields = Array.isArray(fields) ? fields : [];

    // Populate form
    document.getElementById('u-username').value   = user.username   || '';
    document.getElementById('u-email').value      = user.email      || '';
    document.getElementById('u-language').value   = user.language   || '';
    document.getElementById('u-role').value       = user.role       || 'user';
    document.getElementById('u-score').value      = user.score      ?? 0;
    document.getElementById('u-affiliation').value= user.affiliation|| '';
    document.getElementById('u-website').value    = user.website    || '';
    document.getElementById('u-country').value    = user.country    || '';
    document.getElementById('u-verified').checked = !!user.verified;
    document.getElementById('u-hidden').checked   = !!user.hidden;
    document.getElementById('u-banned').checked   = !!user.banned;

    document.title = 'Edit: ' + user.username + ' – heCsTackForse';

    // Custom fields
    if (_userFields.length > 0) {
      const section = document.getElementById('custom-fields-section');
      section.style.display = '';
      const fieldValues = user.field_values || {};
      _userFieldValues = fieldValues;
      renderCustomFields(fieldValues);
    }

    // Submissions
    renderSubmissions(user.submissions || []);

    document.getElementById('user-loading').style.display = 'none';
    document.getElementById('user-content').style.display = '';
  } catch (err) {
    document.getElementById('user-loading').innerHTML =
      `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

function renderCustomFields(values) {
  const container = document.getElementById('custom-fields-list');
  container.innerHTML = _userFields.map(f => `
    <div class="mb-2">
      <label class="form-label">${escHtml(f.name)}</label>
      <input type="text" class="form-control" id="cf-${f.id}"
        value="${escHtml(values[f.id] || values[String(f.id)] || '')}">
    </div>
  `).join('');
}

async function saveUser(e) {
  e.preventDefault();
  const errEl = document.getElementById('user-error');
  const okEl  = document.getElementById('user-success');
  errEl.classList.add('d-none');
  okEl.classList.add('d-none');

  const body = {
    username:    document.getElementById('u-username').value,
    email:       document.getElementById('u-email').value,
    language:    document.getElementById('u-language').value,
    role:        document.getElementById('u-role').value,
    score:       parseInt(document.getElementById('u-score').value) || 0,
    affiliation: document.getElementById('u-affiliation').value,
    website:     document.getElementById('u-website').value,
    country:     document.getElementById('u-country').value,
    verified:    document.getElementById('u-verified').checked,
    hidden:      document.getElementById('u-hidden').checked,
    banned:      document.getElementById('u-banned').checked,
  };

  const password = document.getElementById('u-password').value;
  if (password) body.password = password;

  try {
    await apiFetch('/api/admin/users/' + _userId, { method: 'PUT', body: JSON.stringify(body) });
    okEl.textContent = 'User saved successfully.';
    okEl.classList.remove('d-none');
    document.getElementById('u-password').value = '';
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('d-none');
  }
}

async function saveCustomFields() {
  for (const f of _userFields) {
    const el = document.getElementById('cf-' + f.id);
    if (!el) continue;
    try {
      await apiFetch('/api/admin/users/' + _userId + '/fields/' + f.id, {
        method: 'PUT',
        body: JSON.stringify({ value: el.value }),
      });
    } catch (_) { /* best-effort */ }
  }
  alert('Custom fields saved.');
}

async function doResetScore() {
  if (!confirm('Reset this user\'s score and delete all their submissions?')) return;
  try {
    await apiFetch('/api/admin/users/' + _userId + '/reset_score', { method: 'POST' });
    document.getElementById('u-score').value = 0;
    document.getElementById('submissions-list').innerHTML = '<span class="text-muted">No submissions.</span>';
    alert('Score reset.');
  } catch (err) {
    alert('Error: ' + err.message);
  }
}

async function doDeleteUser() {
  if (!confirm('Permanently delete this user? This cannot be undone.')) return;
  try {
    await apiFetch('/api/admin/users/' + _userId, { method: 'DELETE' });
    location.href = '/admin';
  } catch (err) {
    alert('Error: ' + err.message);
  }
}

function renderSubmissions(submissions) {
  const el = document.getElementById('submissions-list');
  if (!submissions || !submissions.length) {
    el.innerHTML = '<span class="text-muted">No submissions.</span>';
    return;
  }
  el.innerHTML = submissions.map(s => `
    <div class="d-flex justify-content-between align-items-center py-1 border-bottom" style="border-color:var(--ctf-border)!important">
      <div>
        <span class="${s.is_correct ? 'text-success' : 'text-danger'}">${s.is_correct ? '✓' : '✗'}</span>
        <span class="ms-1">${escHtml(s.challenge_name || 'Challenge #' + s.challenge_id)}</span>
      </div>
      <small class="text-muted">${new Date(s.submitted_at || s.created_at).toLocaleDateString()}</small>
    </div>
  `).join('');
}
