// statistics.js

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

async function loadStatistics() {
  try {
    const stats = await apiFetch('/api/statistics');

    document.getElementById('stat-users').textContent      = stats.total_users      ?? stats.users      ?? '–';
    document.getElementById('stat-challenges').textContent = stats.total_challenges ?? stats.challenges ?? '–';
    document.getElementById('stat-correct').textContent    = stats.correct_submissions ?? stats.correct ?? '–';
    document.getElementById('stat-total').textContent      = stats.total_submissions   ?? stats.total   ?? '–';

    const categories = stats.categories || [];
    const tbody = document.getElementById('category-body');
    if (!categories.length) {
      tbody.innerHTML = '<tr><td colspan="3" class="text-center py-4 text-muted">No data.</td></tr>';
      return;
    }
    tbody.innerHTML = categories.map(cat => `
      <tr>
        <td><span class="tag-badge">${escHtml(cat.name || cat.category || '')}</span></td>
        <td>${cat.challenge_count ?? cat.challenges ?? 0}</td>
        <td>${cat.solve_count ?? cat.solves ?? 0}</td>
      </tr>
    `).join('');
  } catch (err) {
    document.getElementById('category-body').innerHTML =
      `<tr><td colspan="3" class="text-center py-4 text-danger">${escHtml(err.message)}</td></tr>`;
  }
}
