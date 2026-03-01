// scoreboard.js

async function loadScoreboard() {
  try {
    const entries = await apiFetch('/api/scoreboard');
    const tbody = document.getElementById('scoreboard-body');
    if (!entries.length) {
      tbody.innerHTML = '<tr><td colspan="4" class="loading">No entries yet.</td></tr>';
      return;
    }
    tbody.innerHTML = entries.map(e => {
      const name = e.user ? escHtml(e.user.username) : escHtml(e.team ? e.team.name : '?');
      const rankClass = e.rank <= 3 ? ` class="rank-${e.rank}"` : '';
      return `<tr${rankClass}>
        <td>${e.rank}</td>
        <td>${name}</td>
        <td>${e.solves}</td>
        <td>${e.score}</td>
      </tr>`;
    }).join('');
  } catch (err) {
    document.getElementById('scoreboard-body').innerHTML =
      `<tr><td colspan="4" class="loading">${err.message}</td></tr>`;
  }
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

loadScoreboard();
