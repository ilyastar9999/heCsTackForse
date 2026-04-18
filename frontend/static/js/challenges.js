// challenges.js

let currentChallenge = null;
let bsModal = null;

async function loadChallenges() {
  const container = document.getElementById('categories');
  try {
    const challenges = await apiFetch('/api/challenges');
    if (!challenges.length) {
      container.innerHTML = '<p class="text-muted">No challenges available yet.</p>';
      return;
    }
    // Group by category
    const cats = {};
    challenges.forEach(c => {
      if (!cats[c.category]) cats[c.category] = [];
      cats[c.category].push(c);
    });
    container.innerHTML = Object.entries(cats).map(([cat, chs]) => `
      <div class="category-header">${escHtml(cat)}</div>
      <div class="challenge-grid mb-4">
        ${chs.map(c => `
          <div class="challenge-card${c.solved ? ' solved' : ''}" onclick="openChallenge(${c.id})">
            ${c.solved ? '<span class="solved-badge">SOLVED</span>' : ''}
            <div class="points">${c.points}</div>
            <div class="ch-name">${escHtml(c.name)}</div>
            <div class="ch-solves">${typeof c.solve_count === 'number' ? c.solve_count : 0} solve${c.solve_count !== 1 ? 's' : ''}</div>
          </div>`).join('')}
      </div>`).join('');
  } catch (err) {
    container.innerHTML = `<div class="alert alert-danger">${err.message}</div>`;
  }
}

async function openChallenge(id) {
  try {
    const c = await apiFetch('/api/challenges/' + id);
    currentChallenge = c;

    document.getElementById('modal-title').textContent = c.name;
    // Render markdown in description
    document.getElementById('modal-desc').innerHTML = window.marked ? marked.parse(c.description || "") : escHtml(c.description || "");
    document.getElementById('modal-solves').textContent = (typeof c.solve_count === 'number' ? c.solve_count : 0) + ' solve' + (c.solve_count !== 1 ? 's' : '');

    // Meta badges
    document.getElementById('modal-meta').innerHTML = `
      <span class="tag">${escHtml(c.category)}</span>
      <span class="tag" style="color:var(--ctf-yellow);border-color:rgba(255,193,7,.3);background:rgba(255,193,7,.1)">${c.points} pts</span>
      ${c.flag_type === 'regex' ? '<span class="tag" style="color:var(--ctf-blue)">regex flag</span>' : ''}
      ${c.deploy_type !== 'no_deploy' ? `<span class="tag" style="color:var(--ctf-blue)">${escHtml(c.deploy_type)}</span>` : ''}
    `;

    // Instance management
    const instBox = document.getElementById('modal-instance');
    if (c.deploy_type !== 'no_deploy') {
      instBox.classList.remove('d-none');
      instBox.innerHTML = '<div class="instance-box">Checking instance status…</div>';
      loadInstanceStatus(c.id);
    } else {
      instBox.classList.add('d-none');
    }

    document.getElementById('flag-error').classList.add('d-none');
    document.getElementById('flag-success').classList.add('d-none');
    document.getElementById('flag-input').value = '';

    const solvedNotice = document.getElementById('modal-solved-notice');
    const submitBtn = document.getElementById('submit-btn');
    if (c.solved) {
      solvedNotice.classList.remove('d-none');
      submitBtn.disabled = true;
    } else {
      solvedNotice.classList.add('d-none');
      submitBtn.disabled = false;
    }

    if (!bsModal) bsModal = new bootstrap.Modal(document.getElementById('challengeModal'));
    bsModal.show();

    // Load hints and files asynchronously
    loadChallengeExtras(c.id, c);
  } catch (err) {
    alert(err.message);
  }
}

