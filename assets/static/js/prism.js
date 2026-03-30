/* PRISM UI v2.0 – JS Companion – secalpha/prism-ui */
/* Vanilla JS, keine Dependencies. ~60 Zeilen. */

const Prism = (() => {
  'use strict';

  /* ── Toast ── */
  function toast(message, type = '', duration = 3500) {
    let container = document.querySelector('.toast-container');
    if (!container) {
      container = document.createElement('div');
      container.className = 'toast-container';
      document.body.appendChild(container);
    }

    const el = document.createElement('div');
    el.className = ['toast', type].filter(Boolean).join(' ');
    el.textContent = message;
    container.appendChild(el);

    const dismiss = () => {
      el.style.transition = 'opacity 0.25s, transform 0.25s';
      el.style.opacity = '0';
      el.style.transform = 'translateY(8px)';
      setTimeout(() => el.remove(), 260);
    };

    const timer = setTimeout(dismiss, duration);
    el.addEventListener('click', () => { clearTimeout(timer); dismiss(); });

    return el;
  }

  /* ── Tabs ── */
  function initTabs(container) {
    if (typeof container === 'string') {
      container = document.querySelector(container);
    }
    if (!container) return;

    const items = Array.from(container.querySelectorAll('.tab-item'));

    items.forEach(item => {
      item.addEventListener('click', () => {
        items.forEach(i => i.classList.remove('active'));
        item.classList.add('active');

        // Panel switching: data-tab="name" auf .tab-item,
        // data-tab-panel="name" auf zugehörigem Panel
        const target = item.dataset.tab;
        if (target) {
          const scope = container.closest('[data-tabs-scope]') || document;
          scope.querySelectorAll('[data-tab-panel]').forEach(panel => {
            panel.hidden = panel.dataset.tabPanel !== target;
          });
        }

        container.dispatchEvent(new CustomEvent('prism:tab', {
          bubbles: true,
          detail: { tab: item.dataset.tab, el: item }
        }));
      });
    });

    // Erstes Panel initial sichtbar, Rest hidden
    const firstActive = items.find(i => i.classList.contains('active')) || items[0];
    if (firstActive?.dataset.tab) {
      const scope = container.closest('[data-tabs-scope]') || document;
      scope.querySelectorAll('[data-tab-panel]').forEach(panel => {
        panel.hidden = panel.dataset.tabPanel !== firstActive.dataset.tab;
      });
    }
  }

  /* ── Auto-Init ── */
  document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('.tabs').forEach(initTabs);
  });

  return { toast, initTabs };
})();
