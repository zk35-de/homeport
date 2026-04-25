// Apply background color from data-color attribute to avoid inline style= CSP violations.
// Called on load and after HTMX swaps that re-render the profile list.
function initAuroraSwatches(root) {
  (root || document).querySelectorAll('.profile-aurora-color[data-color]').forEach(function(el) {
    el.style.background = el.dataset.color;
  });
}

document.addEventListener('DOMContentLoaded', function() {
  initAuroraSwatches();
  // ── Favicon fetch (new service form) ──────────────────────────────
  const btn = document.getElementById('fetch-favicon-btn');
  const iconInput = document.getElementById('svc-icon-input');
  const preview = document.getElementById('favicon-preview');
  const urlInput = document.querySelector("input[name='url']");

  if (btn && iconInput && urlInput) {
    async function doFetchFavicon() {
      const url = urlInput.value.trim();
      if (!url) return;
      btn.textContent = '⏳';
      const faviconURL = '/api/favicon?url=' + encodeURIComponent(url);
      try {
        const resp = await fetch(faviconURL);
        if (resp.ok) {
          iconInput.value = faviconURL;
          preview.src = faviconURL;
          preview.classList.remove('hidden');
          btn.textContent = '✅';
        } else {
          btn.textContent = '❌';
        }
      } catch {
        btn.textContent = '❌';
      }
      setTimeout(function() { btn.textContent = '🌐'; }, 2000);
    }

    btn.addEventListener('click', doFetchFavicon);
    urlInput.addEventListener('blur', function() {
      if (!iconInput.value.trim()) doFetchFavicon();
    });

    if (iconInput.value.startsWith('/api/favicon') || iconInput.value.startsWith('http')) {
      preview.src = iconInput.value;
      preview.style.display = 'inline';
    }
  }

  // ── Restore form confirm ────────────────────────────────────────
  var restoreForm = document.getElementById('restore-form');
  if (restoreForm) {
    restoreForm.addEventListener('submit', function(e) {
      var msg = restoreForm.dataset.confirm || '⚠️ Restore will overwrite ALL data. Continue?';
      if (!confirm(msg)) {
        e.preventDefault();
      }
    });
  }

  // ── Accent color picker ────────────────────────────────────────
  var accentPicker = document.getElementById('accent-picker');
  if (accentPicker) {
    accentPicker.addEventListener('input', function() { previewAccent(this.value); });
    accentPicker.addEventListener('change', function() { saveAccent(this.value); });
  }

  document.getElementById('reset-accent-btn')?.addEventListener('click', resetAccent);
  document.getElementById('save-css-btn')?.addEventListener('click', saveCustomCSS);
  document.getElementById('reset-css-btn')?.addEventListener('click', resetCustomCSS);

  // ── Theme buttons ──────────────────────────────────────────────
  document.addEventListener('click', function(e) {
    var btn = e.target.closest('.theme-btn[data-theme]');
    if (btn) setTheme(btn.dataset.theme);
  });

  // ── Background mode buttons ────────────────────────────────────
  document.addEventListener('click', function(e) {
    var btn = e.target.closest('.bg-btn[data-bg]');
    if (btn) setBgMode(btn.dataset.bg);
  });

  // ── Aurora settings (intensity, animation) ────────────────
  document.addEventListener('click', function(e) {
    const intensityBtn = e.target.closest('.intensity-btn[data-intensity]');
    if (intensityBtn) {
      const intensity = intensityBtn.dataset.intensity;
      savePrefs({ aurora_intensity: intensity });
      const auroraDiv = document.getElementById('bg-aurora');
      if (auroraDiv) {
        auroraDiv.classList.remove('intensity-subtle', 'intensity-medium', 'intensity-vivid');
        auroraDiv.classList.add('intensity-' + intensity);
      }
      document.querySelectorAll('.intensity-btn').forEach(b => b.classList.toggle('active', b.dataset.intensity === intensity));
    }

    const animBtn = e.target.closest('.anim-btn[data-animated]');
    if (animBtn) {
      const animated = animBtn.dataset.animated === 'true';
      savePrefs({ aurora_animated: animated ? 'true' : 'false' });
      const auroraDiv = document.getElementById('bg-aurora');
      if (auroraDiv) auroraDiv.classList.toggle('animated', animated);
      animBtn.dataset.animated = animated ? 'false' : 'true';
      animBtn.classList.toggle('active', animated);
      animBtn.textContent = animated ? animBtn.dataset.tOn : animBtn.dataset.tOff;
    }
  });

  // Re-init swatches after HTMX swaps (e.g. profile list reload after default/delete)
  document.addEventListener('htmx:afterSwap', function(e) {
    initAuroraSwatches(e.target);
  });

  // ── Profil-Aurora-Farbe (per Profile) ────────────────────────────
  document.addEventListener('change', function(e) {
    if (e.target.classList.contains('profile-aurora-input')) {
      const color = e.target.value;
      const profileSlug = e.target.dataset.profile;
      const currentProfile = document.body.dataset.profile;
      // Save to that profile's prefs
      fetch('/api/user/preferences?profile=' + encodeURIComponent(profileSlug), {
        method: 'PATCH',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': (window._getCookie || function(){return '';})('hp_csrf')},
        body: JSON.stringify({ aurora_color: color })
      });
      // Update swatch
      const swatch = e.target.closest('label').querySelector('.profile-aurora-color');
      if (swatch) swatch.style.background = color;
      // Live-update: theme.css für das geänderte Profil laden (nicht das Manage-Seiten-Profil)
      var link = document.getElementById('user-theme');
      if (link) link.href = '/api/profile/' + encodeURIComponent(profileSlug) + '/theme.css?t=' + Date.now();
    }
  });

  // ── Favicon edit buttons (event delegation) ────────────────────
  document.addEventListener('click', function(e) {
    var editBtn = e.target.closest('.fetch-favicon-btn[data-svc-id]');
    if (editBtn) fetchFaviconEdit(editBtn.dataset.svcId, editBtn);
  });

  // ── Manage Tab System ──────────────────────────────────────────
  var STORAGE_KEY = 'hp-manage-tab';
  var DEFAULT_TAB = 'panel-services';

  function showPanel(panelId) {
    document.querySelectorAll('.manage-panel').forEach(function(p) {
      p.style.display = 'none';
    });
    var panel = document.getElementById(panelId);
    if (panel) panel.style.display = 'block';
    document.querySelectorAll('.manage-tab').forEach(function(btn) {
      btn.classList.toggle('active', btn.dataset.panel === panelId);
    });
    try { localStorage.setItem(STORAGE_KEY, panelId); } catch(e) {}
  }

  var saved;
  try { saved = localStorage.getItem(STORAGE_KEY); } catch(e) {}
  var target = (saved && document.getElementById(saved)) ? saved : DEFAULT_TAB;
  showPanel(target);

  document.querySelectorAll('.manage-tab').forEach(function(btn) {
    btn.addEventListener('click', function() { showPanel(btn.dataset.panel); });
  });

  // ── Category collapse toggle ──────────────────────────────────
  var COLLAPSE_KEY = 'hp-cat-collapsed';
  function getCollapsed() { try { return JSON.parse(localStorage.getItem(COLLAPSE_KEY) || '[]'); } catch(e) { return []; } }
  function setCollapsed(ids) { try { localStorage.setItem(COLLAPSE_KEY, JSON.stringify(ids)); } catch(e) {} }

  function applyCollapse() {
    var collapsed = getCollapsed();
    document.querySelectorAll('[data-cat-toggle]').forEach(function(arrow) {
      var id = arrow.dataset.catToggle;
      var cat = document.getElementById('cat-' + id) || document.getElementById('cat-discovery');
      if (!cat) return;
      var list = cat.querySelector('.manage-service-list');
      if (!list) return;
      if (collapsed.indexOf(id) !== -1) {
        list.classList.add('collapsed');
        arrow.classList.add('collapsed');
      } else {
        list.classList.remove('collapsed');
        arrow.classList.remove('collapsed');
      }
    });
  }

  document.addEventListener('click', function(e) {
    var arrow = e.target.closest('[data-cat-toggle]');
    if (!arrow) return;
    var id = arrow.dataset.catToggle;
    var cat = document.getElementById('cat-' + id) || document.getElementById('cat-discovery');
    if (!cat) return;
    var list = cat.querySelector('.manage-service-list');
    if (!list) return;
    var collapsed = getCollapsed();
    var idx = collapsed.indexOf(id);
    if (idx === -1) { collapsed.push(id); } else { collapsed.splice(idx, 1); }
    setCollapsed(collapsed);
    applyCollapse();
  });

  applyCollapse();

  // ── htmx:afterRequest handlers (replaces hx-on::after-request inline eval) ──
  document.body.addEventListener('htmx:afterRequest', function(e) {
    var elt = e.detail && e.detail.elt;
    if (!elt || !e.detail.successful) return;
    if (elt.hasAttribute('data-reset-on-success')) {
      elt.reset();
    }
    if (elt.classList.contains('inbox-accept-form')) {
      window.location.reload();
    }
  });

  // ── Sortable init ──────────────────────────────────────────────
  initSortable();
  document.body.addEventListener('htmx:afterSwap', function(e) {
    if (e.detail && e.detail.target && e.detail.target.id === 'category-list') {
      applyCollapse();
      initSortable();
      // HTMX 2.x fires HX-Trigger events on the target element, not on body.
      // #cat-select listens with "from:body", so we dispatch explicitly (#149).
      htmx.trigger(document.body, 'categoryAdded');
    }
    if (e.detail && e.detail.target && e.detail.target.id === 'profile-list') {
      // Refresh auth password-form profile dropdown (#150)
      htmx.trigger(document.body, 'profileChanged');
    }
  });
});

