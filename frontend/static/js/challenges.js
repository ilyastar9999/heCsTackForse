let currentChallenge = null;
let bsModal = null;
let challengeSploitResultsModal = null;
let instanceTTLLabel = '4h';

function byId(id) {
  return document.getElementById(id);
}

function hideEl(el) {
  if (el) el.classList.add('d-none');
}

function showEl(el) {
  if (el) el.classList.remove('d-none');
}


function normalizeDeployType(value, challengeType = '') {
  let normalized = String(value || '').trim().toLowerCase();
  if (!normalized || normalized === 'none') normalized = 'no_deploy';
  if (normalized === 'single_instance' || normalized === 'attack_defence' || normalized === 'shared' || normalized === 'shared_service') normalized = 'always_on';
  if (normalized === 'per_user' || normalized === 'per_team' || normalized === 'instance' || normalized === 'per_instance_deploy') normalized = 'per_instance';
  if (challengeType === 'dynamic_deploy' && normalized === 'no_deploy') normalized = 'per_instance';
  if ((challengeType === 'pentest' || challengeType === 'attack_defence_attack' || challengeType === 'attack_defence_defense') && normalized === 'no_deploy') normalized = 'always_on';
  return normalized;
}

function deployTypeLabel(value, challengeType = '') {
  const normalized = normalizeDeployType(value, challengeType);
  const labels = {
    no_deploy: 'No Deploy',
    per_instance: 'Per Instance Deploy',
    always_on: 'Always On Service',
  };
  return labels[normalized] || normalized;
}

function workflowBox() {
  return byId('modal-workflow');
}

function workflowHeading(title, body = '') {
  return `
    <div class="workflow-hero">
      <div class="d-flex justify-content-between align-items-start gap-3 flex-wrap">
        <div>
          <h6 class="mb-1">${escHtml(title)}</h6>
          ${body ? `<div class="text-muted small">${body}</div>` : ''}
        </div>
      </div>
    </div>`;
}

function workflowPanel(title, body, extraClass = '') {
  return `
    <section class="workflow-panel ${extraClass}">
      ${title ? `<div class="workflow-panel-title">${escHtml(title)}</div>` : ''}
      ${body}
    </section>`;
}

function renderWorkflowNotes(notes) {
  if (!Array.isArray(notes) || !notes.length) return '';
  return `<ul class="small text-muted mb-0 ps-3">${notes.map(note => `<li>${escHtml(note)}</li>`).join('')}</ul>`;
}

function renderVPNSummary(vpn, downloadURL, syncURL = '') {
  if (!vpn) return '';
  const statusColor = vpn.provisioned ? 'var(--ctf-green)' : 'var(--ctf-yellow)';
  return `
    <div class="workflow-panel vpn-panel">
      <div class="workflow-panel-title">VPN Access</div>
      <div class="small text-muted mb-2">Import the generated WireGuard profile, then reach the game network through the routed subnet below.</div>
      <div class="workflow-metric-grid">
        <div><span>Status</span><strong style="color:${statusColor}">${vpn.provisioned ? 'provisioned' : 'pending sync'}</strong></div>
        <div><span>Owner</span><strong>${escHtml(vpn.owner_type)} #${vpn.owner_id}</strong></div>
        <div><span>Client</span><strong>${escHtml(vpn.client_address)}</strong></div>
        <div><span>Allowed subnet</span><strong>${escHtml(vpn.allowed_subnet)}</strong></div>
        <div><span>Endpoint</span><strong>${escHtml(vpn.server_endpoint)}</strong></div>
        <div><span>Routes</span><strong>${escHtml(vpn.game_net_cidr)}</strong></div>
        ${vpn.server_ip ? `<div><span>Server IP</span><strong>${escHtml(vpn.server_ip)}</strong></div>` : ''}
        ${vpn.dns ? `<div><span>DNS</span><strong>${escHtml(vpn.dns)}</strong></div>` : ''}
      </div>
      ${vpn.last_sync_error ? `<div class="alert alert-danger mt-3 mb-0">${escHtml(vpn.last_sync_error)}</div>` : ''}
      <div class="mt-3 d-flex gap-2 flex-wrap">
        ${downloadURL ? `<button class="btn btn-sm btn-secondary" onclick="downloadWorkflowVPN('${escHtml(downloadURL)}')">Download VPN Config</button>` : ''}
        ${syncURL ? `<button class="btn btn-sm btn-outline-primary" onclick="syncWorkflowVPN('${escHtml(syncURL)}')">Sync VPN Access</button>` : ''}
      </div>
      <div id="challenge-vpn-msg" class="small mt-2"></div>
    </div>`;
}

