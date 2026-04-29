function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/\"/g,'&quot;');
}

function loadProfile(user, cfg = {}) {
  const root = document.getElementById('profile-root');
  const initial = user.username.charAt(0).toUpperCase();
  const teamMode = !!cfg.team_mode;
  root.innerHTML = `
    <div class="row g-4">
      <div class="col-md-4">
        <div class="card p-4 text-center">
          <div class="profile-avatar">${escHtml(initial)}</div>
          <h3 class="mb-1">${escHtml(user.username)}</h3>
          <p class="text-muted mb-1">${escHtml(user.email || '')} &bull; <span class="badge bg-secondary">${escHtml(user.role)}</span></p>
          ${user.affiliation ? `<p class="text-muted small mb-1">${escHtml(user.affiliation)}</p>` : ''}
          ${user.country ? `<p class="text-muted small mb-1">${escHtml(user.country)}</p>` : ''}
          ${user.website ? `<p class="text-muted small mb-2"><a href="${escHtml(user.website)}" target="_blank" rel="noopener">${escHtml(user.website)}</a></p>` : ''}
          <hr style="border-color:var(--ctf-border)">
          <div class="row text-center">
            <div class="col">
              <div style="font-size:2rem;font-weight:800;color:var(--ctf-accent)">${user.score}</div>
              <div class="text-muted small" data-i18n="profile.points">Points</div>
            </div>
          </div>
        </div>

        ${teamMode ? `
        <div class="card mt-3 p-3" id="team-card">
          <h6 class="mb-2" style="color:var(--ctf-muted);text-transform:uppercase;font-size:.75rem;letter-spacing:.06em" data-i18n="profile.team">Team</h6>
          <div id="team-info" class="text-muted small">Loading...</div>
        </div>` : ''}

        <div class="card mt-3 p-3">
          <h6 class="mb-3" style="color:var(--ctf-muted);text-transform:uppercase;font-size:.75rem;letter-spacing:.06em" data-i18n="profile.settings">Settings</h6>

          <form id="profile-update-form" onsubmit="updateProfile(event)">
            <div class="mb-2">
              <label class="form-label small" data-i18n="profile.affiliation">Affiliation</label>
              <input type="text" class="form-control form-control-sm" id="p-affiliation" value="${escHtml(user.affiliation || '')}">
            </div>
            <div class="mb-2">
              <label class="form-label small" data-i18n="profile.website">Website</label>
              <input type="text" class="form-control form-control-sm" id="p-website" value="${escHtml(user.website || '')}">
            </div>
            <div class="mb-2">
              <label class="form-label small" data-i18n="profile.country">Country</label>
              <input type="text" class="form-control form-control-sm" id="p-country" value="${escHtml(user.country || '')}">
            </div>
            <div id="profile-update-msg"></div>
            <button type="submit" class="btn btn-sm btn-primary w-100 mt-1" data-i18n="profile.update">Update Profile</button>
          </form>

          <hr style="border-color:var(--ctf-border)">

          <h6 class="mb-2 small" data-i18n="profile.change_password">Change Password</h6>
          <form id="password-form" onsubmit="changePassword(event)">
            <div class="mb-2">
              <input type="password" class="form-control form-control-sm" id="p-old-pw" data-i18n-placeholder="profile.old_password" placeholder="Current Password" autocomplete="current-password">
            </div>
            <div class="mb-2">
              <input type="password" class="form-control form-control-sm" id="p-new-pw" data-i18n-placeholder="profile.new_password" placeholder="New Password" autocomplete="new-password">
            </div>
            <div class="mb-2">
              <input type="password" class="form-control form-control-sm" id="p-confirm-pw" data-i18n-placeholder="profile.confirm_password" placeholder="Confirm Password" autocomplete="new-password">
            </div>
            <div id="password-msg"></div>
            <button type="submit" class="btn btn-sm btn-secondary w-100 mt-1" data-i18n="profile.save_password">Save Password</button>
          </form>
        </div>
      </div>

      <div class="col-md-8">
        <div class="card">
          <div class="card-header" data-i18n="profile.recent_solves">Recent Solves</div>
          <div id="solves-list" class="p-3">
            <div class="text-muted small">Loading...</div>
          </div>
        </div>
      </div>
    </div>`;

  applyI18n();

  if (teamMode) {
    apiFetch('/api/teams/' + user.id).then(tm => {
      document.getElementById('team-info').innerHTML = `<strong>${escHtml(tm.name)}</strong><br>Score: ${tm.score}`;
    }).catch(() => {
      document.getElementById('team-info').innerHTML =
        `<span class="text-muted" data-i18n="profile.not_in_team">Not in a team.</span><br>
         <button class="btn btn-sm btn-outline-primary mt-2" onclick="showCreateTeam()">${t('profile.create_team')}</button>
         <button class="btn btn-sm btn-secondary mt-2 ms-1" onclick="showJoinTeam()">${t('profile.join_team')}</button>
         <div id="team-form" class="mt-3"></div>`;
    });
  }

  apiFetch('/api/auth/me').then(me => {
    const solvesEl = document.getElementById('solves-list');
    if (!me) { solvesEl.innerHTML = '<p class="text-muted small">-</p>'; return; }
    solvesEl.innerHTML = `<p class="text-muted small">${me.score} points total.</p>`;
  }).catch(() => {});
}