// ── Sortable ──────────────────────────────────────────────────────────
function initSortable() {
  var catList = document.getElementById('sortable-categories');
  if (!catList || typeof Sortable === 'undefined') return;

  Sortable.create(catList, {
    handle: '.drag-handle',
    animation: 150,
    onEnd: function() {
      var items = catList.querySelectorAll('.manage-category[data-id]');
      var payload = Array.from(items).map(function(el, idx) {
        return {id: parseInt(el.dataset.id), sort_order: idx};
      });
      fetch('/manage/sort/category/reorder', {
        method: 'POST',
        headers: {'Content-Type': 'application/json', 'X-CSRF-Token': (window._getCookie || function(n){return ''})('hp_csrf')},
        body: JSON.stringify(payload)
      });
    }
  });

  catList.querySelectorAll('.sortable-services').forEach(function(svcList) {
    Sortable.create(svcList, {
      handle: '.drag-handle',
      group: 'services',
      animation: 150,
      onEnd: function(evt) {
        var destList = evt.to;
        var destCatId = parseInt(destList.dataset.cat);
        var payload = Array.from(destList.querySelectorAll('.manage-service-item[data-id]')).map(function(el, idx) {
          return {id: parseInt(el.dataset.id), sort_order: idx, category_id: destCatId};
        });
        if (evt.from !== evt.to) {
          var srcList = evt.from;
          var srcCatId = parseInt(srcList.dataset.cat);
          Array.from(srcList.querySelectorAll('.manage-service-item[data-id]')).forEach(function(el, idx) {
            payload.push({id: parseInt(el.dataset.id), sort_order: idx, category_id: srcCatId});
          });
        }
        fetch('/manage/sort/service/reorder', {
          method: 'POST',
          headers: {'Content-Type': 'application/json', 'X-CSRF-Token': (window._getCookie || function(n){return ''})('hp_csrf')},
          body: JSON.stringify(payload)
        });
      }
    });
  });
}