function renderBoardVPNStatus(vpn) {
  if (!vpn) return '<div class="text-muted small">VPN is disabled for this board.</div>';
  const statusColor = vpn.provisioned ? 'var(--ctf-green)' : 'var(--ctf-yellow)';
  return `
    <p class="text-muted small mb-3">This WireGuard profile is shared across all VPN-backed challenges on the board.</p>
    <div class="workflow-metric-grid compact">
      <div><span>Status</span><strong style="color:${statusColor}">${vpn.provisioned ? 'provisioned' : 'pending sync'}</strong></div>
      <div><span>Owner</span><strong>${escHtml(vpn.owner_type)} #${vpn.owner_id}</strong></div>
      <div><span>Client</span><strong>${escHtml(vpn.client_address)}</strong></div>
      <div><span>Routes</span><strong>${escHtml(vpn.game_net_cidr || vpn.allowed_subnet || '')}</strong></div>
    </div>
    ${vpn.last_sync_error ? `<div class="alert alert-danger py-2 mt-3 mb-0">${escHtml(vpn.last_sync_error)}</div>` : ''}
    <div class="d-flex gap-2 flex-wrap mt-3">
      <button class="btn btn-sm btn-secondary" onclick="downloadBoardVPN()">Download config</button>
      <button class="btn btn-sm btn-outline-primary" onclick="syncBoardVPN()">Sync access</button>
    </div>
    <div id="board-vpn-msg" class="small mt-2"></div>`;
}

function renderServiceStatus(serviceStatus) {
  if (!serviceStatus) {
    return '<div class="text-muted small">No checker rounds recorded for this service yet.</div>';
  }
  const scoreLabel = serviceStatus.max_score > 0 ? `${serviceStatus.score}/${serviceStatus.max_score}` : `${serviceStatus.score}`;
  return `
    <div class="d-flex flex-wrap align-items-center gap-2 mb-2">
      <span class="tag">Round ${serviceStatus.round}</span>
      <span class="tag" style="color:${serviceStatus.status === 'up' ? 'var(--ctf-green)' : serviceStatus.status === 'corrupt' ? 'var(--ctf-yellow)' : 'var(--ctf-red)'}">${escHtml(serviceStatus.status)}</span>
      <span class="tag">Score ${escHtml(scoreLabel)}</span>
    </div>
    <div class="text-muted small">Last check: ${new Date(serviceStatus.checked_at).toLocaleString()}</div>`;
}

function renderServiceRounds(rounds) {
  if (!Array.isArray(rounds) || !rounds.length) return '';
  return `
    <div class="mt-3">
      <div class="small fw-semibold mb-2">Recent rounds</div>
      <div class="table-responsive">
        <table class="admin-table mb-0">
          <thead><tr><th>Round</th><th>Status</th><th>Score</th><th>Checked</th></tr></thead>
          <tbody>
            ${rounds.map(row => {
              const scoreLabel = row.max_score > 0 ? `${row.score}/${row.max_score}` : `${row.score}`;
              return `<tr>
                <td>${row.round}</td>
                <td>${escHtml(row.status)}</td>
                <td>${escHtml(scoreLabel)}</td>
                <td>${new Date(row.checked_at).toLocaleString()}</td>
              </tr>`;
            }).join('')}
          </tbody>
        </table>
      </div>
    </div>`;
}

function normalizeBindHost(host) {
  const value = String(host || '').trim();
  if (!value || value === '0.0.0.0' || value === '::') return location.hostname || '127.0.0.1';
  return value;
}

function instanceEndpoint(connectionInfo) {
  let info = {};
  try {
    info = JSON.parse(connectionInfo || '{}') || {};
  } catch (_) {
    info = {};
  }
  if (info.ip && info.port) return { ip: normalizeBindHost(info.ip), port: info.port };
  if (info.host && info.port) return { ip: normalizeBindHost(info.host), port: info.port };
  if (info.ports && typeof info.ports === 'object') {
    for (const value of Object.values(info.ports)) {
      if (value && typeof value === 'object' && value.port) {
        return { ip: normalizeBindHost(value.host), port: value.port };
      }
      if (typeof value === 'string' && value) {
        return { ip: location.hostname || '127.0.0.1', port: value };
      }
    }
  }
  return null;
}

function renderInstanceEndpoint(connectionInfo) {
  const endpoint = instanceEndpoint(connectionInfo);
  if (!endpoint) return '<span class="text-muted small">Endpoint is not ready yet.</span>';
  return `
    <div class="d-flex flex-wrap gap-2 align-items-center">
      <span class="tag">IP <code>${escHtml(endpoint.ip)}</code></span>
      <span class="tag">Port <code>${escHtml(String(endpoint.port))}</code></span>
    </div>`;
}

