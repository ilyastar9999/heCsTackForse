let modulesState = [];
let activeModuleSlug = '';
let activeStepId = 0;

async function loadModules() {
  const root = document.getElementById('modules-root');
  try {
    modulesState = await apiFetch('/api/modules');
    if (!modulesState.length) {
      root.innerHTML = '<div class="card p-4 text-muted">No learning modules are published yet.</div>';
      return;
    }
    const params = new URLSearchParams(window.location.search);
    activeModuleSlug = params.get('module') || modulesState[0].slug;
    const current = modulesState.find(m => m.slug === activeModuleSlug) || modulesState[0];
    activeModuleSlug = current.slug;
    const steps = moduleSteps(current);
    activeStepId = parseInt(params.get('step'), 10) || (steps[0] ? steps[0].id : 0);
    renderModules();
  } catch (err) {
    root.innerHTML = `<div class="alert alert-danger">${escHtml(err.message)}</div>`;
  }
}

function moduleSteps(module) {
  return (module.items || []).slice().sort((a, b) => {
    const order = (parseInt(a.sort_order) || 0) - (parseInt(b.sort_order) || 0);
    return order || ((parseInt(a.id) || 0) - (parseInt(b.id) || 0));
  });
}

function renderModules() {
  const root = document.getElementById('modules-root');
  const activeModule = modulesState.find(m => m.slug === activeModuleSlug) || modulesState[0];
  const steps = moduleSteps(activeModule);
  const activeStep = steps.find(i => i.id === activeStepId) || steps[0] || null;
  if (activeStep) activeStepId = activeStep.id;
  const index = activeStep ? Math.max(0, steps.findIndex(s => s.id === activeStep.id)) : -1;

  root.innerHTML = `
    <div class="modules-shell">
      <aside class="modules-course-list">
        <div class="section-kicker">Courses</div>
        ${modulesState.map(m => `
          <button type="button" class="course-pill ${m.slug === activeModule.slug ? 'active' : ''}" onclick="selectModule('${escJs(m.slug)}')">
            <strong>${escHtml(m.title)}</strong>
            <span>${moduleSteps(m).length} steps</span>
          </button>
        `).join('')}
      </aside>
      <section class="module-reader">
        <div class="module-hero">
          <div>
            <div class="section-kicker">Module</div>
            <h2>${escHtml(activeModule.title)}</h2>
            ${activeModule.description ? `<p>${escHtml(activeModule.description)}</p>` : ''}
          </div>
          <div class="module-progress-badge">${index >= 0 ? index + 1 : 0}/${steps.length}</div>
        </div>
        <div class="module-learning-grid">
          <nav class="module-stepper" aria-label="Module steps">
            ${renderStepList(steps, activeStep)}
          </nav>
          <article class="module-step-panel">
            ${renderActiveStep(activeStep, index, steps)}
          </article>
        </div>
      </section>
    </div>`;
}

function renderStepList(steps, activeStep) {
  if (!steps.length) {
    return '<div class="text-muted small">No steps yet.</div>';
  }
  return steps.map((step, idx) => {
    const isChallenge = step.type === 'challenge';
    const label = step.title || (isChallenge ? step.challenge?.name : step.page?.title) || (isChallenge ? 'Challenge' : 'Theory');
    return `
      <button type="button" class="module-step-link ${activeStep && step.id === activeStep.id ? 'active' : ''} ${isChallenge ? 'challenge-step' : ''}" onclick="selectStep(${step.id})">
        <span class="step-index">${idx + 1}</span>
        <span class="step-copy">
          <strong>${escHtml(label)}</strong>
          <small>${isChallenge ? escHtml(step.challenge?.category || 'Practice') : 'Theory'}</small>
        </span>
      </button>`;
  }).join('');
}

function renderActiveStep(step, index, steps) {
  if (!step) {
    return '<div class="card p-4 text-muted">This module has no steps yet.</div>';
  }
  const prev = steps[index - 1];
  const next = steps[index + 1];
  return `
    ${step.type === 'challenge' ? renderChallengeStep(step) : renderTheoryStep(step)}
    <div class="module-step-nav">
      <button class="btn btn-sm btn-secondary" ${prev ? '' : 'disabled'} onclick="${prev ? `selectStep(${prev.id})` : ''}">Previous</button>
      <button class="btn btn-sm btn-primary" ${next ? '' : 'disabled'} onclick="${next ? `selectStep(${next.id})` : ''}">Next</button>
    </div>`;
}

function renderTheoryStep(step) {
  const title = step.title || step.page?.title || 'Theory';
  const raw = step.content || step.page?.content || '';
  const rendered = window.marked ? marked.parse(raw || '_No theory content yet._') : escHtml(raw || 'No theory content yet.');
  return `
    <div class="module-step-card theory">
      <div class="step-type-label">Theory Step</div>
      <h3>${escHtml(title)}</h3>
      <div class="markdown-body">${sanitizeHtml(rendered)}</div>
    </div>`;
}

function renderChallengeStep(step) {
  const ch = step.challenge || {};
  return `
    <div class="module-step-card challenge">
      <div class="step-type-label">Practice Step</div>
      <h3>${escHtml(step.title || ch.name || 'Challenge')}</h3>
      <p class="text-muted">${escHtml(ch.description ? stripMarkdown(ch.description).slice(0, 220) : 'Open the challenge, solve it, then continue through the module.')}</p>
      <div class="d-flex flex-wrap gap-2 mb-3">
        <span class="tag">${escHtml(ch.category || 'Challenge')}</span>
        <span class="tag">${Number.isFinite(ch.points) ? ch.points : 0} pts</span>
        ${ch.challenge_type ? `<span class="tag">${escHtml(ch.challenge_type)}</span>` : ''}
      </div>
      <a class="btn btn-primary" href="/challenges?challenge=${encodeURIComponent(ch.id || step.challenge_id || '')}">Open Challenge</a>
    </div>`;
}

function selectModule(slug) {
  activeModuleSlug = slug;
  const module = modulesState.find(m => m.slug === slug);
  const steps = moduleSteps(module || {});
  activeStepId = steps[0] ? steps[0].id : 0;
  history.replaceState(null, '', `/modules?module=${encodeURIComponent(slug)}${activeStepId ? `&step=${encodeURIComponent(activeStepId)}` : ''}`);
  renderModules();
}

function selectStep(id) {
  activeStepId = id;
  history.replaceState(null, '', `/modules?module=${encodeURIComponent(activeModuleSlug)}&step=${encodeURIComponent(id)}`);
  renderModules();
}

function stripMarkdown(value) {
  return String(value || '').replace(/[`*_>#\[\]()]/g, '').replace(/\s+/g, ' ').trim();
}

function escJs(s) {
  return String(s).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}