// ── Favicon edit (called from event delegation above) ──────────────
async function fetchFaviconEdit(svcId, btn) {
  const urlInput = document.querySelector('#svc-edit-' + svcId + ' input[name="url"]');
  const iconInput = document.getElementById('edit-icon-' + svcId);
  const preview = document.getElementById('edit-favicon-preview-' + svcId);
  if (!urlInput || !urlInput.value.trim()) return;

  btn.textContent = '⏳';
  const faviconURL = '/api/favicon?url=' + encodeURIComponent(urlInput.value.trim());
  try {
    const resp = await fetch(faviconURL);
    if (resp.ok) {
      iconInput.value = faviconURL;
      preview.src = faviconURL;
      preview.style.display = 'inline';
      btn.textContent = '✅';
    } else {
      btn.textContent = '❌';
    }
  } catch {
    btn.textContent = '❌';
  }
  setTimeout(function() { btn.textContent = '🌐'; }, 2000);
}

// ── Theme Engine ──────────────────────────────────────────────────
async function savePrefs(patch) {
  var section = document.getElementById('appearance');
  var saveAll = section && section.dataset.saveAll === '1';
  var profile = document.body.dataset.profile || '';
  var url = '/api/user/preferences' + (saveAll ? '?all=1' : (profile ? '?profile=' + encodeURIComponent(profile) : ''));
  try {
    var resp = await fetch(url, {
      method: 'PATCH',
      headers: {'Content-Type': 'application/json', 'X-CSRF-Token': (window._getCookie || function(){return '';})('hp_csrf')},
      body: JSON.stringify(patch)
    });
    if (!resp.ok) { console.warn('prefs save failed', resp.status); }
  } catch(e) { console.warn('prefs save failed', e); }
}