function renderSharedServiceRuntime(instance) {
  if (!instance) {
    return '<div class="text-muted small">No running service instance is registered right now.</div>';
  }
  return `
    ${renderInstanceEndpoint(instance.connection_info)}`;
}

function renderRestartEvents(events) {
  if (!Array.isArray(events) || !events.length) return '';
  return `
    <div class="workflow-panel">
      <div class="workflow-panel-title">Recent Restarts</div>
      <div class="table-responsive">
        <table class="admin-table mb-0">
          <thead><tr><th>When</th><th>Mode</th><th>Actor</th><th>Result</th><th>Message</th></tr></thead>
          <tbody>
            ${events.map(evt => `<tr>
              <td>${new Date(evt.created_at).toLocaleString()}</td>
              <td>${escHtml(evt.trigger_mode)}</td>
              <td>${escHtml(evt.actor || 'system')}</td>
              <td>${escHtml(evt.result)}</td>
              <td>${escHtml(evt.message || '')}</td>
            </tr>`).join('')}
          </tbody>
        </table>
      </div>
    </div>`;
}

function renderSploitList(sploits) {
  if (!Array.isArray(sploits) || !sploits.length) {
    return '<div class="text-muted small">No exploits uploaded for this challenge yet.</div>';
  }
  return `<div class="d-flex flex-column gap-2">${sploits.map(sp => `
    <div class="border rounded p-3">
      <div class="d-flex justify-content-between align-items-start flex-wrap gap-2">
        <div>
          <strong>${escHtml(sp.name)}</strong>
          <span class="tag ms-2" style="color:${sp.enabled ? 'var(--ctf-green)' : 'var(--ctf-red)'}">${sp.enabled ? 'enabled' : 'disabled'}</span>
          <div class="text-muted small mt-1">${escHtml(sp.language)} · last run ${sp.last_run_at ? new Date(sp.last_run_at).toLocaleString() : 'never'}</div>
        </div>
        <div class="d-flex gap-2 flex-wrap">
          <button class="btn btn-sm btn-secondary" onclick="viewChallengeSploitResults(${sp.id})">Results</button>
          <button class="btn btn-sm btn-secondary" onclick="toggleChallengeSploit(${sp.id}, ${!sp.enabled})">${sp.enabled ? 'Disable' : 'Enable'}</button>
          <button class="btn btn-sm btn-danger" onclick="deleteChallengeSploit(${sp.id})">Delete</button>
        </div>
      </div>
    </div>`).join('')}</div>`;
}

function renderAttackWorkflow(data) {
  const notes = renderWorkflowNotes(data.notes);
  return `
    ${workflowHeading('Attack Workflow', 'Upload an exploit file or paste a script. Enabled exploits run automatically every round against this challenge.')}
    ${workflowPanel('Target Service', renderSharedServiceRuntime(data.service_instance), 'target-panel')}
    <div class="workflow-panel">
      <div class="row g-2 mb-3">
        <div class="col-md-4">
          <label class="form-label">Exploit Name</label>
          <input class="form-control" id="challenge-sploit-name" placeholder="HTTP race exploit">
        </div>
        <div class="col-md-3">
          <label class="form-label">Language</label>
          <select class="form-select" id="challenge-sploit-language">
            ${(Array.isArray(data.languages) && data.languages.length ? data.languages : ['python3', 'bash']).map(lang => `<option value="${escHtml(lang)}">${escHtml(lang)}</option>`).join('')}
          </select>
        </div>
        <div class="col-md-5">
          <label class="form-label">Exploit File</label>
          <input class="form-control" id="challenge-sploit-file" type="file" accept=".py,.sh,.txt,.exploit">
        </div>
      </div>
      <div class="mb-3">
        <label class="form-label">Script</label>
        <textarea class="form-control" id="challenge-sploit-script" rows="8" placeholder="#!/usr/bin/env python3"></textarea>
        <div class="form-text">If a file is attached, the file contents override the textarea.</div>
      </div>
      <div class="d-flex align-items-center gap-3 flex-wrap">
        <button class="btn btn-primary btn-sm" onclick="uploadChallengeSploit()">Upload Exploit</button>
        <span id="challenge-sploit-msg" class="small"></span>
      </div>
      ${notes ? `<div class="mt-3">${notes}</div>` : ''}
    </div>
    <div class="workflow-panel">
      <div class="workflow-panel-title">My exploits for this challenge</div>
      <div id="challenge-sploit-list">${renderSploitList(data.sploits)}</div>
    </div>`;
}

