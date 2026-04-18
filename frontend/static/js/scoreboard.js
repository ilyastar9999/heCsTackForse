// scoreboard.js

async function loadScoreboard() {
  const tbody = document.getElementById('scoreboard-body');
  try {
    const entries = await apiFetch('/api/scoreboard');
    if (!entries.length) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-center py-4 text-muted">No entries yet.</td></tr>';
      return;
    }
    tbody.innerHTML = entries.map(e => {
      const name = e.user ? escHtml(e.user.username) : escHtml(e.team ? e.team.name : '?');
      let rowClass = '';
      if (e.rank === 1) rowClass = ' class="rank-gold"';
      else if (e.rank === 2) rowClass = ' class="rank-silver"';
      else if (e.rank === 3) rowClass = ' class="rank-bronze"';
      const medal = e.rank === 1 ? '🥇' : e.rank === 2 ? '🥈' : e.rank === 3 ? '🥉' : '';
      return `<tr${rowClass}>
        <td>${medal || e.rank}</td>
        <td>${name}</td>
        <td>${e.solves}</td>
        <td><strong>${e.score}</strong></td>
      </tr>`;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-center py-4 text-danger">${err.message}</td></tr>`;
  }
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

loadScoreboard();
