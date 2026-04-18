// scoreboard.js

let _scoreboardEntries = [];

async function loadScoreboard() {
  const tbody = document.getElementById('scoreboard-body');
  try {
    _scoreboardEntries = await apiFetch('/api/scoreboard');
    renderScoreboard();
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-center py-4 text-danger">${err.message}</td></tr>`;
  }
}

function filterScoreboard() {
  renderScoreboard();
}

function renderScoreboard() {
  const tbody = document.getElementById('scoreboard-body');
  const showAdmins = document.getElementById('show-admins') && document.getElementById('show-admins').checked;

  let entries = _scoreboardEntries;
  if (!showAdmins) {
    entries = entries.filter(e => !(e.user && e.user.role === 'admin'));
  }

  if (!entries.length) {
    tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4 text-muted">No entries yet.</td></tr>';
    return;
  }

  tbody.innerHTML = entries.map((e, idx) => {
    const displayRank = showAdmins ? e.rank : (idx + 1);
    const name = e.user ? escHtml(e.user.username) : escHtml(e.team ? e.team.name : '?');
    let rowClass = '';
    if (displayRank === 1) rowClass = ' class="rank-gold"';
    else if (displayRank === 2) rowClass = ' class="rank-silver"';
    else if (displayRank === 3) rowClass = ' class="rank-bronze"';
    const medal = displayRank === 1 ? '🥇' : displayRank === 2 ? '🥈' : displayRank === 3 ? '🥉' : '';
    return `<tr${rowClass}>
      <td>${medal || displayRank}</td>
      <td>${name}</td>
      <td>${e.solves}</td>
      <td><strong>${e.score}</strong></td>
    </tr>`;
  }).join('');
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

loadScoreboard();