async function loadChallengeExtras(id, c) {
  // Connection info
  const connSection = document.getElementById('modal-connection');
  const connInfo    = document.getElementById('modal-connection-info');
  if (c.connection_info) {
    connInfo.textContent = c.connection_info;
    connSection.classList.remove('d-none');
  } else {
    connSection.classList.add('d-none');
  }

  // Files
  const filesSection = document.getElementById('modal-files-section');
  const filesList    = document.getElementById('modal-files-list');
  try {
    const files = await apiFetch('/api/challenges/' + id + '/files');
    if (files && files.length) {
      filesList.innerHTML = files.map(f => `
        <div class="file-item">
          <span class="file-icon">📄</span>
          <a href="${escHtml(f.url || f.path || '')}" target="_blank" rel="noopener noreferrer">${escHtml(f.name || f.filename || f.url || 'File')}</a>
        </div>`).join('');
      filesSection.classList.remove('d-none');
    } else {
      filesSection.classList.add('d-none');
    }
  } catch (_) {
    filesSection.classList.add('d-none');
  }

  // Hints
  const hintsSection = document.getElementById('modal-hints-section');
  const hintsList    = document.getElementById('modal-hints-list');
  try {
    const hints = await apiFetch('/api/challenges/' + id + '/hints');
    if (hints && hints.length) {
      hintsList.innerHTML = hints.map(h => `
        <div class="hint-item">
          <span class="hint-cost">${h.cost > 0 ? h.cost + ' pts' : t('challenges.hint_free')}</span>
          <span class="ms-2 text-muted">${h.content !== undefined ? escHtml(h.content) : 'Unlock to reveal'}</span>
        </div>`).join('');
      hintsSection.classList.remove('d-none');
    } else {
      hintsSection.classList.add('d-none');
    }
  } catch (_) {
    hintsSection.classList.add('d-none');
  }

async function loadInstanceStatus(challengeId) {
  const instBox = document.getElementById('modal-instance');
  try {
    const inst = await apiFetch('/api/challenges/' + challengeId + '/instance');
    if (inst && inst.status === 'running') {
      const info = JSON.parse(inst.connection_info || '{}');
      const conn = Object.entries(info).map(([k,v]) => `<b>${escHtml(k)}:</b> <code>${escHtml(String(v))}</code>`).join(' &nbsp;|&nbsp; ');
      instBox.innerHTML = `
        <div class="instance-box">
          <div class="d-flex justify-content-between align-items-start">
            <div><strong>Instance running</strong><br>${conn || 'No connection info'}</div>
            <button class="btn btn-sm btn-danger ms-3" onclick="stopInstance(${challengeId})">Stop</button>
          </div>
          ${inst.expires_at ? `<div class="text-muted small mt-1">Expires: ${new Date(inst.expires_at).toLocaleString()}</div>` : ''}
        </div>`;
    } else {
      showStartButton(instBox, challengeId);
    }
  } catch {
    showStartButton(instBox, challengeId);
  }
}

function showStartButton(instBox, challengeId) {
  instBox.innerHTML = `
    <div class="d-flex align-items-center gap-2">
      <button class="btn btn-sm btn-secondary" onclick="startInstance(${challengeId})">Start Instance</button>
      <span class="text-muted small">No running instance</span>
    </div>`;
}

async function startInstance(challengeId) {
  const instBox = document.getElementById('modal-instance');
  instBox.innerHTML = '<div class="instance-box">Starting instance…</div>';
  try {
    await apiFetch('/api/challenges/' + challengeId + '/instance', { method: 'POST' });
    await loadInstanceStatus(challengeId);
  } catch(err) {
    instBox.innerHTML = `<div class="alert alert-danger">${err.message}</div>`;
  }
}

async function stopInstance(challengeId) {
  const instBox = document.getElementById('modal-instance');
  instBox.innerHTML = '<div class="instance-box">Stopping…</div>';
  try {
    await apiFetch('/api/challenges/' + challengeId + '/instance', { method: 'DELETE' });
    showStartButton(instBox, challengeId);
  } catch(err) {
    instBox.innerHTML = `<div class="alert alert-danger">${err.message}</div>`;
  }
}

async function submitFlag(e) {
  e.preventDefault();
  if (!currentChallenge) return;
  const flag = document.getElementById('flag-input').value.trim();
  const errEl = document.getElementById('flag-error');
  const okEl = document.getElementById('flag-success');
  errEl.classList.add('d-none');
  okEl.classList.add('d-none');
  try {
    const res = await apiFetch('/api/challenges/' + currentChallenge.id + '/submit', {
      method: 'POST',
      body: JSON.stringify({ flag }),
    });
    if (res.correct) {
      okEl.textContent = '🎉 Correct! +' + res.points + ' points';
      okEl.classList.remove('d-none');
      document.getElementById('submit-btn').disabled = true;
      document.getElementById('modal-solved-notice').classList.remove('d-none');
      loadChallenges();
    } else {
      errEl.textContent = '✗ Incorrect flag. Try again.';
      errEl.classList.remove('d-none');
    }
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('d-none');
  }
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
