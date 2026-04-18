// statistics.js

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

async function loadStatistics() {
  try {
    const stats = await apiFetch('/api/statistics');

    // API returns: users_count, teams_count, challenges_count, submissions_count, correct_submissions, categories[{name,count,solved}]
    document.getElementById('stat-users').textContent      = stats.users_count         ?? '–';
    document.getElementById('stat-challenges').textContent = stats.challenges_count    ?? '–';
    document.getElementById('stat-correct').textContent    = stats.correct_submissions ?? '–';
    document.getElementById('stat-total').textContent      = stats.submissions_count   ?? '–';

    const categories = stats.categories || [];
    const tbody = document.getElementById('category-body');
    if (!categories.length) {
      tbody.innerHTML = '<tr><td colspan="3" class="text-center py-4 text-muted">No data.</td></tr>';
      return;
    }
    tbody.innerHTML = categories.map(cat => `
      <tr>
        <td><span class="tag-badge">${escHtml(cat.name || '')}</span></td>
        <td>${cat.count ?? 0}</td>
        <td>${cat.solved ?? 0}</td>
      </tr>
    `).join('');
  } catch (err) {
    document.getElementById('category-body').innerHTML =
      `<tr><td colspan="3" class="text-center py-4 text-danger">${escHtml(err.message)}</td></tr>`;
  }
}
