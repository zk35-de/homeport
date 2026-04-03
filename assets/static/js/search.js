const BANGS = {
  '!g':  'https://www.google.com/search?q=',
  '!d':  'https://duckduckgo.com/?q=',
  '!ddg':'https://duckduckgo.com/?q=',
  '!b':  'https://search.brave.com/search?q=',
  '!gh': 'https://github.com/search?q=',
  '!yt': 'https://www.youtube.com/results?search_query=',
  '!w':  'https://de.wikipedia.org/w/index.php?search=',
  '!wp': 'https://en.wikipedia.org/w/index.php?search=',
  '!sp': 'https://www.startpage.com/search?q=',
};

let currentEngineURL = (function() {
  const action = document.getElementById('search-form')?.action || 'https://duckduckgo.com/?q=';
  if (action.includes('?')) return action + (action.endsWith('?') ? 'q=' : '&q=');
  return action + '?q=';
})();

document.addEventListener('DOMContentLoaded', function() {
  const searchForm  = document.getElementById('search-form');
  const searchInput = document.getElementById('search-input');
  const spotlight   = document.getElementById('search-spotlight');
  const engineBtn   = document.getElementById('search-engine-btn');
  const engineLabel = document.getElementById('search-engine-label');
  const engineMenu  = document.getElementById('search-engine-menu');

  // ── Suchanbieter-Dropdown ─────────────────────────────────────────
  engineBtn?.addEventListener('click', function(e) {
    e.stopPropagation();
    engineMenu.classList.toggle('hidden');
  });

  document.querySelectorAll('.se-option').forEach(function(btn) {
    btn.addEventListener('mousedown', function(e) {
      e.preventDefault();
      const url   = btn.dataset.url;
      const label = btn.dataset.label;
      currentEngineURL = url;
      if (engineLabel) engineLabel.textContent = label + '▾';
      engineMenu.classList.add('hidden');
      const profile = document.body.dataset.profile || '';
      const apiUrl  = '/api/user/preferences' + (profile ? '?profile=' + encodeURIComponent(profile) : '');
      const csrf    = document.cookie.match('(^|;)\\s*hp_csrf\\s*=\\s*([^;]+)');
      fetch(apiUrl, {
        method: 'PATCH',
        headers: Object.assign({'Content-Type': 'application/json'}, csrf ? {'X-CSRF-Token': csrf.pop()} : {}),
        body: JSON.stringify({search_engine: url.replace('?q=', '').replace('&q=', '')})
      }).catch(function() {});
    });
  });

  document.addEventListener('click', function() {
    engineMenu?.classList.add('hidden');
  });

  // ── Spotlight-Suche ───────────────────────────────────────────────
  let spotlightIdx = -1;
  let spotlightItems = [];

  function getServices() {
    const results = [];
    document.querySelectorAll('.service-card').forEach(function(a) {
      const name = a.querySelector('.service-name')?.textContent?.trim();
      const desc = a.querySelector('.service-desc')?.textContent?.trim() || '';
      const cat  = a.closest('.category')?.querySelector('.category-title')?.textContent?.trim() || '';
      const iconEl = a.querySelector('.service-icon');
      const imgEl  = iconEl?.querySelector('img');
      const icon   = imgEl ? imgEl.src : (iconEl?.textContent?.trim() || '🔗');
      const isImg  = !!imgEl;
      if (name) results.push({ name, desc, cat, url: a.href, icon, isImg });
    });
    return results;
  }

  function fuzzy(str, q) {
    str = str.toLowerCase(); q = q.toLowerCase();
    let si = 0;
    for (let i = 0; i < q.length; i++) {
      si = str.indexOf(q[i], si);
      if (si === -1) return false; si++;
    }
    return true;
  }

  let _allServices = null;
  function getServicesCached() {
    if (_allServices !== null) return _allServices;
    _allServices = getServices();
    return _allServices;
  }

  function renderSpotlight(query) {
    if (!spotlight) return;
    const allSvcs = getServicesCached();
    const svcs = query
      ? allSvcs.filter(function(s) { return fuzzy(s.name, query) || fuzzy(s.desc, query) || fuzzy(s.cat, query); })
      : allSvcs;
    spotlightItems = svcs.slice(0, 8);
    spotlightIdx = -1;

    if (!query && !allSvcs.length) { spotlight.style.display = 'none'; return; }

    spotlight.innerHTML = spotlightItems.map(function(s, i) {
      const iconHtml = s.isImg
        ? '<img src="' + s.icon + '" width="16" height="16" style="border-radius:3px;flex-shrink:0">'
        : '<span style="flex-shrink:0">' + s.icon + '</span>';
      return '<li class="spotlight-item" data-idx="' + i + '" data-url="' + s.url + '">' +
        iconHtml +
        '<span class="spotlight-name">' + s.name + '</span>' +
        '<span class="spotlight-cat">' + s.cat + '</span>' +
        '</li>';
    }).join('');

    if (query) {
      spotlight.innerHTML += '<li class="spotlight-web" data-web="1">🔍 Im Web suchen: „' + query + '"</li>';
    }

    spotlight.querySelectorAll('.spotlight-item').forEach(function(li) {
      li.addEventListener('mousedown', function(e) { e.preventDefault(); window.location.href = li.dataset.url; });
      li.addEventListener('mouseenter', function() { setSpotlightActive(parseInt(li.dataset.idx)); });
    });
    spotlight.querySelector('.spotlight-web')?.addEventListener('mousedown', function(e) {
      e.preventDefault(); submitSearch(query);
    });

    if (spotlight.innerHTML.trim()) spotlight.style.display = 'block';
    else spotlight.style.display = 'none';
  }

  function setSpotlightActive(idx) {
    spotlightIdx = idx;
    spotlight.querySelectorAll('.spotlight-item').forEach(function(li, i) {
      li.classList.toggle('active', i === idx);
    });
  }

  function submitSearch(query) {
    const parts = query.split(/\s+/);
    let bang = null, rest = query;
    if (BANGS[parts[0]]) { bang = parts[0]; rest = parts.slice(1).join(' '); }
    else if (BANGS[parts[parts.length-1]]) { bang = parts[parts.length-1]; rest = parts.slice(0,-1).join(' '); }
    if (bang) { window.open(BANGS[bang] + encodeURIComponent(rest), '_blank'); }
    else { window.open(currentEngineURL + encodeURIComponent(query), '_blank'); }
  }

  searchInput?.addEventListener('focus', function() { if (searchInput.value.trim()) renderSpotlight(searchInput.value.trim()); });
  searchInput?.addEventListener('input', function() { _allServices = null; renderSpotlight(searchInput.value.trim()); });
  searchInput?.addEventListener('blur', function() { setTimeout(function() { spotlight.style.display = 'none'; }, 200); });

  searchInput?.addEventListener('keydown', function(e) {
    const showing = spotlight && spotlight.style.display !== 'none';
    if (e.key === 'ArrowDown' && showing) {
      e.preventDefault();
      setSpotlightActive(Math.min(spotlightIdx + 1, spotlightItems.length - 1));
    } else if (e.key === 'ArrowUp' && showing) {
      e.preventDefault();
      setSpotlightActive(Math.max(spotlightIdx - 1, 0));
    } else if (e.key === 'Escape') {
      if (spotlight) spotlight.style.display = 'none';
      searchInput.value = '';
    } else if (e.key === 'Enter') {
      if (showing && spotlightIdx >= 0 && spotlightItems[spotlightIdx]) {
        e.preventDefault();
        window.location.href = spotlightItems[spotlightIdx].url;
      } else {
        e.preventDefault();
        spotlight.style.display = 'none';
        submitSearch(searchInput.value.trim());
      }
    }
  });

  searchForm?.addEventListener('submit', function(e) { e.preventDefault(); });

  document.addEventListener('keydown', function(e) {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
    if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault(); searchInput?.focus();
    } else if (e.key === '/') {
      e.preventDefault(); searchInput?.focus();
    }
  });

  // Collapsible categories – restore state
  document.querySelectorAll('.category[data-cat-id]').forEach(function(el) {
    const id = el.dataset.catId;
    if (localStorage.getItem('hp-cat-' + id) === '1') {
      _collapseCategory(id, false);
    }
  });

  // Status SSE
  const evtSource = new EventSource('/status/stream');
  evtSource.onmessage = function(event) {
    try {
      const data = JSON.parse(event.data);
      const el = document.getElementById('service-' + data.id);
      if (el) {
        el.classList.remove('status-alive', 'status-dead');
        el.classList.add(data.alive ? 'status-alive' : 'status-dead');
      }
    } catch (e) {
      console.error('Error parsing SSE data', e);
    }
  };

  // Page tabs: restore last active
  const PAGE_KEY = 'hp-active-page-' + (document.body.dataset.profile || '');
  const pageTabs = document.getElementById('page-tabs');
  if (pageTabs) {
    const saved = parseInt(localStorage.getItem(PAGE_KEY) || '0', 10);
    applyPage(saved);

    document.addEventListener('keydown', function(e) {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
      if (e.key === '0' || e.key === '`') { switchPage(0); return; }
      const n = parseInt(e.key, 10);
      if (n >= 1 && n <= 9) {
        const btns = Array.from(pageTabs.querySelectorAll('.page-tab[data-page]'))
          .filter(function(b) { return parseInt(b.dataset.page, 10) !== 0; });
        if (btns[n - 1]) switchPage(parseInt(btns[n-1].dataset.page, 10));
      }
    });
  }
});