async function updateProfile(e) {
  e.preventDefault();
  const msgEl = document.getElementById('profile-update-msg');
  msgEl.innerHTML = '';
  try {
    await apiFetch('/api/auth/me', {
      method: 'PUT',
      body: JSON.stringify({
        affiliation: document.getElementById('p-affiliation').value,
        website: document.getElementById('p-website').value,
        country: document.getElementById('p-country').value,
      }),
    });
    msgEl.innerHTML = `<div class="alert alert-success small py-1 mt-1">${t('profile.updated_ok')}</div>`;
  } catch (err) {
    msgEl.innerHTML = `<div class="alert alert-danger small py-1 mt-1">${escHtml(err.message)}</div>`;
  }
}

async function changePassword(e) {
  e.preventDefault();
  const msgEl = document.getElementById('password-msg');
  msgEl.innerHTML = '';
  const newPw = document.getElementById('p-new-pw').value;
  const confirmPw = document.getElementById('p-confirm-pw').value;
  if (newPw !== confirmPw) {
    msgEl.innerHTML = `<div class="alert alert-danger small py-1 mt-1">${t('profile.password_mismatch')}</div>`;
    return;
  }
  try {
    await apiFetch('/api/auth/me/password', {
      method: 'PUT',
      body: JSON.stringify({
        old_password: document.getElementById('p-old-pw').value,
        new_password: newPw,
      }),
    });
    msgEl.innerHTML = `<div class="alert alert-success small py-1 mt-1">${t('profile.password_ok')}</div>`;
    document.getElementById('password-form').reset();
  } catch (err) {
    msgEl.innerHTML = `<div class="alert alert-danger small py-1 mt-1">${escHtml(err.message)}</div>`;
  }
}

function showCreateTeam() {
  document.getElementById('team-form').innerHTML = `
    <div class="input-group mb-2">
      <input type="text" class="form-control form-control-sm" id="team-name-input" placeholder="${t('profile.create_team')}">
      <button class="btn btn-sm btn-primary" onclick="createTeam()">${t('profile.create_team')}</button>
    </div>
    <div id="team-msg"></div>`;
}

function showJoinTeam() {
  document.getElementById('team-form').innerHTML = `
    <div class="input-group mb-2">
      <input type="text" class="form-control form-control-sm" id="invite-code-input" placeholder="${t('profile.invite_code')}">
      <button class="btn btn-sm btn-primary" onclick="joinTeam()">${t('profile.join_team')}</button>
    </div>
    <div id="team-msg"></div>`;
}

async function createTeam() {
  const name = document.getElementById('team-name-input').value.trim();
  const msgEl = document.getElementById('team-msg');
  try {
    const tm = await apiFetch('/api/teams', { method: 'POST', body: JSON.stringify({ name }) });
    msgEl.innerHTML = `<div class="alert alert-success small">Team <strong>${escHtml(tm.name)}</strong> created. Invite code: <code>${escHtml(tm.invite_code)}</code></div>`;
  } catch(err) {
    msgEl.innerHTML = `<div class="alert alert-danger small">${escHtml(err.message)}</div>`;
  }
}

async function joinTeam() {
  const invite_code = document.getElementById('invite-code-input').value.trim();
  const msgEl = document.getElementById('team-msg');
  try {
    const tm = await apiFetch('/api/teams/join', { method: 'POST', body: JSON.stringify({ invite_code }) });
    msgEl.innerHTML = `<div class="alert alert-success small">Joined team <strong>${escHtml(tm.name)}</strong>.</div>`;
  } catch(err) {
    msgEl.innerHTML = `<div class="alert alert-danger small">${escHtml(err.message)}</div>`;
  }
}
