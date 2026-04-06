// Service Worker
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js');
}

// HTMX config: disable eval (no inline hx-on::* expressions in this app)
if (typeof htmx !== 'undefined') { htmx.config.allowEval = false; }

// CSRF: inject cookie value as X-CSRF-Token header on every HTMX request
(function() {
  function getCookie(name) {
    var v = document.cookie.match('(^|;)\\s*' + name + '\\s*=\\s*([^;]+)');
    return v ? v.pop() : '';
  }
  window._getCookie = getCookie;

  document.addEventListener('htmx:configRequest', function(e) {
    var token = getCookie('hp_csrf');
    if (token) e.detail.headers['X-CSRF-Token'] = token;
  });
  document.addEventListener('htmx:beforeRequest', function() {
    document.documentElement.classList.add('htmx-loading');
  });
  document.addEventListener('htmx:afterRequest', function() {
    document.documentElement.classList.remove('htmx-loading');
  });
})();

// Clock widget runtime
(function() {
  function pad(n) { return String(n).padStart(2, '0'); }

  var _LANG_LOCALE = {de:'de-DE', en:'en-US', es:'es-ES', sk:'sk-SK', fr:'fr-FR'};
  function formatDate(d, tz) {
    var lang = document.documentElement.lang || 'de';
    var locale = _LANG_LOCALE[lang] || 'de-DE';
    return d.toLocaleDateString(locale, {
      weekday: 'long', year: 'numeric', month: 'long', day: 'numeric', timeZone: tz
    });
  }

  function tickClock(el) {
    const mode = el.dataset.mode || 'digital';
    const tz   = el.dataset.timezone || 'Europe/Berlin';
    const showDate = el.dataset.showDate !== 'false';
    const showSec  = el.dataset.showSeconds !== 'false';
    const now = new Date();
    const dateEl = el.querySelector('[id^="clock-date-"]');

    if (mode === 'analog') {
      const h = now.getHours() % 12, m = now.getMinutes(), s = now.getSeconds();
      const ha = (h * 30) + (m * 0.5), ma = m * 6, sa = s * 6;
      const hourLine   = el.querySelector('.clock-hour');
      const minuteLine = el.querySelector('.clock-minute');
      const secondLine = el.querySelector('.clock-second');
      if (hourLine)   setHand(hourLine, ha, 28);
      if (minuteLine) setHand(minuteLine, ma, 35);
      if (secondLine) setHand(secondLine, sa, 38);
      if (dateEl && showDate) dateEl.textContent = formatDate(now, tz);
      return;
    }

    if (mode === 'countdown') {
      const target = el.dataset.countdown;
      const digitalEl = el.querySelector('[id^="clock-digital-"]');
      const labelEl   = el.querySelector('[id^="clock-label-"]');
      if (target && digitalEl) {
        const diff = new Date(target + 'T00:00:00') - now;
        if (diff > 0) {
          const days = Math.floor(diff / 86400000);
          const hrs  = Math.floor((diff % 86400000) / 3600000);
          const mins = Math.floor((diff % 3600000) / 60000);
          const secs = Math.floor((diff % 60000) / 1000);
          digitalEl.textContent = days + 'd ' + pad(hrs) + ':' + pad(mins) + ':' + pad(secs);
        } else {
          digitalEl.textContent = '00d 00:00:00';
        }
      }
      if (labelEl && target) {
        var _l = document.documentElement.lang || 'de';
        var _loc = _LANG_LOCALE[_l] || 'de-DE';
        labelEl.textContent = new Date(target).toLocaleDateString(_loc);
      }
      return;
    }

    // digital
    const digitalEl = el.querySelector('[id^="clock-digital-"]');
    if (digitalEl) {
      const h = pad(now.getHours()), m = pad(now.getMinutes()), s = pad(now.getSeconds());
      digitalEl.textContent = showSec ? h + ':' + m + ':' + s : h + ':' + m;
    }
    if (dateEl && showDate) dateEl.textContent = formatDate(now, tz);
  }

  function setHand(line, angle, length) {
    const rad = (angle - 90) * Math.PI / 180;
    const x2 = 50 + length * Math.cos(rad);
    const y2 = 50 + length * Math.sin(rad);
    line.setAttribute('x2', x2.toFixed(2));
    line.setAttribute('y2', y2.toFixed(2));
  }

  function initClocks() {
    document.querySelectorAll('.widget-clock').forEach(function(el) {
      tickClock(el);
      setInterval(function() { tickClock(el); }, 1000);
    });
  }

  document.addEventListener('DOMContentLoaded', initClocks);
})();