// Category collapse/expand – event delegation (removes onclick from template)
document.addEventListener('click', function(e) {
  var header = e.target.closest('.category-header');
  if (header) {
    var cat = header.closest('.category[data-cat-id]');
    if (cat) toggleCategory(cat.dataset.catId);
  }
});

// Page tab click – event delegation (removes onclick from template)
document.addEventListener('click', function(e) {
  var btn = e.target.closest('.page-tab[data-page]');
  if (btn && document.getElementById('page-tabs')) {
    switchPage(parseInt(btn.dataset.page, 10));
  }
});

function _collapseCategory(id, animate) {
  const body  = document.getElementById('cat-body-' + id);
  const arrow = document.getElementById('cat-arrow-' + id);
  if (!body) return;
  if (!animate) body.style.transition = 'none';
  body.style.maxHeight = '0';
  body.style.overflow  = 'hidden';
  if (arrow) arrow.classList.add('collapsed');
  if (!animate) requestAnimationFrame(function() { body.style.transition = ''; });
}

function _expandCategory(id) {
  const body  = document.getElementById('cat-body-' + id);
  const arrow = document.getElementById('cat-arrow-' + id);
  if (!body) return;
  body.style.maxHeight = body.scrollHeight + 'px';
  body.style.overflow  = 'hidden';
  if (arrow) arrow.classList.remove('collapsed');
  body.addEventListener('transitionend', function h() {
    body.style.maxHeight = '';
    body.style.overflow  = '';
    body.removeEventListener('transitionend', h);
  });
}

