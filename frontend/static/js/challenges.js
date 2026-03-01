// challenges.js

let currentChallenge = null;

async function loadChallenges() {
  try {
    const challenges = await apiFetch('/api/challenges');
    const container = document.getElementById('categories');
    if (!challenges.length) {
      container.innerHTML = '<p style="color:var(--text-muted)">No challenges available yet.</p>';
      return;
    }
    // Group by category
    const cats = {};
    challenges.forEach(c => {
      if (!cats[c.category]) cats[c.category] = [];
      cats[c.category].push(c);
    });
    container.innerHTML = Object.entries(cats).map(([cat, chs]) => `
      <div class="category-section">
        <div class="category-title">${escHtml(cat)}</div>
        <div class="challenges-grid">
          ${chs.map(c => `
            <div class="challenge-card${c.solved ? ' solved' : ''}" onclick="openChallenge(${c.id})">
              ${c.solved ? '<span class="solved-badge">✓</span>' : ''}
              <div class="ch-name">${escHtml(c.name)}</div>
              <div class="ch-points">${c.points}</div>
              <div class="ch-solves">${c.solve_count} solve${c.solve_count !== 1 ? 's' : ''}</div>
            </div>`).join('')}
        </div>
      </div>`).join('');
  } catch (err) {
    document.getElementById('categories').innerHTML = `<p style="color:var(--red)">${err.message}</p>`;
  }
}

async function openChallenge(id) {
  try {
    const c = await apiFetch('/api/challenges/' + id);
    currentChallenge = c;
    document.getElementById('modal-title').textContent = c.name;
    document.getElementById('modal-desc').textContent = c.description;
    document.getElementById('modal-category').textContent = c.category;
    document.getElementById('modal-points').textContent = c.points + ' pts';
    document.getElementById('modal-solves').textContent = c.solve_count + ' solves';
    document.getElementById('flag-error').style.display = 'none';
    document.getElementById('flag-success').style.display = 'none';
    document.getElementById('flag-input').value = '';
    const solvedNotice = document.getElementById('modal-solved-notice');
    const submitBtn = document.getElementById('submit-btn');
    if (c.solved) {
      solvedNotice.style.display = 'block';
      submitBtn.disabled = true;
    } else {
      solvedNotice.style.display = 'none';
      submitBtn.disabled = false;
    }
    document.getElementById('modal-overlay').style.display = 'flex';
  } catch (err) {
    alert(err.message);
  }
}

function closeModal() {
  document.getElementById('modal-overlay').style.display = 'none';
  currentChallenge = null;
}

async function submitFlag(e) {
  e.preventDefault();
  if (!currentChallenge) return;
  const flag = document.getElementById('flag-input').value.trim();
  const errEl = document.getElementById('flag-error');
  const okEl = document.getElementById('flag-success');
  errEl.style.display = 'none';
  okEl.style.display = 'none';
  try {
    const res = await apiFetch('/api/challenges/' + currentChallenge.id + '/submit', {
      method: 'POST',
      body: JSON.stringify({ flag }),
    });
    if (res.correct) {
      okEl.textContent = '🎉 Correct! +' + res.points + ' points';
      okEl.style.display = 'block';
      document.getElementById('submit-btn').disabled = true;
      loadChallenges();
    } else {
      errEl.textContent = '✗ Incorrect flag. Try again.';
      errEl.style.display = 'block';
    }
  } catch (err) {
    errEl.textContent = err.message;
    errEl.style.display = 'block';
  }
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
