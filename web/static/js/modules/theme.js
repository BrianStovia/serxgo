/**
 * SearXGo Module: Theme & Service Worker
 */
  // --- Theme Management ---
  function initTheme() {
    const savedTheme = localStorage.getItem('searxgo_theme') || getCookie('searxgo_theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
    updateThemeToggleIcons();
  }

  window.setTheme = function (theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('searxgo_theme', theme);
    setCookie('searxgo_theme', theme, 365);
    updateThemeToggleIcons();
  };

  window.toggleTheme = function () {
    const current = document.documentElement.getAttribute('data-theme') || 'dark';
    const next = current === 'light' ? 'dark' : 'light';
    window.setTheme(next);
  };

  function updateThemeToggleIcons() {
    const current = document.documentElement.getAttribute('data-theme') || 'dark';
    document.querySelectorAll('.btn-toggle-theme').forEach(btn => {
      btn.innerHTML = current === 'light' ? '<span>🌙</span>' : '<span>☀️</span>';
      btn.setAttribute('title', current === 'light' ? 'Switch to Dark Mode' : 'Switch to Light Mode');
    });
  }

  document.addEventListener('click', function (e) {
    if (e.target.closest('.btn-toggle-theme')) {
      e.preventDefault();
      window.toggleTheme();
    }
  });

  // --- Cookie Helpers ---
  function setCookie(name, value, days) {
    let expires = '';
    if (days) {
      const date = new Date();
      date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
      expires = '; expires=' + date.toUTCString();
    }
    document.cookie = name + '=' + (value || '') + expires + '; path=/; SameSite=Lax';
  }

  function getCookie(name) {
    const nameEQ = name + '=';
    const ca = document.cookie.split(';');
    for (let i = 0; i < ca.length; i++) {
      let c = ca[i];
      while (c.charAt(0) === ' ') c = c.substring(1, c.length);
      if (c.indexOf(nameEQ) === 0) return c.substring(nameEQ.length, c.length);
    }
    return null;
  }


  // --- PWA Service Worker Registration ---
  let swInitialized = false;
  function initServiceWorker() {
    if (swInitialized) return;
    swInitialized = true;
    if ('serviceWorker' in navigator) {
      if (document.readyState === 'complete') {
        navigator.serviceWorker.register('/sw.js').catch(() => {});
      } else {
        window.addEventListener('load', () => {
          navigator.serviceWorker.register('/sw.js').catch(() => {});
        });
      }
    }
  }


