// profile.js

function loadProfile(user) {
  const card = document.getElementById('profile-card');
  const initial = user.username.charAt(0).toUpperCase();
  card.innerHTML = `
    <div class="profile-avatar">${escHtml(initial)}</div>
    <div class="profile-username">${escHtml(user.username)}</div>
    <div class="profile-role">${escHtml(user.role)}</div>
    <div class="profile-score">${user.score}</div>
    <div class="profile-score-label">points</div>
  `;
}

function escHtml(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