// Language toggle (nav button) – cycles through all supported languages
var _SUPPORTED_LANGS = ['de', 'en', 'es', 'sk', 'fr'];
function toggleLang() {
  var cur = localStorage.getItem('hp-lang') || 'de';
  var idx = _SUPPORTED_LANGS.indexOf(cur);
  var next = _SUPPORTED_LANGS[(idx + 1) % _SUPPORTED_LANGS.length];
  localStorage.setItem('hp-lang', next);
  document.querySelectorAll('.nav-lang-toggle').forEach(function(b) { b.textContent = next.toUpperCase(); });
  try {
    fetch('/api/user/preferences', {
      method: 'PATCH',
      headers: {'Content-Type': 'application/json', 'X-CSRF-Token': window._getCookie('hp_csrf')},
      body: JSON.stringify({language: next})
    }).then(function() { window.location.reload(); });
  } catch(e) { window.location.reload(); }
}

// Theme toggle (nav button)
function cycleTheme() {
  var cur = localStorage.getItem('hp-theme') || 'dark';
  var next = cur === 'dark' ? 'light' : cur === 'light' ? 'system' : 'dark';
  if (next === 'system') {
    delete document.documentElement.dataset.theme;
  } else {
    document.documentElement.dataset.theme = next;
  }
  localStorage.setItem('hp-theme', next);
  var icons = {'dark':'🌙','light':'☀️','system':'💻'};
  document.querySelectorAll('.nav-theme-toggle').forEach(function(b) { b.textContent = icons[next]; });
  try {
    fetch('/api/user/preferences', {
      method: 'PATCH',
      headers: {'Content-Type': 'application/json', 'X-CSRF-Token': window._getCookie('hp_csrf')},
      body: JSON.stringify({theme: next})
    });
  } catch(e) {}
}

// Init nav toggle icons + attach click listeners
document.addEventListener('DOMContentLoaded', function() {
  var t = localStorage.getItem('hp-theme') || 'dark';
  var icons = {'dark':'🌙','light':'☀️','system':'💻'};
  document.querySelectorAll('.nav-theme-toggle').forEach(function(b) {
    b.textContent = icons[t] || '🌙';
    b.addEventListener('click', cycleTheme);
  });

  document.querySelectorAll('.nav-lang-toggle').forEach(function(b) {
    var serverLang = b.dataset.lang || '';
    var lang = serverLang || localStorage.getItem('hp-lang') || 'de';
    if (serverLang) localStorage.setItem('hp-lang', serverLang);
    b.textContent = lang.toUpperCase();
    b.addEventListener('click', toggleLang);
  });
});

// manage.html widget form toggle
function updateWidgetForm() {
  const sel = document.getElementById('widget-type-select');
  if (!sel) return;
  const types = ['ical', 'caldav', 'clock', 'todo', 'bookmarks', 'notes'];
  types.forEach(function(t) {
    const el = document.getElementById('widget-' + t + '-fields');
    if (el) el.style.display = sel.value === t ? '' : 'none';
  });
  const modeEl = document.querySelector('select[name="clock_mode"]');
  if (modeEl) updateClockMode(modeEl);
}
function updateClockMode(sel) {
  const cg = document.getElementById('clock-countdown-group');
  if (cg) cg.style.display = sel && sel.value === 'countdown' ? '' : 'none';
}

// Analytics profile filter (used on analytics page)
document.addEventListener('DOMContentLoaded', function() {
  var analyticsSelect = document.getElementById('analytics-profile-select');
  if (analyticsSelect) {
    analyticsSelect.addEventListener('change', function() {
      window.location.href = '/manage/analytics?profile=' + encodeURIComponent(this.value);
    });
  }
});

// Favicon error fallback (hide broken images with class a-favicon)
document.addEventListener('error', function(e) {
  if (e.target.tagName === 'IMG' && e.target.classList.contains('a-favicon')) {
    e.target.style.display = 'none';
  }
}, true);

// Cancel-edit buttons: data-clear-target="element-id" → clear that element's innerHTML
document.addEventListener('click', function(e) {
  var btn = e.target.closest('[data-clear-target]');
  if (btn) {
    var el = document.getElementById(btn.dataset.clearTarget);
    if (el) el.innerHTML = '';
  }
});