function setTheme(theme) {
  if (theme === 'system') {
    delete document.documentElement.dataset.theme;
  } else {
    document.documentElement.dataset.theme = theme;
  }
  localStorage.setItem('hp-theme', theme);
  document.querySelectorAll('.theme-btn').forEach(function(b) {
    b.classList.toggle('active', b.dataset.theme === theme);
  });
  savePrefs({theme: theme});
}

function setBgMode(mode) {
  document.body.dataset.bg = mode;
  document.querySelectorAll('.bg-btn').forEach(function(b) {
    b.classList.toggle('active', b.dataset.bg === mode);
  });
  var auroraPanel = document.getElementById('aurora-options');
  if (auroraPanel) auroraPanel.classList.toggle('hidden', mode !== 'aurora');
  savePrefs({background_mode: mode});
}

function previewAccent(hex) {
  document.getElementById('accent-hex').textContent = hex;
  const rgb = hexToRGBjs(hex);
  document.documentElement.style.setProperty('--accent', hex);
  document.documentElement.style.setProperty('--accent-hover', hex);
  document.documentElement.style.setProperty('--accent-rgb', rgb);
}

function saveAccent(hex) {
  previewAccent(hex);
  savePrefs({accent_color: hex});
  var link = document.getElementById('user-theme');
  var slug = document.body.dataset.profile || 'default';
  if (link) link.href = '/api/profile/' + encodeURIComponent(slug) + '/theme.css?t=' + Date.now();
}

function resetAccent() {
  var hex = '#6366f1';
  var picker = document.getElementById('accent-picker');
  if (picker) picker.value = hex;
  saveAccent(hex);
}

async function saveCustomCSS() {
  const css = document.getElementById('custom-css-input').value;
  await savePrefs({custom_css: css});
  var link = document.getElementById('user-theme');
  var slug = document.body.dataset.profile || 'default';
  if (link) link.href = '/api/profile/' + encodeURIComponent(slug) + '/theme.css?t=' + Date.now();
}

function resetCustomCSS() {
  document.getElementById('custom-css-input').value = '';
  saveCustomCSS();
}

function hexToRGBjs(hex) {
  hex = hex.replace('#','');
  if (hex.length !== 6) return '99,102,241';
  const r = parseInt(hex.slice(0,2),16), g = parseInt(hex.slice(2,4),16), b = parseInt(hex.slice(4,6),16);
  return r + ',' + g + ',' + b;
}