function toggleCategory(id) {
  const body = document.getElementById('cat-body-' + id);
  if (!body) return;
  const isCollapsed = body.style.maxHeight === '0px' || body.style.maxHeight === '0';
  if (isCollapsed) {
    _expandCategory(id);
    localStorage.removeItem('hp-cat-' + id);
  } else {
    body.style.maxHeight = body.scrollHeight + 'px';
    body.style.overflow  = 'hidden';
    requestAnimationFrame(function() { body.style.maxHeight = '0'; });
    document.getElementById('cat-arrow-' + id)?.classList.add('collapsed');
    localStorage.setItem('hp-cat-' + id, '1');
  }
}

function switchPage(pageID) {
  const PAGE_KEY = 'hp-active-page-' + (document.body.dataset.profile || '');
  localStorage.setItem(PAGE_KEY, pageID);
  applyPage(pageID);
}

function applyPage(pageID) {
  pageID = parseInt(pageID, 10);
  document.querySelectorAll('.page-tab').forEach(function(btn) {
    btn.classList.toggle('active', parseInt(btn.dataset.page, 10) === pageID);
  });
  document.querySelectorAll('.dashboard .category[data-page]').forEach(function(el) {
    const elPage = parseInt(el.dataset.page, 10);
    const show = (pageID === 0) || (elPage === 0) || (elPage === pageID);
    el.style.display = show ? '' : 'none';
  });
  document.querySelectorAll('.widget-page-item[data-widget-page]').forEach(function(el) {
    const elPage = parseInt(el.dataset.widgetPage, 10);
    const show = (pageID === 0) || (elPage === 0) || (elPage === pageID);
    el.style.display = show ? '' : 'none';
  });
}