function renderDefenseWorkflow(data) {
  const vpnSummary = data.vpn
    ? renderVPNSummary(data.vpn, data.vpn_download_url, data.vpn_sync_url)
    : '<div class="text-muted small">VPN is not configured on this server.</div>';
  return `
    ${workflowHeading('Defense Workflow', 'This service stays online continuously. Use VPN to reach it and run an immediate attack-check from here when needed.')}
    <div class="workflow-panel">
      <div class="d-flex justify-content-between align-items-start gap-3 flex-wrap">
        <div>
          <div class="small fw-semibold mb-2">Current service state</div>
          <div id="challenge-defense-current">${renderServiceStatus(data.service_status)}</div>
        </div>
        <div class="d-flex gap-2 flex-wrap align-items-start">
          <button class="btn btn-sm btn-primary" onclick="runDefenseAttackCheck()">Run Attack Check</button>
        </div>
      </div>
      <div id="challenge-defense-check-msg" class="small mt-2"></div>
      ${renderWorkflowNotes(data.notes)}
      ${vpnSummary}
      ${renderServiceRounds(data.service_rounds)}
    </div>`;
}

function renderPentestWorkflow(data) {
  const vpnSummary = data.vpn
    ? renderVPNSummary(data.vpn, data.vpn_download_url, data.vpn_sync_url)
    : '<div class="text-muted small">VPN is not configured on this server.</div>';
  const restartVoteMarkup = data.restart_vote ? `
    <div class="workflow-panel restart-panel">
      <div class="workflow-panel-title">Restart Vote</div>
      <div class="small text-muted mb-2">If the shared pentest service is broken, players can vote to restart it.</div>
      <div class="d-flex flex-wrap gap-2 align-items-center mb-2">
        <span class="tag">Votes ${data.restart_vote.votes}/${data.restart_vote.threshold}</span>
        <span class="tag">Eligible ${data.restart_vote.eligible_voters}</span>
        ${data.restart_vote.has_voted ? '<span class="tag" style="color:var(--ctf-green)">Your vote recorded</span>' : ''}
      </div>
      <div class="d-flex gap-2 flex-wrap">
        <button class="btn btn-sm btn-outline-warning" ${data.restart_vote.can_vote ? '' : 'disabled'} onclick="submitPentestRestartVote()">Vote to Restart</button>
        ${data.can_admin_restart ? '<button class="btn btn-sm btn-danger" onclick="triggerPentestAdminRestart()">Admin Restart</button>' : ''}
      </div>
      <div id="challenge-restart-vote-msg" class="small mt-2"></div>
    </div>` : '';
  return `
    ${workflowHeading('Pentest Access', 'Reach the service through VPN, collect the required flags, then submit them in the normal flag form below.')}
    <div class="workflow-panel">
      ${renderWorkflowNotes(data.notes)}
      ${vpnSummary}
      ${workflowPanel('Target Service', renderSharedServiceRuntime(data.service_instance), 'target-panel')}
      ${restartVoteMarkup}
      ${renderRestartEvents(data.restart_events)}
    </div>`;
}

function clearWorkflowSection() {
  const box = workflowBox();
  if (!box) return;
  hideEl(box);
  box.innerHTML = '';
}

