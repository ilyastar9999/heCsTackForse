// profile.js

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function loadProfile(user) {
  const root = document.getElementById('profile-root');
  const initial = user.username.charAt(0).toUpperCase();
  root.innerHTML = `
    <div class="row g-4">
      <div class="col-md-4">
        <div class="card p-4 text-center">
          <div class="profile-avatar">${escHtml(initial)}</div>
          <h3 class="mb-1">${escHtml(user.username)}</h3>
          <p class="text-muted mb-3">${escHtml(user.email || '')} &bull; <span class="badge bg-secondary">${escHtml(user.role)}</span></p>
          <hr style="border-color:var(--ctf-border)">
          <div class="row text-center">
            <div class="col">
              <div style="font-size:2rem;font-weight:800;color:var(--ctf-accent)">${user.score}</div>
              <div class="text-muted small">Points</div>
            </div>
          </div>
        </div>
        <div class="card mt-3 p-3" id="team-card">
          <h6 class="mb-2" style="color:var(--ctf-muted);text-transform:uppercase;font-size:.75rem;letter-spacing:.06em">Team</h6>
          <div id="team-info" class="text-muted small">Loading…</div>
        </div>
      </div>
      <div class="col-md-8">
        <div class="card">
          <div class="card-header">Recent Solves</div>
          <div id="solves-list" class="p-3">
            <div class="text-muted small">Loading…</div>
          </div>
        </div>
      </div>
    </div>`;

  // Load team
  apiFetch('/api/teams/' + user.id).then(t => {
    document.getElementById('team-info').innerHTML = `<strong>${escHtml(t.name)}</strong><br>Score: ${t.score}`;
  }).catch(() => {
    document.getElementById('team-info').innerHTML =
      `<span class="text-muted">Not in a team.</span><br>
       <button class="btn btn-sm btn-outline-primary mt-2" onclick="showCreateTeam()">Create Team</button>
       <button class="btn btn-sm btn-secondary mt-2 ms-1" onclick="showJoinTeam()">Join Team</button>
       <div id="team-form" class="mt-3"></div>`;
  });
}

function showCreateTeam() {
  document.getElementById('team-form').innerHTML = `
    <div class="input-group mb-2">
      <input type="text" class="form-control" id="team-name-input" placeholder="Team name">
      <button class="btn btn-primary" onclick="createTeam()">Create</button>
    </div>
    <div id="team-msg"></div>`;
}

function showJoinTeam() {
  document.getElementById('team-form').innerHTML = `
    <div class="input-group mb-2">
      <input type="text" class="form-control" id="invite-code-input" placeholder="Invite code">
      <button class="btn btn-primary" onclick="joinTeam()">Join</button>
    </div>
    <div id="team-msg"></div>`;
}

async function createTeam() {
  const name = document.getElementById('team-name-input').value.trim();
  const msgEl = document.getElementById('team-msg');
  try {
    const t = await apiFetch('/api/teams', { method: 'POST', body: JSON.stringify({ name }) });
    msgEl.innerHTML = `<div class="alert alert-success small">Team <strong>${escHtml(t.name)}</strong> created! Invite code: <code>${escHtml(t.invite_code)}</code></div>`;
  } catch(err) {
    msgEl.innerHTML = `<div class="alert alert-danger small">${escHtml(err.message)}</div>`;
  }
}

async function joinTeam() {
  const invite_code = document.getElementById('invite-code-input').value.trim();
  const msgEl = document.getElementById('team-msg');
  try {
    const t = await apiFetch('/api/teams/join', { method: 'POST', body: JSON.stringify({ invite_code }) });
    msgEl.innerHTML = `<div class="alert alert-success small">Joined team <strong>${escHtml(t.name)}</strong>!</div>`;
  } catch(err) {
    msgEl.innerHTML = `<div class="alert alert-danger small">${escHtml(err.message)}</div>`;
  }
}