async function submitAnyFlag(event) {
  event.preventDefault();
  const input = byId('global-flag-input');
  const result = byId('global-flag-result');
  const btn = byId('global-flag-submit-btn');
  const flag = String(input?.value || '').trim();
  if (!flag) {
    if (result) result.innerHTML = '<span class="text-danger">Enter a flag first.</span>';
    return;
  }
  if (btn) btn.disabled = true;
  if (result) result.innerHTML = '<span class="text-muted">Checking...</span>';
  try {
    const res = await apiFetch('/api/flags/submit', {
      method: 'POST',
      body: JSON.stringify({ flag }),
    });
    if (res.correct) {
      if (result) {
        result.innerHTML = `<span class="text-success">Correct: ${escHtml(res.challenge_name || ('challenge #' + res.challenge_id))} (+${res.points || 0})</span>`;
      }
      if (input) input.value = '';
      await loadChallenges();
      if (currentChallenge && Number(currentChallenge.id) === Number(res.challenge_id)) {
        await openChallenge(currentChallenge.id);
      }
    } else if (result) {
      result.innerHTML = '<span class="text-danger">Incorrect flag or no matching active dynamic instance.</span>';
    }
  } catch (err) {
    if (result) result.innerHTML = `<span class="text-danger">${escHtml(err.message)}</span>`;
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function loadBoardVPN() {
  const card = byId('board-vpn-card');
  const body = byId('board-vpn-body');
  if (!card || !body) return;
  try {
    const status = await apiFetch('/api/ad/status');
    if (!status.vpn_enabled) {
      body.innerHTML = '<div class="text-muted small">VPN is disabled in board settings.</div>';
      return;
    }
    const vpn = await apiFetch('/api/ad/vpn/status');
    body.innerHTML = renderBoardVPNStatus(vpn);
  } catch (err) {
    body.innerHTML = `<div class="text-danger small">${escHtml(err.message)}</div>`;
  }
}

async function syncBoardVPN() {
  const msg = byId('board-vpn-msg');
  if (msg) msg.innerHTML = '<span class="text-muted">Syncing...</span>';
  try {
    const res = await apiFetch('/api/ad/vpn/sync', { method: 'POST' });
    if (msg) msg.innerHTML = `<span class="text-success">${escHtml(res.message || 'VPN synchronized')}</span>`;
    await loadBoardVPN();
  } catch (err) {
    if (msg) msg.innerHTML = `<span class="text-danger">${escHtml(err.message)}</span>`;
  }
}

async function downloadBoardVPN() {
  const msg = byId('board-vpn-msg');
  try {
    const res = await fetch('/api/ad/vpn', { credentials: 'include' });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || 'failed to download VPN config');
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'board-wg.conf';
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    if (msg) msg.innerHTML = '<span class="text-success">Config downloaded.</span>';
  } catch (err) {
    if (msg) msg.innerHTML = `<span class="text-danger">${escHtml(err.message)}</span>`;
  }
}

async function loadChallenges() {
  const container = document.getElementById('categories');
  try {
    const challenges = await apiFetch('/api/challenges');
    if (!challenges.length) {
      container.innerHTML = '<p class="text-muted">No challenges available yet.</p>';
      return;
    }
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
    container.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

async function openChallenge(id) {
  try {
    const c = await apiFetch('/api/challenges/' + id);
    currentChallenge = c;
    const deployType = normalizeDeployType(c.deploy_type, c.challenge_type);

    document.getElementById('modal-title').textContent = c.name;
    const renderedDescription = window.marked ? marked.parse(c.description || '') : escHtml(c.description || '');
    document.getElementById('modal-desc').innerHTML = sanitizeHtml(renderedDescription);
    document.getElementById('modal-solves').textContent = (typeof c.solve_count === 'number' ? c.solve_count : 0) + ' solve' + (c.solve_count !== 1 ? 's' : '');

    document.getElementById('modal-meta').innerHTML = `
      <span class="tag">${escHtml(c.category)}</span>
      <span class="tag" style="color:var(--ctf-yellow);border-color:rgba(255,193,7,.3);background:rgba(255,193,7,.1)">${c.points} pts</span>
      ${c.flag_type === 'regex' ? '<span class="tag" style="color:var(--ctf-blue)">regex flag</span>' : ''}
      ${deployType !== 'no_deploy' ? `<span class="tag" style="color:var(--ctf-blue)">${escHtml(deployTypeLabel(c.deploy_type, c.challenge_type))}</span>` : ''}
    `;

    const instBox = document.getElementById('modal-instance');
    if (deployType !== 'no_deploy') {
      showEl(instBox);
      instBox.innerHTML = '<div class="instance-box">Checking instance status...</div>';
      loadInstanceStatus(c.id);
    } else {
      hideEl(instBox);
    }

    hideEl(byId('flag-error'));
    hideEl(byId('flag-success'));
    const flagInput = byId('flag-input');
    if (flagInput) flagInput.value = '';

    const solvedNotice = byId('modal-solved-notice');
    const submitBtn = byId('submit-btn');
    const flagForm = byId('flag-form');
    if (c.challenge_type === 'attack_defence_attack' || c.challenge_type === 'attack_defence_defense') {
      hideEl(flagForm);
      hideEl(solvedNotice);
    } else {
      showEl(flagForm);
      if (c.solved) {
        showEl(solvedNotice);
        if (submitBtn) submitBtn.disabled = true;
      } else {
        hideEl(solvedNotice);
        if (submitBtn) submitBtn.disabled = false;
      }
    }

    clearWorkflowSection();
    if (!bsModal) bsModal = new bootstrap.Modal(document.getElementById('challengeModal'));
    bsModal.show();
    await loadChallengeExtras(c.id, c);
    await loadChallengeWorkflow(c.id, c);
  } catch (err) {
    alert(err.message);
  }
}

async function loadChallengeWorkflow(challengeId, challenge) {
  const box = workflowBox();
  if (!box) return;
  if (!['attack_defence_attack', 'attack_defence_defense', 'pentest'].includes(challenge.challenge_type)) {
    clearWorkflowSection();
    return;
  }
  showEl(box);
  box.innerHTML = workflowHeading('Challenge Runtime', 'Loading workflow details...');
  try {
    const data = await apiFetch('/api/challenges/' + challengeId + '/workflow');
    if (challenge.challenge_type === 'attack_defence_attack') {
      box.innerHTML = renderAttackWorkflow(data);
    } else if (challenge.challenge_type === 'attack_defence_defense') {
      box.innerHTML = renderDefenseWorkflow(data);
    } else {
      box.innerHTML = renderPentestWorkflow(data);
    }
  } catch (err) {
    box.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

async function loadChallengeExtras(id, c) {
  const connSection = document.getElementById('modal-connection');
  const connInfo = document.getElementById('modal-connection-info');
  const deployType = normalizeDeployType(c.deploy_type, c.challenge_type);
  if (c.connection_info && deployType === 'no_deploy') {
    connInfo.textContent = c.connection_info;
    showEl(connSection);
  } else {
    hideEl(connSection);
  }

  const filesSection = document.getElementById('modal-files-section');
  const filesList = document.getElementById('modal-files-list');
  try {
    const files = await apiFetch('/api/challenges/' + id + '/files');
    if (files && files.length) {
      filesList.innerHTML = files.map(f => `
        <div class="file-item">
          <span class="file-icon">File</span>
          <a href="${escHtml(f.url || f.path || '')}" target="_blank" rel="noopener noreferrer">${escHtml(f.name || f.filename || f.url || 'File')}</a>
        </div>`).join('');
      showEl(filesSection);
    } else {
      hideEl(filesSection);
    }
  } catch (_) {
    hideEl(filesSection);
  }

  const hintsSection = document.getElementById('modal-hints-section');
  const hintsList = document.getElementById('modal-hints-list');
  try {
    const hints = await apiFetch('/api/challenges/' + id + '/hints');
    if (hints && hints.length) {
      hintsList.innerHTML = hints.map(h => `
        <div class="hint-item">
          <span class="hint-cost">${h.cost > 0 ? h.cost + ' pts' : t('challenges.hint_free')}</span>
          <span class="ms-2 text-muted">${h.content !== undefined ? escHtml(h.content) : 'Unlock to reveal'}</span>
        </div>`).join('');
      showEl(hintsSection);
    } else {
      hideEl(hintsSection);
    }
  } catch (_) {
    hideEl(hintsSection);
  }
}

async function loadInstanceStatus(challengeId) {
  const instBox = document.getElementById('modal-instance');
  const deployType = normalizeDeployType(currentChallenge?.deploy_type, currentChallenge?.challenge_type);
  try {
    const inst = await apiFetch('/api/challenges/' + challengeId + '/instance');
    if (inst && inst.status === 'running') {
      const actions = deployType === 'per_instance'
        ? `<div class="d-flex gap-2 flex-wrap">
             <button class="btn btn-sm btn-danger" onclick="stopInstance(${challengeId})">Stop</button>
             <button class="btn btn-sm btn-secondary" onclick="extendInstance(${challengeId})">+${escHtml(instanceTTLLabel)}</button>
           </div>`
        : '';
      instBox.innerHTML = `
        <div class="instance-box">
          <div class="d-flex justify-content-between align-items-center gap-3 flex-wrap">
            ${renderInstanceEndpoint(inst.connection_info)}
            ${actions}
          </div>
        </div>`;
    } else {
      showStartButton(instBox, challengeId, deployType);
    }
  } catch {
    showStartButton(instBox, challengeId, deployType);
  }
}

function showStartButton(instBox, challengeId, deployType) {
  if (deployType === 'always_on') {
    instBox.innerHTML = `
      <div class="d-flex align-items-center gap-2">
        <span class="text-muted small">This service is provisioned by operators and stays online outside player sessions.</span>
      </div>`;
    return;
  }
  instBox.innerHTML = `
    <div class="d-flex align-items-center gap-2">
      <button class="btn btn-sm btn-secondary" onclick="startInstance(${challengeId})">Deploy Instance</button>
      <span class="text-muted small">No running instance</span>
    </div>`;
}

async function startInstance(challengeId) {
  const instBox = document.getElementById('modal-instance');
  instBox.innerHTML = '<div class="instance-box">Starting instance...</div>';
  try {
    await apiFetch('/api/challenges/' + challengeId + '/instance', { method: 'POST' });
    await loadInstanceStatus(challengeId);
  } catch (err) {
    instBox.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

async function stopInstance(challengeId) {
  const instBox = document.getElementById('modal-instance');
  instBox.innerHTML = '<div class="instance-box">Stopping...</div>';
  try {
    await apiFetch('/api/challenges/' + challengeId + '/instance', { method: 'DELETE' });
    showStartButton(instBox, challengeId, normalizeDeployType(currentChallenge?.deploy_type, currentChallenge?.challenge_type));
  } catch (err) {
    instBox.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

async function extendInstance(challengeId) {
  const instBox = document.getElementById('modal-instance');
  instBox.innerHTML = '<div class="instance-box">Extending...</div>';
  try {
    await apiFetch('/api/challenges/' + challengeId + '/instance/extend', { method: 'POST' });
    await loadInstanceStatus(challengeId);
  } catch (err) {
    instBox.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

async function readChallengeSploitScript() {
  const fileInput = document.getElementById('challenge-sploit-file');
  const text = document.getElementById('challenge-sploit-script').value;
  if (fileInput && fileInput.files && fileInput.files[0]) {
    return await fileInput.files[0].text();
  }
  return text;
}

async function uploadChallengeSploit() {
  if (!currentChallenge) return;
  const msg = document.getElementById('challenge-sploit-msg');
  const name = document.getElementById('challenge-sploit-name').value.trim();
  const language = document.getElementById('challenge-sploit-language').value;
  const script = (await readChallengeSploitScript()).trim();
  if (!name || !script) {
    msg.textContent = 'Exploit name and script are required.';
    msg.style.color = 'var(--ctf-red)';
    return;
  }
  try {
    const res = await apiFetch('/api/ad/sploits', {
      method: 'POST',
      body: JSON.stringify({ name, language, script, challenge_id: currentChallenge.id }),
    });
    msg.textContent = `Exploit #${res.id} uploaded.`;
    msg.style.color = 'var(--ctf-green)';
    document.getElementById('challenge-sploit-name').value = '';
    document.getElementById('challenge-sploit-script').value = '';
    document.getElementById('challenge-sploit-file').value = '';
    await loadChallengeWorkflow(currentChallenge.id, currentChallenge);
  } catch (err) {
    msg.textContent = err.message;
    msg.style.color = 'var(--ctf-red)';
  }
}

async function toggleChallengeSploit(id, enabled) {
  try {
    await apiFetch('/api/ad/sploits/' + id, { method: 'PUT', body: JSON.stringify({ enabled }) });
    await loadChallengeWorkflow(currentChallenge.id, currentChallenge);
  } catch (err) {
    alert(err.message);
  }
}

async function deleteChallengeSploit(id) {
  if (!confirm('Delete this exploit?')) return;
  try {
    await apiFetch('/api/ad/sploits/' + id, { method: 'DELETE' });
    await loadChallengeWorkflow(currentChallenge.id, currentChallenge);
  } catch (err) {
    alert(err.message);
  }
}

async function viewChallengeSploitResults(id) {
  const resultsBody = document.getElementById('challenge-sploit-results-body');
  resultsBody.textContent = 'Loading...';
  try {
    const results = await apiFetch('/api/ad/sploits/' + id + '/results');
    if (!challengeSploitResultsModal) {
      challengeSploitResultsModal = new bootstrap.Modal(document.getElementById('challenge-sploit-results-modal'));
    }
    document.getElementById('challenge-sploit-results-title').textContent = `Exploit #${id} Results`;
    if (!results.length) {
      resultsBody.innerHTML = '<div class="text-muted">No results yet.</div>';
    } else {
      resultsBody.innerHTML = results.slice(0, 50).map(r => `
        <div class="border rounded p-3 mb-3">
          <div class="d-flex justify-content-between align-items-center flex-wrap gap-2 mb-2">
            <div><strong>Round ${r.round}</strong> <span class="text-muted">target ${r.target_team_id}</span></div>
            <div class="small text-muted">${new Date(r.ran_at).toLocaleString()}</div>
          </div>
          <div class="small mb-2">
            <span class="tag me-2">points: ${r.awarded_points || 0}</span>
            <span class="tag me-2">captured: ${r.flags_captured}</span>
            <span class="tag">submitted: ${r.flags_submitted}</span>
          </div>
          <div class="row g-3">
            <div class="col-lg-6">
              <div class="small fw-semibold mb-1">Error</div>
              <pre class="bg-dark text-light p-2 rounded small mb-0" style="min-height:72px;white-space:pre-wrap">${escHtml(r.error || '(none)')}</pre>
            </div>
            <div class="col-lg-6">
              <div class="small fw-semibold mb-1">Stdout</div>
              <pre class="bg-dark text-light p-2 rounded small mb-0" style="min-height:72px;max-height:280px;overflow:auto;white-space:pre-wrap">${escHtml(r.stdout || '(empty)')}</pre>
            </div>
          </div>
        </div>`).join('');
    }
    challengeSploitResultsModal.show();
  } catch (err) {
    resultsBody.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

async function downloadWorkflowVPN(url) {
  try {
    const resp = await fetch(url, { credentials: 'same-origin' });
    const data = await resp.blob();
    if (!resp.ok) {
      let msg = 'VPN download failed';
      try {
        const text = await data.text();
        const parsed = JSON.parse(text);
        msg = parsed.error || msg;
      } catch (_) {}
      throw new Error(msg);
    }
    const blobURL = URL.createObjectURL(data);
    const link = document.createElement('a');
    link.href = blobURL;
    link.download = 'team-wg.conf';
    link.click();
    URL.revokeObjectURL(blobURL);
  } catch (err) {
    alert(err.message);
  }
}

async function syncWorkflowVPN(url) {
  const msg = document.getElementById('challenge-vpn-msg');
  if (msg) {
    msg.textContent = 'Synchronizing VPN access...';
    msg.style.color = 'var(--ctf-text-muted)';
  }
  try {
    const resp = await fetch(url, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' }
    });
    const payload = await resp.json().catch(() => ({}));
    if (!resp.ok) {
      throw new Error(payload.error || 'VPN sync failed');
    }
    if (msg) {
      msg.textContent = payload.message || 'VPN access synchronized';
      msg.style.color = 'var(--ctf-green)';
    }
    if (currentChallenge) {
      await loadChallengeWorkflow(currentChallenge.id, currentChallenge);
    }
  } catch (err) {
    if (msg) {
      msg.textContent = err.message || 'VPN sync failed';
      msg.style.color = 'var(--ctf-red)';
    } else {
      alert(err.message);
    }
  }
}

async function submitPentestRestartVote() {
  if (!currentChallenge) return;
  const msg = byId('challenge-restart-vote-msg');
  if (msg) {
    msg.textContent = 'Submitting restart vote...';
    msg.style.color = 'var(--ctf-text-muted)';
  }
  try {
    const result = await apiFetch('/api/challenges/' + currentChallenge.id + '/workflow/restart-vote', { method: 'POST' });
    if (msg) {
      msg.textContent = result.message || 'Restart vote recorded';
      msg.style.color = 'var(--ctf-green)';
    }
    await loadChallengeWorkflow(currentChallenge.id, currentChallenge);
    await loadInstanceStatus(currentChallenge.id);
  } catch (err) {
    if (msg) {
      msg.textContent = err.message || 'Restart vote failed';
      msg.style.color = 'var(--ctf-red)';
    } else {
      alert(err.message);
    }
  }
}

async function triggerPentestAdminRestart() {
  if (!currentChallenge) return;
  const msg = byId('challenge-restart-vote-msg');
  if (msg) {
    msg.textContent = 'Triggering admin restart...';
    msg.style.color = 'var(--ctf-text-muted)';
  }
  try {
    const result = await apiFetch('/api/challenges/' + currentChallenge.id + '/workflow/restart', { method: 'POST' });
    if (msg) {
      msg.textContent = result.message || 'Service restart triggered';
      msg.style.color = 'var(--ctf-green)';
    }
    await loadChallengeWorkflow(currentChallenge.id, currentChallenge);
    await loadInstanceStatus(currentChallenge.id);
  } catch (err) {
    if (msg) {
      msg.textContent = err.message || 'Admin restart failed';
      msg.style.color = 'var(--ctf-red)';
    } else {
      alert(err.message);
    }
  }
}

async function runDefenseAttackCheck() {
  if (!currentChallenge) return;
  const msg = document.getElementById('challenge-defense-check-msg');
  const current = document.getElementById('challenge-defense-current');
  if (msg) {
    msg.textContent = 'Running attack-check...';
    msg.style.color = 'var(--ctf-text-muted)';
  }
  try {
    const result = await apiFetch('/api/challenges/' + currentChallenge.id + '/workflow/check', { method: 'POST' });
    if (current) current.innerHTML = renderServiceStatus(result);
    if (msg) {
      msg.textContent = 'Attack-check completed. This run is informational and does not change scoreboard points.';
      msg.style.color = 'var(--ctf-green)';
    }
  } catch (err) {
    if (msg) {
      msg.textContent = err.message;
      msg.style.color = 'var(--ctf-red)';
    } else {
      alert(err.message);
    }
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
      okEl.textContent = 'Correct! +' + res.points + ' points';
      okEl.classList.remove('d-none');
      document.getElementById('submit-btn').disabled = true;
      document.getElementById('modal-solved-notice').classList.remove('d-none');
      loadChallenges();
    } else {
      errEl.textContent = 'Incorrect flag. Try again.';
      errEl.classList.remove('d-none');
    }
  } catch (err) {
    errEl.textContent = err.message;
    errEl.classList.remove('d-none');
  }
}





