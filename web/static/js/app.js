/**
 * SearXGo Pro - Client Application Script
 */

(function () {
  'use strict';

  // --- Theme Management ---
  function initTheme() {
    const savedTheme = localStorage.getItem('searxgo_theme') || getCookie('searxgo_theme') || 'dark';
    document.documentElement.setAttribute('data-theme', savedTheme);
  }

  window.setTheme = function (theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('searxgo_theme', theme);
    setCookie('searxgo_theme', theme, 365);
  };

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

  // --- Autocomplete & Search Box ---
  function initSearchBox() {
    const input = document.querySelector('.search-input');
    const clearBtn = document.querySelector('.btn-clear');
    const dropdown = document.querySelector('.suggestions-dropdown');
    const form = document.querySelector('.search-form');

    if (!input || !dropdown) return;

    let selectedIndex = -1;
    let debounceTimer = null;

    function updateClearBtn() {
      if (clearBtn) {
        clearBtn.style.display = input.value.trim().length > 0 ? 'inline-flex' : 'none';
      }
    }

    input.addEventListener('input', function () {
      updateClearBtn();
      const val = input.value.trim();

      clearTimeout(debounceTimer);
      if (val.length < 1 || (!val.startsWith('!') && !val.startsWith(':') && val.length < 2)) {
        hideDropdown();
        return;
      }

      debounceTimer = setTimeout(() => {
        fetchSuggestions(val);
      }, 150);
    });

    if (clearBtn) {
      clearBtn.addEventListener('click', function () {
        input.value = '';
        updateClearBtn();
        hideDropdown();
        input.focus();
      });
    }

    input.addEventListener('keydown', function (e) {
      const items = dropdown.querySelectorAll('.suggestion-item');
      if (!dropdown.classList.contains('active') || items.length === 0) {
        return;
      }

      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedIndex = (selectedIndex + 1) % items.length;
        updateSelection(items);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedIndex = (selectedIndex - 1 + items.length) % items.length;
        updateSelection(items);
      } else if (e.key === 'Escape') {
        hideDropdown();
      } else if (e.key === 'Enter') {
        if (selectedIndex >= 0 && selectedIndex < items.length) {
          e.preventDefault();
          const targetVal = items[selectedIndex].dataset.val;
          applySuggestionValue(targetVal);
        }
      }
    });

    function applySuggestionValue(rawVal) {
      let insertVal = rawVal;
      if (rawVal.includes(' (')) {
        insertVal = rawVal.split(' (')[0] + ' ';
      }
      input.value = insertVal;
      hideDropdown();
      input.focus();
      if (!insertVal.startsWith('!') && !insertVal.startsWith(':')) {
        if (form) form.submit();
      }
    }

    function updateSelection(items) {
      items.forEach((it, idx) => {
        if (idx === selectedIndex) {
          it.classList.add('selected');
          let val = it.dataset.val;
          if (val.includes(' (')) {
            val = val.split(' (')[0] + ' ';
          }
          input.value = val;
        } else {
          it.classList.remove('selected');
        }
      });
    }

    function fetchSuggestions(query) {
      fetch('/api/suggest?q=' + encodeURIComponent(query))
        .then(res => res.json())
        .then(data => {
          if (Array.isArray(data) && data.length > 0) {
            renderSuggestions(data);
          } else {
            hideDropdown();
          }
        })
        .catch(() => {
          hideDropdown();
        });
    }

    function renderSuggestions(items) {
      dropdown.innerHTML = '';
      selectedIndex = -1;

      items.forEach(text => {
        const li = document.createElement('li');
        li.className = 'suggestion-item';
        li.dataset.val = text;
        li.innerHTML = `
          <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" stroke-width="2" fill="none">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
          <span>${escapeHtml(text)}</span>
        `;

        li.addEventListener('mousedown', function (e) {
          e.preventDefault();
          applySuggestionValue(text);
        });

        dropdown.appendChild(li);
      });

      dropdown.classList.add('active');
    }

    function hideDropdown() {
      dropdown.classList.remove('active');
      dropdown.innerHTML = '';
      selectedIndex = -1;
    }

    document.addEventListener('click', function (e) {
      if (!input.contains(e.target) && !dropdown.contains(e.target)) {
        hideDropdown();
      }
    });

    updateClearBtn();
  }

  function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  // --- Interactive Filter Toolbar ---
  function initFilterToolbar() {
    const timeSel = document.getElementById('filter-time');
    const safeSel = document.getElementById('filter-safesearch');
    const langSel = document.getElementById('filter-language');

    function applyFilters() {
      const url = new URL(window.location.href);
      if (timeSel) {
        if (timeSel.value) url.searchParams.set('time_range', timeSel.value);
        else url.searchParams.delete('time_range');
      }
      if (safeSel) {
        url.searchParams.set('safesearch', safeSel.value);
      }
      if (langSel) {
        if (langSel.value) url.searchParams.set('language', langSel.value);
        else url.searchParams.delete('language');
      }
      url.searchParams.set('page', '1');
      window.location.href = url.toString();
    }

    if (timeSel) timeSel.addEventListener('change', applyFilters);
    if (safeSel) safeSel.addEventListener('change', applyFilters);
    if (langSel) langSel.addEventListener('change', applyFilters);
  }

  // --- Interactive Leaflet Map for Category Maps ---
  function initLeafletMap() {
    const mapEl = document.getElementById('interactive-map');
    if (!mapEl || typeof L === 'undefined') return;

    const items = document.querySelectorAll('.result-item[data-lat]');
    if (items.length === 0) return;

    const map = L.map('interactive-map').setView([0, 0], 2);
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '© OpenStreetMap contributors'
    }).addTo(map);

    const bounds = [];
    items.forEach(it => {
      const coordStr = it.dataset.lat;
      if (!coordStr) return;
      const parts = coordStr.split(',');
      if (parts.length === 2) {
        const lat = parseFloat(parts[0]);
        const lon = parseFloat(parts[1]);
        if (!isNaN(lat) && !isNaN(lon)) {
          const title = it.querySelector('.result-title a')?.textContent || 'Location';
          const marker = L.marker([lat, lon]).addTo(map).bindPopup(`<b>${escapeHtml(title)}</b>`);
          bounds.push([lat, lon]);
        }
      }
    });

    if (bounds.length > 0) {
      map.fitBounds(bounds, { padding: [40, 40], maxZoom: 15 });
    }
  }

  // --- Video Modal Handler ---
  function initVideoModal() {
    const modal = document.getElementById('video-modal');
    const iframe = document.getElementById('modal-iframe');
    const title = document.getElementById('modal-video-title');
    const closeBtn = document.getElementById('btn-close-modal');

    if (!modal || !iframe) return;

    document.querySelectorAll('.video-thumb-wrap').forEach(wrap => {
      wrap.addEventListener('click', function () {
        const url = this.dataset.videoUrl;
        const vidTitle = this.dataset.videoTitle || 'Video Player';
        if (url) {
          iframe.src = url;
          if (title) title.textContent = vidTitle;
          modal.style.display = 'flex';
        }
      });
    });

    function closeModal() {
      modal.style.display = 'none';
      iframe.src = '';
    }

    if (closeBtn) closeBtn.addEventListener('click', closeModal);
    modal.addEventListener('click', function (e) {
      if (e.target === modal) closeModal();
    });
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && modal.style.display === 'flex') {
        closeModal();
      }
    });
  }

  // --- Magnet Link Handler ---
  function initMagnetButtons() {
    document.querySelectorAll('.btn-magnet').forEach(btn => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        const magnet = this.dataset.magnet;
        if (navigator.clipboard && magnet) {
          navigator.clipboard.writeText(magnet).then(() => {
            const orig = btn.textContent;
            btn.textContent = '✓ Copied!';
            setTimeout(() => { btn.textContent = orig; }, 2000);
          });
        }
      });
    });
  }

  // --- Universal Toast Notification Helper ---
  function showToast(msg, icon = '✓') {
    let toast = document.getElementById('searxgo-toast');
    if (!toast) {
      toast = document.createElement('div');
      toast.id = 'searxgo-toast';
      toast.className = 'searxgo-toast';
      document.body.appendChild(toast);
    }
    toast.innerHTML = `<span style="font-size:1.1rem;">${icon}</span><span>${msg}</span>`;
    toast.classList.add('show');
    clearTimeout(toast._timer);
    toast._timer = setTimeout(() => {
      toast.classList.remove('show');
    }, 2800);
  }

  // --- Keyboard Shortcuts Cheatsheet Modal ---
  function showShortcutsModal() {
    let modal = document.getElementById('searxgo-shortcuts-modal');
    if (!modal) {
      modal = document.createElement('div');
      modal.id = 'searxgo-shortcuts-modal';
      modal.className = 'cmd-palette-backdrop';
      modal.innerHTML = `
        <div class="cmd-palette-modal" style="max-width:540px;">
          <div class="cmd-palette-header" style="justify-content:space-between;">
            <div style="display:flex; align-items:center; gap:0.5rem; font-weight:700; color:var(--text-primary);">
              <span>⌨️ Keyboard Shortcuts Cheatsheet</span>
            </div>
            <button type="button" class="player-btn btn-close-shortcuts" style="font-size:1.2rem;">&times;</button>
          </div>
          <div style="padding:1.25rem; font-size:0.875rem;">
            <div style="display:grid; grid-template-columns:auto 1fr; gap:0.75rem 1.25rem; align-items:center;">
              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">Ctrl+K</kbd>
              <span>Open universal Command Palette</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">j / ↓</kbd>
              <span>Move down to next search result</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">k / ↑</kbd>
              <span>Move up to previous search result</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">Enter</kbd>
              <span>Open selected search result</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">r</kbd>
              <span>Open selected result in Clean Reader View</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">b</kbd>
              <span>Toggle bookmark on selected result</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">c / y</kbd>
              <span>Copy selected result link to clipboard</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">/ or s</kbd>
              <span>Focus search bar</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">?</kbd>
              <span>Show this shortcuts guide</span>

              <kbd style="font-family:monospace; padding:3px 8px; background:rgba(255,255,255,0.1); border-radius:4px;">Esc</kbd>
              <span>Close any open modal</span>
            </div>
          </div>
        </div>
      `;
      document.body.appendChild(modal);
      modal.querySelector('.btn-close-shortcuts').addEventListener('click', () => modal.classList.remove('active'));
      modal.addEventListener('click', (e) => {
        if (e.target === modal) modal.classList.remove('active');
      });
    }
    modal.classList.add('active');
  }

  // --- Enhanced Vim Navigation Shortcuts ---
  function initVimKeybindings() {
    let currentIndex = -1;
    const input = document.querySelector('.search-input');

    function getItems() {
      return document.querySelectorAll('.result-item, .video-card, .image-card');
    }

    document.addEventListener('keydown', function (e) {
      if (document.activeElement === input || document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA' || document.activeElement.isContentEditable) {
        return;
      }
      if (e.ctrlKey || e.metaKey || e.altKey) return;

      const items = getItems();

      if (e.key === '/' || e.key === 's') {
        e.preventDefault();
        if (input) {
          input.focus();
          input.select();
        }
      } else if (e.key === 'j' || e.key === 'ArrowDown') {
        if (items.length === 0) return;
        e.preventDefault();
        currentIndex = Math.min(currentIndex + 1, items.length - 1);
        highlightResult(currentIndex, items);
      } else if (e.key === 'k' || e.key === 'ArrowUp') {
        if (items.length === 0) return;
        e.preventDefault();
        currentIndex = Math.max(currentIndex - 1, 0);
        highlightResult(currentIndex, items);
      } else if (e.key === 'Enter') {
        if (currentIndex >= 0 && currentIndex < items.length) {
          const link = items[currentIndex].querySelector('.result-title a, .video-title a, a.image-thumb-wrap');
          if (link) link.click();
        }
      } else if (e.key === 'r') {
        if (currentIndex >= 0 && currentIndex < items.length) {
          const link = items[currentIndex].querySelector('.result-title a');
          if (link && link.href) {
            window.open('/reader?url=' + encodeURIComponent(link.href), '_blank');
            showToast('Opening in Clean Reader Mode...', '📖');
          }
        }
      } else if (e.key === 'b') {
        if (currentIndex >= 0 && currentIndex < items.length) {
          const bBtn = items[currentIndex].querySelector('.btn-bookmark');
          if (bBtn) {
            bBtn.click();
            showToast('Bookmark toggled', '🔖');
          }
        }
      } else if (e.key === 'y' || e.key === 'c') {
        if (currentIndex >= 0 && currentIndex < items.length) {
          const link = items[currentIndex].querySelector('.result-title a, .video-title a, a.image-thumb-wrap');
          if (link && link.href) {
            navigator.clipboard.writeText(link.href).then(() => {
              showToast('Link copied to clipboard!', '📋');
            }).catch(() => {
              showToast('Copied: ' + link.href, '📋');
            });
          }
        }
      } else if (e.key === '?') {
        e.preventDefault();
        showShortcutsModal();
      }
    });

    function highlightResult(index, items) {
      items.forEach((it, idx) => {
        if (idx === index) {
          it.classList.add('keyboard-selected');
          it.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        } else {
          it.classList.remove('keyboard-selected');
        }
      });
    }
  }

  // --- Universal Command Palette (Ctrl+K / Cmd+K) ---
  function initCommandPalette() {
    let backdrop = document.getElementById('searxgo-cmd-palette');
    if (!backdrop) {
      backdrop = document.createElement('div');
      backdrop.id = 'searxgo-cmd-palette';
      backdrop.className = 'cmd-palette-backdrop';
      backdrop.innerHTML = `
        <div class="cmd-palette-modal" role="dialog" aria-modal="true">
          <div class="cmd-palette-header">
            <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2" fill="none">
              <circle cx="11" cy="11" r="8"></circle>
              <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
            </svg>
            <input type="text" class="cmd-palette-input" placeholder="Type a command, search engine, theme, or shortcut..." autocomplete="off" spellcheck="false" />
          </div>
          <ul class="cmd-palette-results"></ul>
          <div class="cmd-palette-footer">
            <div class="cmd-palette-footer-keys">
              <span><kbd>↑</kbd> <kbd>↓</kbd> to navigate</span>
              <span><kbd>↵</kbd> to select</span>
              <span><kbd>esc</kbd> to close</span>
            </div>
            <span>SearXGo Command Center</span>
          </div>
        </div>
      `;
      document.body.appendChild(backdrop);
    }

    const input = backdrop.querySelector('.cmd-palette-input');
    const list = backdrop.querySelector('.cmd-palette-results');
    let selectedIdx = 0;

    const commands = [
      // Navigation
      { category: 'Navigation', icon: '🪐', title: 'Home Page', action: () => window.location.href = '/' },
      { category: 'Navigation', icon: '🔍', title: 'Domain Recon & Security Auditor', action: () => window.location.href = '/recon' },
      { category: 'Navigation', icon: '🎯', title: 'Google Dorking Recon Suite', action: () => window.location.href = '/dorks' },
      { category: 'Navigation', icon: '🛡️', title: 'EXIF Metadata Cleaner & Privacy Scrubber', action: () => window.location.href = '/scrub' },
      { category: 'Navigation', icon: '🕵️', title: 'OSINT Sherlock Username Recon', action: () => window.location.href = '/sherlock' },
      { category: 'Navigation', icon: '⚠️', title: 'URL Threat Intelligence & Phishing Scanner', action: () => window.location.href = '/threat' },
      { category: 'Navigation', icon: '🕸️', title: 'Interactive Knowledge & Entity Graph', action: () => window.location.href = '/graph' },
      { category: 'Navigation', icon: '⚖️', title: 'Split-Screen Multi Engine Comparison', action: () => window.location.href = '/split' },
      { category: 'Navigation', icon: '⚙️', title: 'Preferences & Engine Settings', action: () => window.location.href = '/settings' },
      { category: 'Navigation', icon: '📊', title: 'Engine Telemetry & Diagnostics', action: () => window.location.href = '/stats' },

      // Quick Bangs
      { category: 'Engines & Bangs', icon: '🟢', title: 'DuckDuckGo Bang (!ddg)', shortcut: '!ddg', action: () => appendBang('!ddg ') },
      { category: 'Engines & Bangs', icon: '🔵', title: 'Google Bang (!g)', shortcut: '!g', action: () => appendBang('!g ') },
      { category: 'Engines & Bangs', icon: '🔴', title: 'YouTube Video Search (!yt)', shortcut: '!yt', action: () => appendBang('!yt ') },
      { category: 'Engines & Bangs', icon: '🟣', title: 'Wikipedia Articles (!w)', shortcut: '!w', action: () => appendBang('!w ') },
      { category: 'Engines & Bangs', icon: '⬛', title: 'GitHub Code & Repos (!gh)', shortcut: '!gh', action: () => appendBang('!gh ') },
      { category: 'Engines & Bangs', icon: '🧅', title: 'Tor Onion Darkweb Search (!ahmia)', shortcut: '!tor', action: () => appendBang('!ahmia ') },
      { category: 'Engines & Bangs', icon: '💣', title: 'Exploit-DB & 0-Day Vulnerabilities (!exploit)', shortcut: '!exploit', action: () => appendBang('!exploit ') },
      { category: 'Engines & Bangs', icon: '🔓', title: 'Data Breach & Leaks OSINT (!leak)', shortcut: '!leak', action: () => appendBang('!leak ') },

      // Themes
      { category: 'Appearance', icon: '🌙', title: 'Theme: Dark Glass (Default)', action: () => { window.setTheme('dark'); showToast('Switched to Dark Glass theme'); } },
      { category: 'Appearance', icon: '☀️', title: 'Theme: Light Minimal', action: () => { window.setTheme('light'); showToast('Switched to Light Minimal theme'); } },
      { category: 'Appearance', icon: '🖤', title: 'Theme: OLED Pitch Black', action: () => { window.setTheme('black'); showToast('Switched to OLED Black theme'); } },
      { category: 'Appearance', icon: '🧛', title: 'Theme: Dracula Theme', action: () => { window.setTheme('dracula'); showToast('Switched to Dracula theme'); } },
      { category: 'Appearance', icon: '⚡', title: 'Theme: Cyberpunk Neon', action: () => { window.setTheme('cyberpunk'); showToast('Switched to Cyberpunk Neon theme'); } },

      // Tools & Utilities
      { category: 'Actions', icon: '🔖', title: 'Open Saved Bookmarks Workspace', action: () => {
        const bmBtn = document.querySelector('.btn-open-bookmarks');
        if (bmBtn) bmBtn.click();
      }},
      { category: 'Actions', icon: '🔐', title: 'Export Encrypted Vault Backup', action: () => exportEncryptedVault() },
      { category: 'Actions', icon: '🧹', title: 'Clear All Cookies & Preferences', action: () => window.location.href = '/clear_cookies' },
      { category: 'Actions', icon: '❓', title: 'View Keyboard Navigation Cheatsheet', action: () => showShortcutsModal() },
    ];

    function appendBang(bang) {
      const searchInp = document.querySelector('.search-input');
      if (searchInp) {
        searchInp.value = bang + searchInp.value.replace(/^!\w+\s*/, '');
        searchInp.focus();
        searchInp.setSelectionRange(searchInp.value.length, searchInp.value.length);
      } else {
        window.location.href = '/search?q=' + encodeURIComponent(bang);
      }
    }

    function openPalette() {
      backdrop.classList.add('active');
      input.value = '';
      selectedIdx = 0;
      renderResults('');
      setTimeout(() => input.focus(), 50);
    }

    function closePalette() {
      backdrop.classList.remove('active');
    }

    function renderResults(filterText) {
      const query = filterText.toLowerCase().trim();
      const filtered = commands.filter(c => 
        c.title.toLowerCase().includes(query) || 
        c.category.toLowerCase().includes(query) || 
        (c.shortcut && c.shortcut.toLowerCase().includes(query))
      );

      if (filtered.length === 0) {
        list.innerHTML = `<li style="padding:1.5rem; text-align:center; color:var(--text-muted); font-size:0.9rem;">No matching commands found</li>`;
        return;
      }

      let html = '';
      let currentCat = '';
      filtered.forEach((cmd, idx) => {
        if (cmd.category !== currentCat) {
          currentCat = cmd.category;
          html += `<div class="cmd-palette-group-title">${currentCat}</div>`;
        }
        const isSel = idx === selectedIdx;
        html += `
          <li class="cmd-palette-item ${isSel ? 'active' : ''}" data-idx="${idx}">
            <div class="cmd-palette-item-left">
              <span class="cmd-palette-item-icon">${cmd.icon}</span>
              <span class="cmd-palette-item-title">${cmd.title}</span>
            </div>
            ${cmd.shortcut ? `<span class="cmd-palette-item-shortcut">${cmd.shortcut}</span>` : ''}
          </li>
        `;
      });
      list.innerHTML = html;

      list.querySelectorAll('.cmd-palette-item').forEach(item => {
        item.addEventListener('click', () => {
          const idx = parseInt(item.dataset.idx, 10);
          closePalette();
          if (filtered[idx] && filtered[idx].action) filtered[idx].action();
        });
      });
    }

    input.addEventListener('input', () => {
      selectedIdx = 0;
      renderResults(input.value);
    });

    input.addEventListener('keydown', (e) => {
      const items = list.querySelectorAll('.cmd-palette-item');
      if (items.length === 0) return;

      if (e.key === 'ArrowDown') {
        e.preventDefault();
        selectedIdx = (selectedIdx + 1) % items.length;
        updateSelection(items);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        selectedIdx = (selectedIdx - 1 + items.length) % items.length;
        updateSelection(items);
      } else if (e.key === 'Enter') {
        e.preventDefault();
        if (items[selectedIdx]) items[selectedIdx].click();
      } else if (e.key === 'Escape') {
        closePalette();
      }
    });

    function updateSelection(items) {
      items.forEach((it, idx) => {
        if (idx === selectedIdx) {
          it.classList.add('active');
          it.scrollIntoView({ block: 'nearest' });
        } else {
          it.classList.remove('active');
        }
      });
    }

    document.addEventListener('keydown', (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        if (backdrop.classList.contains('active')) {
          closePalette();
        } else {
          openPalette();
        }
      } else if (e.key === 'Escape' && backdrop.classList.contains('active')) {
        closePalette();
      }
    });

    backdrop.addEventListener('click', (e) => {
      if (e.target === backdrop) closePalette();
    });

    document.addEventListener('click', (e) => {
      if (e.target.closest('.btn-open-cmd-palette')) {
        e.preventDefault();
        openPalette();
      }
    });
  }

  // --- Custom Bangs & Custom Engines Builder ---
  function initCustomBangs() {
    function getCustomBangs() {
      try {
        return JSON.parse(localStorage.getItem('searxgo_custom_bangs') || '[]');
      } catch (e) {
        return [];
      }
    }

    function saveCustomBangs(bangs) {
      localStorage.setItem('searxgo_custom_bangs', JSON.stringify(bangs));
      renderSettingsCustomBangs();
    }

    function renderSettingsCustomBangs() {
      const listEl = document.getElementById('custom-bangs-list');
      const emptyEl = document.getElementById('custom-bangs-empty');
      if (!listEl || !emptyEl) return;

      const bangs = getCustomBangs();
      if (bangs.length === 0) {
        listEl.innerHTML = '';
        emptyEl.style.display = 'block';
        return;
      }

      emptyEl.style.display = 'none';
      listEl.innerHTML = bangs.map((b, idx) => `
        <tr style="border-bottom:1px solid var(--border-glass);">
          <td style="padding:8px 10px; font-weight:600; color:var(--text-primary);">${b.name}</td>
          <td style="padding:8px 10px;"><code style="color:var(--accent-primary); font-weight:700;">${b.prefix}</code></td>
          <td style="padding:8px 10px; color:var(--text-muted); font-family:monospace; font-size:0.8rem;">${b.url}</td>
          <td style="padding:8px 10px; text-align:right;">
            <button type="button" class="nav-btn btn-del-custom-bang" data-idx="${idx}" style="padding:2px 8px; font-size:0.75rem; color:#ef4444; border-color:rgba(239,68,68,0.3);">
              Delete
            </button>
          </td>
        </tr>
      `).join('');

      listEl.querySelectorAll('.btn-del-custom-bang').forEach(btn => {
        btn.addEventListener('click', () => {
          const idx = parseInt(btn.dataset.idx, 10);
          const current = getCustomBangs();
          current.splice(idx, 1);
          saveCustomBangs(current);
          showToast('Custom bang deleted');
        });
      });
    }

    const addBtn = document.getElementById('btn-add-custom-bang');
    if (addBtn) {
      addBtn.addEventListener('click', () => {
        const nameInp = document.getElementById('custom-bang-name');
        const prefixInp = document.getElementById('custom-bang-prefix');
        const urlInp = document.getElementById('custom-bang-url');

        const name = (nameInp.value || '').trim();
        let prefix = (prefixInp.value || '').trim();
        const targetUrl = (urlInp.value || '').trim();

        if (!name || !prefix || !targetUrl) {
          alert('Please fill out Name, Bang Prefix, and Search URL');
          return;
        }
        if (!prefix.startsWith('!')) prefix = '!' + prefix;
        if (!targetUrl.includes('%s')) {
          alert('Search URL must contain %s placeholder for the query term (e.g. https://site.com/search?q=%s)');
          return;
        }

        const bangs = getCustomBangs();
        bangs.push({ name, prefix, url: targetUrl });
        saveCustomBangs(bangs);

        nameInp.value = '';
        prefixInp.value = '';
        urlInp.value = '';
        showToast(`Custom bang ${prefix} added!`, '⚡');
      });
      renderSettingsCustomBangs();
    }

    // Intercept search submit
    document.addEventListener('submit', function (e) {
      const form = e.target.closest('.search-form');
      if (!form) return;
      const input = form.querySelector('.search-input');
      if (!input) return;

      const val = input.value.trim();
      const parts = val.split(/\s+/);
      const firstWord = parts[0];

      const bangs = getCustomBangs();
      const match = bangs.find(b => b.prefix.toLowerCase() === firstWord.toLowerCase());
      if (match) {
        e.preventDefault();
        const queryTerm = parts.slice(1).join(' ');
        const dest = match.url.replace('%s', encodeURIComponent(queryTerm));
        window.location.href = dest;
      }
    });
  }

  // --- Encrypted Vault Backup & Restore (PBKDF2 + AES-GCM) ---
  async function exportEncryptedVault() {
    const password = prompt('Enter a password to encrypt your vault backup:');
    if (!password) return;

    const dataObj = {
      version: 1,
      timestamp: new Date().toISOString(),
      bookmarks: JSON.parse(localStorage.getItem('searxgo_bookmarks') || '[]'),
      saved_queries: JSON.parse(localStorage.getItem('searxgo_saved_queries') || '[]'),
      custom_bangs: JSON.parse(localStorage.getItem('searxgo_custom_bangs') || '[]'),
      theme: localStorage.getItem('searxgo_theme') || 'dark',
    };

    try {
      const enc = new TextEncoder();
      const salt = window.crypto.getRandomValues(new Uint8Array(16));
      const iv = window.crypto.getRandomValues(new Uint8Array(12));

      const keyMaterial = await window.crypto.subtle.importKey(
        'raw', enc.encode(password), 'PBKDF2', false, ['deriveKey']
      );
      const key = await window.crypto.subtle.deriveKey(
        { name: 'PBKDF2', salt: salt, iterations: 100000, hash: 'SHA-256' },
        keyMaterial,
        { name: 'AES-GCM', length: 256 },
        false,
        ['encrypt']
      );
      const ciphertext = await window.crypto.subtle.encrypt(
        { name: 'AES-GCM', iv: iv },
        key,
        enc.encode(JSON.stringify(dataObj))
      );

      const vaultPayload = {
        searxgo_vault: true,
        version: 1,
        salt: Array.from(salt),
        iv: Array.from(iv),
        data: Array.from(new Uint8Array(ciphertext))
      };

      const blob = new Blob([JSON.stringify(vaultPayload, null, 2)], { type: 'application/json' });
      const dlLink = document.createElement('a');
      dlLink.href = URL.createObjectURL(blob);
      dlLink.download = `searxgo-vault-${new Date().toISOString().slice(0, 10)}.json`;
      dlLink.click();
      showToast('Encrypted vault exported safely!', '🔐');
    } catch (err) {
      alert('Encryption failed: ' + err.message);
    }
  }

  function initEncryptedVault() {
    const exportBtn = document.getElementById('btn-export-vault');
    const importInput = document.getElementById('file-import-vault');

    if (exportBtn) {
      exportBtn.addEventListener('click', exportEncryptedVault);
    }

    if (importInput) {
      importInput.addEventListener('change', async (e) => {
        const file = e.target.files[0];
        if (!file) return;

        try {
          const text = await file.text();
          const vaultPayload = JSON.parse(text);

          if (!vaultPayload.searxgo_vault || !vaultPayload.salt || !vaultPayload.iv || !vaultPayload.data) {
            alert('Invalid SearXGo vault file format');
            return;
          }

          const password = prompt('Enter your vault password to decrypt:');
          if (!password) return;

          const enc = new TextEncoder();
          const salt = new Uint8Array(vaultPayload.salt);
          const iv = new Uint8Array(vaultPayload.iv);
          const ciphertext = new Uint8Array(vaultPayload.data);

          const keyMaterial = await window.crypto.subtle.importKey(
            'raw', enc.encode(password), 'PBKDF2', false, ['deriveKey']
          );
          const key = await window.crypto.subtle.deriveKey(
            { name: 'PBKDF2', salt: salt, iterations: 100000, hash: 'SHA-256' },
            keyMaterial,
            { name: 'AES-GCM', length: 256 },
            false,
            ['decrypt']
          );
          const decrypted = await window.crypto.subtle.decrypt(
            { name: 'AES-GCM', iv: iv },
            key,
            ciphertext
          );

          const dataObj = JSON.parse(new TextDecoder().decode(decrypted));

          if (dataObj.bookmarks) localStorage.setItem('searxgo_bookmarks', JSON.stringify(dataObj.bookmarks));
          if (dataObj.saved_queries) localStorage.setItem('searxgo_saved_queries', JSON.stringify(dataObj.saved_queries));
          if (dataObj.custom_bangs) localStorage.setItem('searxgo_custom_bangs', JSON.stringify(dataObj.custom_bangs));
          if (dataObj.theme) window.setTheme(dataObj.theme);

          showToast('Vault decrypted & restored successfully!', '🔓');
          setTimeout(() => window.location.reload(), 1200);
        } catch (err) {
          alert('Decryption failed: Incorrect password or corrupted vault file.');
        }
      });
    }
  }

  // --- Dynamic Infinite Scroll & Unlimited Search ---
  function initInfiniteScroll() {
    const resultsList = document.querySelector('.results-list');
    const imageGrid = document.querySelector('.image-grid');
    const videoGrid = document.querySelector('.video-grid');
    const pagination = document.querySelector('.pagination');
    const loadMoreBtn = document.getElementById('btn-load-more');

    const container = resultsList || imageGrid || videoGrid;
    if (!container) return;

    const urlParams = new URLSearchParams(window.location.search);
    const query = urlParams.get('q');
    if (!query) return;

    const isAutoInfinite = getCookie('searxgo_infinite_scroll') === 'true' || urlParams.get('infinite_scroll') === '1' || urlParams.get('unlimited') === '1';

    let currentPage = parseInt(urlParams.get('page') || '1', 10);
    const category = urlParams.get('category') || 'general';
    const timeRange = urlParams.get('time_range') || '';
    let isFetching = false;
    let hasMore = true;

    // Create spinner element
    const spinner = document.createElement('div');
    spinner.className = 'infinite-loading';
    spinner.style.display = 'none';
    spinner.innerHTML = '<div class="infinite-spinner"></div><span>Streaming more results from engines...</span>';
    container.parentNode.appendChild(spinner);

    if (isAutoInfinite && pagination) {
      pagination.style.display = 'none'; // Hide static pagination when auto infinite scroll is active
    }

    function checkScroll() {
      if (!isAutoInfinite || isFetching || !hasMore) return;
      const scrollPos = window.innerHeight + window.scrollY;
      const threshold = document.body.offsetHeight - 650;

      if (scrollPos >= threshold) {
        fetchNextPage();
      }
    }

    function fetchNextPage() {
      if (isFetching || !hasMore) return;
      isFetching = true;
      spinner.style.display = 'flex';
      if (loadMoreBtn) {
        loadMoreBtn.disabled = true;
        loadMoreBtn.innerHTML = '<span>⏳ Fetching Page ' + (currentPage + 1) + '...</span>';
      }

      const nextPage = currentPage + 1;
      const fetchUrl = `/search?q=${encodeURIComponent(query)}&category=${encodeURIComponent(category)}&time_range=${encodeURIComponent(timeRange)}&page=${nextPage}&format=json`;

      fetch(fetchUrl)
        .then(res => {
          if (!res.ok) throw new Error('Network response not ok');
          return res.json();
        })
        .then(data => {
          spinner.style.display = 'none';
          isFetching = false;

          if (!data || !data.results || data.results.length === 0) {
            hasMore = false;
            if (loadMoreBtn) {
              loadMoreBtn.disabled = true;
              loadMoreBtn.innerHTML = '<span>✓ All results loaded</span>';
            }
            return;
          }

          currentPage = nextPage;
          if (loadMoreBtn) {
            loadMoreBtn.disabled = false;
            loadMoreBtn.innerHTML = '<span>⚡ Load More Results (Page ' + (currentPage + 1) + ')</span>';
          }

          renderAppendResults(data.results, data.category);
        })
        .catch(err => {
          spinner.style.display = 'none';
          isFetching = false;
          if (loadMoreBtn) {
            loadMoreBtn.disabled = false;
            loadMoreBtn.innerHTML = '<span>⚠️ Retry Loading Page ' + (currentPage + 1) + '</span>';
          }
        });
    }

    if (loadMoreBtn) {
      loadMoreBtn.addEventListener('click', function () {
        fetchNextPage();
      });
    }

    if (isAutoInfinite) {
      window.addEventListener('scroll', checkScroll, { passive: true });
    }

    function renderAppendResults(results, cat) {
      if (cat === 'images' && imageGrid) {
        results.forEach(it => {
          const card = document.createElement('div');
          card.className = 'image-card';
          card.innerHTML = `
            <a href="${escapeHtml(it.url)}" target="_blank" rel="noreferrer noopener" class="image-thumb-wrap">
              <img src="/proxy/image?url=${encodeURIComponent(it.thumbnail || it.url)}" alt="${escapeHtml(it.title)}" class="image-thumb" loading="lazy" onerror="this.onerror=null; this.src='${escapeHtml(it.thumbnail || it.url)}';" />
            </a>
            <div class="image-info">
              <div class="image-title" title="${escapeHtml(it.title)}">${escapeHtml(it.title)}</div>
              <div class="image-source">${escapeHtml(it.pretty_url || '')}</div>
            </div>
          `;
          imageGrid.appendChild(card);
        });
      } else if (cat === 'videos' && videoGrid) {
        results.forEach(it => {
          const card = document.createElement('div');
          card.className = 'video-card';
          card.innerHTML = `
            <div class="video-thumb-wrap" data-video-url="${escapeHtml(it.video_url || '')}" data-video-title="${escapeHtml(it.title)}">
              <img src="/proxy/image?url=${encodeURIComponent(it.thumbnail || '')}" alt="${escapeHtml(it.title)}" class="video-thumb" loading="lazy" onerror="this.onerror=null; this.src='${escapeHtml(it.thumbnail || '')}';" />
              ${it.duration ? `<span class="video-duration">${escapeHtml(it.duration)}</span>` : ''}
              <div class="video-play-overlay">▶</div>
            </div>
            <div class="video-info">
              <h3 class="video-title">
                <a href="${escapeHtml(it.url)}" target="_blank" rel="noreferrer noopener">${escapeHtml(it.title)}</a>
              </h3>
              <div class="video-channel">${escapeHtml(it.author || '')} &bull; ${escapeHtml(it.pretty_url || '')}</div>
            </div>
          `;
          videoGrid.appendChild(card);
        });
        initVideoModal();
      } else if (resultsList) {
        results.forEach(it => {
          const domain = (it.url || '').replace(/^https?:\/\//, '').split('/')[0].replace(/^www\./, '');
          const article = document.createElement('article');
          article.className = 'result-item';
          
          let enginesBadges = '';
          if (Array.isArray(it.engines)) {
            it.engines.forEach(eng => {
              enginesBadges += `<span class="engine-badge">${escapeHtml(eng)}</span>`;
            });
          }

          let extraPills = '';
          if (it.file_size) extraPills += `<span class="extra-pill">📦 ${escapeHtml(it.file_size)}</span>`;
          if (it.seeders && it.seeders > 0) extraPills += `<span class="extra-pill" style="color:var(--accent-emerald);">▲ ${it.seeders} seeds</span>`;

          article.innerHTML = `
            <div class="result-url-wrap">
              <img src="/proxy/image?url=https://icons.duckduckgo.com/ip2/${domain}.ico" class="site-favicon" onerror="this.style.display='none'" alt="" />
              <span class="result-url">${escapeHtml(it.pretty_url || it.url)}</span>
              ${it.cached_url ? `<a href="${escapeHtml(it.cached_url)}" target="_blank" rel="noreferrer noopener" class="cached-link" title="View cached snapshot">[Cached]</a>` : ''}
              ${it.magnet_url ? `<button type="button" class="btn-magnet" data-magnet="${escapeHtml(it.magnet_url)}" title="Copy Magnet Link">🧲 Copy Magnet</button>` : ''}
            </div>
            <h2 class="result-title">
              <a href="${escapeHtml(it.url)}" target="_blank" rel="noreferrer noopener">${escapeHtml(it.title)}</a>
            </h2>
            <div class="result-snippet">${escapeHtml(it.content || '')}</div>
            <div class="result-footer">
              <div class="result-badges">
                ${enginesBadges}
                ${extraPills}
              </div>
              ${it.author ? `<span class="result-author">By ${escapeHtml(it.author)}</span>` : ''}
            </div>
          `;
          resultsList.appendChild(article);
        });
        initMagnetButtons();
      }
    }
  }

  // --- Settings Page Tabs & Operations ---
  function initSettingsPage() {
    const form = document.querySelector('#settings-form');
    if (!form) return;

    // Main Tabs switching
    const tabBtns = document.querySelectorAll('.settings-tab-btn');
    const panels = document.querySelectorAll('.settings-tab-panel');

    tabBtns.forEach(btn => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        tabBtns.forEach(b => b.classList.remove('active'));
        panels.forEach(p => {
          p.classList.remove('active');
          p.style.display = 'none';
        });

        this.classList.add('active');
        const target = document.getElementById(this.dataset.tab);
        if (target) {
          target.classList.add('active');
          target.style.display = 'block';
        }
      });
    });

    // Engine Sub-Category Tabs switching
    const catTabBtns = document.querySelectorAll('.engine-cat-tab-btn');
    const catPanels = document.querySelectorAll('.engine-cat-panel');

    catTabBtns.forEach(btn => {
      btn.addEventListener('click', function () {
        catTabBtns.forEach(b => b.classList.remove('active'));
        catPanels.forEach(p => {
          p.classList.remove('active');
          p.style.display = 'none';
        });

        this.classList.add('active');
        const target = document.getElementById(this.dataset.cat);
        if (target) {
          target.classList.add('active');
          target.style.display = 'block';
        }
      });
    });

    // Category-specific Enable All / Disable All
    document.querySelectorAll('.btn-enable-all-cat').forEach(btn => {
      btn.addEventListener('click', function () {
        const catId = this.dataset.cat;
        const panel = document.getElementById(catId);
        if (panel) {
          panel.querySelectorAll('input[type="checkbox"]').forEach(cb => {
            if (!cb.disabled) cb.checked = true;
          });
        }
      });
    });

    document.querySelectorAll('.btn-disable-all-cat').forEach(btn => {
      btn.addEventListener('click', function () {
        const catId = this.dataset.cat;
        const panel = document.getElementById(catId);
        if (panel) {
          panel.querySelectorAll('input[type="checkbox"]').forEach(cb => {
            if (!cb.disabled) cb.checked = false;
          });
        }
      });
    });

    // Load saved settings
    const savedEngines = getCookie('searxgo_engines');
    if (savedEngines) {
      const activeList = savedEngines.split(',');
      form.querySelectorAll('input[name^="engine_"]').forEach(cb => {
        cb.checked = activeList.includes(cb.value);
      });
    }

    const themeSelect = form.querySelector('#theme-select');
    if (themeSelect) {
      themeSelect.value = localStorage.getItem('searxgo_theme') || getCookie('searxgo_theme') || 'dark';
      themeSelect.addEventListener('change', function () {
        window.setTheme(this.value);
      });
    }

    const safeSearchSelect = form.querySelector('#safesearch-select');
    if (safeSearchSelect) {
      const savedSafe = getCookie('searxgo_safesearch');
      if (savedSafe !== null) safeSearchSelect.value = savedSafe;
    }

    const langSelect = form.querySelector('#language-select');
    if (langSelect) {
      const savedLang = getCookie('searxgo_language');
      if (savedLang) langSelect.value = savedLang;
    }

    const infiniteToggle = document.getElementById('infinite-scroll-toggle');
    if (infiniteToggle) {
      infiniteToggle.checked = getCookie('searxgo_infinite_scroll') === 'true';
    }

    const newtabToggle = document.getElementById('newtab-toggle');
    if (newtabToggle) {
      const savedNewtab = getCookie('searxgo_newtab');
      if (savedNewtab !== null) newtabToggle.checked = (savedNewtab === 'true');
    }

    const methodSelect = document.getElementById('http-method-select');
    if (methodSelect) {
      const savedMethod = getCookie('searxgo_method');
      if (savedMethod) methodSelect.value = savedMethod;
    }

    const acSelect = document.getElementById('autocomplete-provider');
    if (acSelect) {
      const savedAC = getCookie('searxgo_autocomplete');
      if (savedAC) acSelect.value = savedAC;
    }

    const trackerToggle = document.getElementById('tracker-stripper-toggle');
    if (trackerToggle) {
      const savedTracker = getCookie('searxgo_tracker_remover');
      if (savedTracker !== null) trackerToggle.checked = (savedTracker === 'true');
    }

    const doiSelect = document.getElementById('doi-resolver-select');
    if (doiSelect) {
      const savedDOI = getCookie('searxgo_doi_resolver');
      if (savedDOI) doiSelect.value = savedDOI;
    }

    const redToggle = document.getElementById('redirects-toggle');
    if (redToggle) {
      const savedRed = getCookie('searxgo_redirects');
      if (savedRed !== null) redToggle.checked = (savedRed === 'true');
    }

    // JSON Export
    const btnExport = document.getElementById('btn-export-json');
    if (btnExport) {
      btnExport.addEventListener('click', function () {
        const engines = [];
        form.querySelectorAll('input[name^="engine_"]:checked').forEach(cb => engines.push(cb.value));
        const data = {
          theme: themeSelect ? themeSelect.value : 'dark',
          safesearch: safeSearchSelect ? safeSearchSelect.value : '0',
          language: langSelect ? langSelect.value : '',
          infinite_scroll: infiniteToggle ? infiniteToggle.checked : false,
          new_tab: newtabToggle ? newtabToggle.checked : true,
          method: methodSelect ? methodSelect.value : 'GET',
          engines: engines,
          redirects: document.getElementById('redirects-toggle')?.checked ?? true,
          proxy: document.getElementById('proxy-toggle')?.checked ?? true,
        };
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'searxgo_preferences.json';
        a.click();
      });
    }

    // JSON Import
    const fileImport = document.getElementById('file-import-json');
    if (fileImport) {
      fileImport.addEventListener('change', function (e) {
        const file = e.target.files[0];
        if (!file) return;
        const reader = new FileReader();
        reader.onload = function (evt) {
          try {
            const conf = JSON.parse(evt.target.result);
            if (conf.theme && themeSelect) {
              themeSelect.value = conf.theme;
              window.setTheme(conf.theme);
            }
            if (conf.safesearch && safeSearchSelect) safeSearchSelect.value = conf.safesearch;
            if (conf.language && langSelect) langSelect.value = conf.language;
            if (conf.infinite_scroll !== undefined && infiniteToggle) infiniteToggle.checked = conf.infinite_scroll;
            if (conf.new_tab !== undefined && newtabToggle) newtabToggle.checked = conf.new_tab;
            if (conf.method && methodSelect) methodSelect.value = conf.method;
            if (Array.isArray(conf.engines)) {
              form.querySelectorAll('input[name^="engine_"]').forEach(cb => {
                cb.checked = conf.engines.includes(cb.value);
              });
            }
            alert('Preferences successfully loaded from file! Click "Save All Preferences" to persist.');
          } catch (err) {
            alert('Invalid JSON configuration file: ' + err.message);
          }
        };
        reader.readAsText(file);
      });
    }

    // Reset Defaults
    const btnReset = document.getElementById('btn-reset-defaults');
    if (btnReset) {
      btnReset.addEventListener('click', function () {
        if (confirm('Reset all search preferences and engines to default settings?')) {
          setCookie('searxgo_engines', '', -1);
          setCookie('searxgo_safesearch', '', -1);
          setCookie('searxgo_language', '', -1);
          setCookie('searxgo_infinite_scroll', '', -1);
          setCookie('searxgo_newtab', '', -1);
          setCookie('searxgo_method', '', -1);
          localStorage.removeItem('searxgo_theme');
          window.location.reload();
        }
      });
    }

    // Form Save
    form.addEventListener('submit', function (e) {
      e.preventDefault();

      const checkedEngines = [];
      form.querySelectorAll('input[name^="engine_"]:checked').forEach(cb => {
        checkedEngines.push(cb.value);
      });
      setCookie('searxgo_engines', checkedEngines.join(','), 365);

      if (safeSearchSelect) setCookie('searxgo_safesearch', safeSearchSelect.value, 365);
      if (langSelect) setCookie('searxgo_language', langSelect.value, 365);
      if (themeSelect) window.setTheme(themeSelect.value);

      if (infiniteToggle) setCookie('searxgo_infinite_scroll', infiniteToggle.checked ? 'true' : 'false', 365);
      if (newtabToggle) setCookie('searxgo_newtab', newtabToggle.checked ? 'true' : 'false', 365);
      if (methodSelect) setCookie('searxgo_method', methodSelect.value, 365);

      const redirectsToggle = document.getElementById('redirects-toggle');
      if (redirectsToggle) {
        setCookie('searxgo_redirects', redirectsToggle.checked ? 'true' : 'false', 365);
      }

      const acSelectEl = document.getElementById('autocomplete-provider');
      if (acSelectEl) {
        setCookie('searxgo_autocomplete', acSelectEl.value, 365);
      }

      const trackerToggleEl = document.getElementById('tracker-stripper-toggle');
      if (trackerToggleEl) {
        setCookie('searxgo_tracker_remover', trackerToggleEl.checked ? 'true' : 'false', 365);
      }

      const doiSelectEl = document.getElementById('doi-resolver-select');
      if (doiSelectEl) {
        setCookie('searxgo_doi_resolver', doiSelectEl.value, 365);
      }

      const msg = document.getElementById('save-feedback');
      if (msg) {
        msg.style.display = 'block';
        window.scrollTo({ top: 0, behavior: 'smooth' });
        setTimeout(() => { msg.style.display = 'none'; }, 2500);
      }
    });

    // Real-time Bangs & Syntax Table Filter
    window.filterBangsTable = function (query) {
      const q = (query || '').toLowerCase().trim();
      const rows = document.querySelectorAll('.bang-row');
      rows.forEach(row => {
        const text = row.textContent.toLowerCase();
        if (!q || text.includes(q)) {
          row.style.display = '';
        } else {
          row.style.display = 'none';
        }
      });
    };
  }

  // Apply user-selected form method & link target rules
  function initUserPreferencesOnLoad() {
    const savedMethod = getCookie('searxgo_method');
    if (savedMethod === 'POST') {
      document.querySelectorAll('form.search-form').forEach(f => {
        f.method = 'POST';
      });
    }

    const savedNewtab = getCookie('searxgo_newtab');
    if (savedNewtab === 'false') {
      document.querySelectorAll('.result-title a, .image-thumb-wrap, .video-title a').forEach(a => {
        a.removeAttribute('target');
      });
    }
  }

  // --- PWA Service Worker Registration ---
  function initServiceWorker() {
    if ('serviceWorker' in navigator) {
      window.addEventListener('load', () => {
        navigator.serviceWorker.register('/sw.js').catch(() => {});
      });
    }
  }

  // --- Private Bookmarks & Saved Searches ---
  function initBookmarks() {
    const modal = document.getElementById('bookmarks-modal');
    const closeBtn = document.getElementById('btn-close-bookmarks');
    const openBtns = document.querySelectorAll('.btn-open-bookmarks');
    const countResultsEl = document.getElementById('bm-count-results');
    const countQueriesEl = document.getElementById('bm-count-queries');
    const resultsListEl = document.getElementById('bm-results-list');
    const queriesListEl = document.getElementById('bm-queries-list');
    const badgeCounters = document.querySelectorAll('.bookmark-counter');
    const exportBtn = document.getElementById('btn-export-bookmarks');
    const clearBtn = document.getElementById('btn-clear-bookmarks');

    function getBookmarks() {
      try {
        return JSON.parse(localStorage.getItem('searxgo_bookmarks') || '[]');
      } catch (e) {
        return [];
      }
    }

    function saveBookmarks(bms) {
      localStorage.setItem('searxgo_bookmarks', JSON.stringify(bms));
      updateCounters();
      updateResultButtons();
    }

    function getSavedQueries() {
      try {
        return JSON.parse(localStorage.getItem('searxgo_saved_queries') || '[]');
      } catch (e) {
        return [];
      }
    }

    function saveSavedQueries(queries) {
      localStorage.setItem('searxgo_saved_queries', JSON.stringify(queries));
      updateCounters();
    }

    function updateCounters() {
      const bms = getBookmarks();
      const queries = getSavedQueries();
      const total = bms.length + queries.length;

      if (countResultsEl) countResultsEl.textContent = bms.length;
      if (countQueriesEl) countQueriesEl.textContent = queries.length;

      badgeCounters.forEach(el => {
        if (total > 0) {
          el.textContent = total;
          el.style.display = 'inline-block';
        } else {
          el.style.display = 'none';
        }
      });
    }

    function updateResultButtons() {
      const bms = getBookmarks();
      const urls = new Set(bms.map(b => b.url));

      document.querySelectorAll('.btn-bookmark').forEach(btn => {
        const url = btn.dataset.url;
        if (urls.has(url)) {
          btn.classList.add('bookmarked');
          btn.textContent = '⭐ Pinned';
          btn.title = 'Remove pin from bookmarks';
        } else {
          btn.classList.remove('bookmarked');
          btn.textContent = '🔖 Bookmark';
          btn.title = 'Pin / Bookmark this result';
        }
      });
    }

    // Toggle bookmark click on results
    document.querySelectorAll('.btn-bookmark').forEach(btn => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        e.stopPropagation();
        const url = this.dataset.url;
        const title = this.dataset.title || url;
        const engine = this.dataset.engine || '';

        let bms = getBookmarks();
        const exists = bms.findIndex(b => b.url === url);

        if (exists >= 0) {
          bms.splice(exists, 1);
        } else {
          bms.unshift({
            url: url,
            title: title,
            engine: engine,
            timestamp: new Date().toISOString()
          });
        }
        saveBookmarks(bms);
      });
    });

    // Save Query button click
    document.querySelectorAll('.btn-save-search').forEach(btn => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        const q = this.dataset.query;
        const cat = this.dataset.category || 'general';
        if (!q) return;

        let queries = getSavedQueries();
        if (!queries.some(it => it.query === q && it.category === cat)) {
          queries.unshift({
            query: q,
            category: cat,
            timestamp: new Date().toISOString()
          });
          saveSavedQueries(queries);
          btn.textContent = '✓ Query Saved!';
          setTimeout(() => { btn.textContent = '⭐ Save Query'; }, 2000);
        } else {
          btn.textContent = '✓ Already Saved';
          setTimeout(() => { btn.textContent = '⭐ Save Query'; }, 2000);
        }
      });
    });

    // Render lists in modal
    function renderModalLists() {
      const bms = getBookmarks();
      const queries = getSavedQueries();

      if (resultsListEl) {
        if (bms.length === 0) {
          resultsListEl.innerHTML = '<div style="text-align:center; padding:2rem 0; color:var(--text-muted); font-size:0.9rem;">No pinned search results yet. Click 🔖 Bookmark on any result to pin it here!</div>';
        } else {
          resultsListEl.innerHTML = '';
          bms.forEach((b, idx) => {
            const row = document.createElement('div');
            row.className = 'bookmark-row';
            row.style.cssText = 'display:flex; justify-content:space-between; align-items:center; padding:0.6rem 0.5rem; border-bottom:1px solid var(--border-glass); gap:0.75rem;';
            row.innerHTML = `
              <div style="min-width:0; flex:1;">
                <a href="${escapeHtml(b.url)}" target="_blank" rel="noreferrer noopener" style="font-weight:600; color:var(--text-primary); text-decoration:none; display:block; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">
                  ${escapeHtml(b.title)}
                </a>
                <div style="font-size:0.75rem; color:var(--text-muted); white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">
                  ${escapeHtml(b.url)}
                </div>
              </div>
              <button type="button" class="btn-clear btn-delete-bm" data-idx="${idx}" title="Delete bookmark" style="display:inline-flex; color:#ef4444; flex-shrink:0;">✕</button>
            `;
            resultsListEl.appendChild(row);
          });

          resultsListEl.querySelectorAll('.btn-delete-bm').forEach(delBtn => {
            delBtn.addEventListener('click', function () {
              const idx = parseInt(this.dataset.idx, 10);
              let cur = getBookmarks();
              cur.splice(idx, 1);
              saveBookmarks(cur);
              renderModalLists();
            });
          });
        }
      }

      if (queriesListEl) {
        if (queries.length === 0) {
          queriesListEl.innerHTML = '<div style="text-align:center; padding:2rem 0; color:var(--text-muted); font-size:0.9rem;">No saved search queries yet. Click ⭐ Save Query on results to save!</div>';
        } else {
          queriesListEl.innerHTML = '';
          queries.forEach((q, idx) => {
            const row = document.createElement('div');
            row.className = 'bookmark-row';
            row.style.cssText = 'display:flex; justify-content:space-between; align-items:center; padding:0.6rem 0.5rem; border-bottom:1px solid var(--border-glass); gap:0.75rem;';
            row.innerHTML = `
              <div style="min-width:0; flex:1;">
                <a href="/search?q=${encodeURIComponent(q.query)}&category=${encodeURIComponent(q.category)}" style="font-weight:600; color:var(--accent-cyan); text-decoration:none;">
                  🔍 ${escapeHtml(q.query)}
                </a>
                <span class="extra-pill" style="font-size:0.7rem; margin-left:0.5rem;">${escapeHtml(q.category)}</span>
              </div>
              <button type="button" class="btn-clear btn-delete-query" data-idx="${idx}" title="Delete query" style="display:inline-flex; color:#ef4444; flex-shrink:0;">✕</button>
            `;
            queriesListEl.appendChild(row);
          });

          queriesListEl.querySelectorAll('.btn-delete-query').forEach(delBtn => {
            delBtn.addEventListener('click', function () {
              const idx = parseInt(this.dataset.idx, 10);
              let cur = getSavedQueries();
              cur.splice(idx, 1);
              saveSavedQueries(cur);
              renderModalLists();
            });
          });
        }
      }
    }

    // Tab switching inside bookmarks modal
    document.querySelectorAll('.bookmark-tab-btn').forEach(btn => {
      btn.addEventListener('click', function () {
        document.querySelectorAll('.bookmark-tab-btn').forEach(b => {
          b.classList.remove('active');
          b.style.background = 'var(--bg-glass)';
          b.style.color = 'var(--text-secondary)';
        });
        document.querySelectorAll('.bookmark-tab-panel').forEach(p => p.style.display = 'none');

        this.classList.add('active');
        this.style.background = 'var(--accent-primary)';
        this.style.color = 'white';
        const target = document.getElementById(this.dataset.tab);
        if (target) target.style.display = 'block';
      });
    });

    // Open Modal
    openBtns.forEach(btn => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        renderModalLists();
        if (modal) modal.style.display = 'flex';
      });
    });

    // Close Modal
    function closeModal() {
      if (modal) modal.style.display = 'none';
    }
    if (closeBtn) closeBtn.addEventListener('click', closeModal);
    if (modal) {
      modal.addEventListener('click', function (e) {
        if (e.target === modal) closeModal();
      });
    }
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && modal && modal.style.display === 'flex') {
        closeModal();
      }
    });

    // Export JSON
    if (exportBtn) {
      exportBtn.addEventListener('click', function () {
        const data = {
          bookmarks: getBookmarks(),
          saved_queries: getSavedQueries(),
          exported_at: new Date().toISOString()
        };
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = 'searxgo_bookmarks.json';
        a.click();
      });
    }

    // Clear all
    if (clearBtn) {
      clearBtn.addEventListener('click', function () {
        if (confirm('Clear all pinned results and saved queries from local storage?')) {
          localStorage.removeItem('searxgo_bookmarks');
          localStorage.removeItem('searxgo_saved_queries');
          updateCounters();
          updateResultButtons();
          renderModalLists();
        }
      });
    }

    updateCounters();
    updateResultButtons();
  }

  // --- Smart Topic Clustering Filter ---
  function initTopicClusters() {
    const clusterContainer = document.querySelector('.topic-clusters-bar');
    if (!clusterContainer) return;

    clusterContainer.addEventListener('click', function (e) {
      const pill = e.target.closest('.cluster-pill');
      if (!pill) return;

      e.preventDefault();
      const targetCluster = (pill.getAttribute('data-cluster') || pill.dataset.cluster || '').trim().toLowerCase();
      if (!targetCluster) return;

      const clusterPills = clusterContainer.querySelectorAll('.cluster-pill');
      clusterPills.forEach(p => p.classList.remove('active'));
      pill.classList.add('active');

      const resultItems = document.querySelectorAll('.result-item, .image-card, .video-card');

      resultItems.forEach(item => {
        if (targetCluster === 'all') {
          item.style.display = '';
          item.style.opacity = '1';
          return;
        }

        const rawClusters = item.getAttribute('data-clusters') || item.dataset.clusters || '';
        const itemClusters = rawClusters.split(',').map(s => s.trim().toLowerCase()).filter(Boolean);

        if (itemClusters.includes(targetCluster) || (targetCluster === 'general' && itemClusters.length === 0)) {
          item.style.display = '';
          item.style.opacity = '1';
        } else {
          item.style.display = 'none';
        }
      });
    });
  }

  // --- Visual Reverse Image Search System ---
  function initReverseImageSearch() {
    const modal = document.getElementById('image-search-modal');
    if (!modal) return;

    const openBtns = document.querySelectorAll('.btn-image-search, #btn-image-search');
    const closeBtns = modal.querySelectorAll('.btn-close-modal, #btn-close-modal');
    const dropZone = document.getElementById('image-drop-zone');
    const fileInput = document.getElementById('image-file-input');
    const browseBtn = modal.querySelector('.btn-browse-file');
    const urlInput = document.getElementById('image-url-input');
    const searchUrlBtn = document.getElementById('btn-search-by-url');
    const loadingState = document.getElementById('image-upload-loading');
    const resultsPanel = document.getElementById('image-results-panel');
    const previewThumb = document.getElementById('image-preview-thumb');
    const previewFilename = document.getElementById('image-preview-filename');
    const badgeDims = document.getElementById('badge-image-dimensions');
    const badgeSize = document.getElementById('badge-image-size');
    const badgeMime = document.getElementById('badge-image-mime');
    const enginesGrid = document.getElementById('reverse-engines-grid');
    const openAllBtn = document.getElementById('btn-open-all-reverse');

    let currentEngineLinks = [];

    function openModal() {
      modal.style.display = 'flex';
      if (urlInput) {
        setTimeout(() => urlInput.focus(), 100);
      }
    }

    function closeModal() {
      modal.style.display = 'none';
      if (loadingState) loadingState.style.display = 'none';
    }

    openBtns.forEach(btn => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        openModal();
      });
    });

    closeBtns.forEach(btn => {
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        closeModal();
      });
    });

    modal.addEventListener('click', function (e) {
      if (e.target === modal) {
        closeModal();
      }
    });

    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && modal.style.display === 'flex') {
        closeModal();
      }
    });

    function formatBytes(bytes) {
      if (!bytes || bytes <= 0) return '0 B';
      const k = 1024;
      const sizes = ['B', 'KB', 'MB', 'GB'];
      const i = Math.floor(Math.log(bytes) / Math.log(k));
      return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    function renderReverseResults(data) {
      if (loadingState) loadingState.style.display = 'none';
      if (!resultsPanel) return;

      if (!data || !data.success) {
        alert(data && data.error ? 'Reverse search failed: ' + data.error : 'Failed to process image');
        return;
      }

      resultsPanel.style.display = 'block';

      if (previewThumb) previewThumb.src = data.image_url;
      if (previewFilename) previewFilename.textContent = data.filename || 'Uploaded Image';
      if (badgeDims) badgeDims.textContent = data.dimensions || 'Image';
      if (badgeSize) badgeSize.textContent = formatBytes(data.size);
      if (badgeMime) badgeMime.textContent = (data.mime_type || 'image/jpeg').replace('image/', '').toUpperCase();

      currentEngineLinks = data.engines || [];

      if (enginesGrid) {
        enginesGrid.innerHTML = '';
        currentEngineLinks.forEach(eng => {
          const card = document.createElement('a');
          card.className = 'reverse-engine-card';
          card.href = eng.url;
          card.target = '_blank';
          card.rel = 'noopener noreferrer';
          card.innerHTML = `
            <div>
              <div class="reverse-engine-header">
                <span class="reverse-engine-icon">${eng.icon || '🔍'}</span>
                <span class="reverse-engine-name">${escapeHtml(eng.name)}</span>
              </div>
              <div class="reverse-engine-desc">${escapeHtml(eng.description || '')}</div>
            </div>
            <div style="display:flex; justify-content:flex-end;">
              <span class="reverse-engine-btn">
                <span>Search Provider</span>
                <span>↗</span>
              </span>
            </div>
          `;
          enginesGrid.appendChild(card);
        });
      }
    }

    if (openAllBtn) {
      openAllBtn.addEventListener('click', function (e) {
        e.preventDefault();
        if (!currentEngineLinks || currentEngineLinks.length === 0) return;
        currentEngineLinks.forEach(eng => {
          window.open(eng.url, '_blank');
        });
      });
    }

    function processImageFile(file) {
      if (!file || !file.type.startsWith('image/')) {
        alert('Please select a valid image file (PNG, JPG, WebP, GIF).');
        return;
      }

      if (loadingState) loadingState.style.display = 'block';
      if (resultsPanel) resultsPanel.style.display = 'none';

      const formData = new FormData();
      formData.append('image', file);

      fetch('/api/reverse-image', {
        method: 'POST',
        body: formData
      })
        .then(res => res.json())
        .then(data => {
          renderReverseResults(data);
        })
        .catch(err => {
          if (loadingState) loadingState.style.display = 'none';
          alert('Upload failed: ' + err.message);
        });
    }

    function processImageUrl(url) {
      const trimmed = (url || '').trim();
      if (!trimmed) {
        alert('Please enter a valid image URL.');
        return;
      }

      if (loadingState) loadingState.style.display = 'block';
      if (resultsPanel) resultsPanel.style.display = 'none';

      fetch('/api/reverse-image', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ image_url: trimmed })
      })
        .then(res => res.json())
        .then(data => {
          renderReverseResults(data);
        })
        .catch(err => {
          if (loadingState) loadingState.style.display = 'none';
          alert('Error searching by URL: ' + err.message);
        });
    }

    // Drag and drop event handlers
    if (dropZone) {
      ['dragenter', 'dragover'].forEach(eventName => {
        dropZone.addEventListener(eventName, function (e) {
          e.preventDefault();
          e.stopPropagation();
          dropZone.classList.add('drag-over');
        });
      });

      ['dragleave', 'drop'].forEach(eventName => {
        dropZone.addEventListener(eventName, function (e) {
          e.preventDefault();
          e.stopPropagation();
          dropZone.classList.remove('drag-over');
        });
      });

      dropZone.addEventListener('drop', function (e) {
        const dt = e.dataTransfer;
        if (dt && dt.files && dt.files.length > 0) {
          processImageFile(dt.files[0]);
        }
      });

      dropZone.addEventListener('click', function (e) {
        if (fileInput) fileInput.click();
      });
    }

    if (browseBtn && fileInput) {
      browseBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        fileInput.click();
      });
    }

    if (fileInput) {
      fileInput.addEventListener('change', function () {
        if (this.files && this.files.length > 0) {
          processImageFile(this.files[0]);
        }
      });
    }

    if (searchUrlBtn && urlInput) {
      searchUrlBtn.addEventListener('click', function () {
        processImageUrl(urlInput.value);
      });

      urlInput.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          processImageUrl(urlInput.value);
        }
      });
    }

    // Global Clipboard Paste Listener (Ctrl+V / Cmd+V)
    document.addEventListener('paste', function (e) {
      if (!e.clipboardData || !e.clipboardData.items) return;
      const items = e.clipboardData.items;

      for (let i = 0; i < items.length; i++) {
        if (items[i].type && items[i].type.indexOf('image') !== -1) {
          const blob = items[i].getAsFile();
          if (blob) {
            e.preventDefault();
            openModal();
            processImageFile(blob);
            break;
          }
        }
      }
    });
  }

  // --- Ad-Free Floating Video & Audio Mini-Player ---
  function initFloatingPlayer() {
    const player = document.getElementById('floating-media-player');
    if (!player) return;

    const iframe = document.getElementById('player-iframe');
    const titleEl = document.getElementById('player-title');
    const closeBtn = document.getElementById('btn-player-close');
    const minBtn = document.getElementById('btn-player-minimize');

    if (closeBtn) {
      closeBtn.addEventListener('click', function() {
        if (iframe) iframe.src = '';
        player.style.display = 'none';
      });
    }

    if (minBtn) {
      minBtn.addEventListener('click', function() {
        player.classList.toggle('minimized');
      });
    }

    function getEmbedURL(rawUrl) {
      const ytMatch = rawUrl.match(/(?:youtu\.be\/|youtube\.com\/(?:watch\?v=|embed\/|shorts\/))([\w-]{11})/i);
      if (ytMatch && ytMatch[1]) {
        return 'https://www.youtube-nocookie.com/embed/' + ytMatch[1] + '?autoplay=1';
      }
      const vimeoMatch = rawUrl.match(/vimeo\.com\/(?:video\/)?(\d+)/i);
      if (vimeoMatch && vimeoMatch[1]) {
        return 'https://player.vimeo.com/video/' + vimeoMatch[1] + '?autoplay=1';
      }
      const dailyMatch = rawUrl.match(/dailymotion\.com\/video\/([a-zA-Z0-9]+)/i);
      if (dailyMatch && dailyMatch[1]) {
        return 'https://www.dailymotion.com/embed/video/' + dailyMatch[1] + '?autoplay=1';
      }
      return rawUrl;
    }

    document.addEventListener('click', function(e) {
      const btn = e.target.closest('.btn-play-trigger, .video-thumb-wrap');
      if (btn) {
        const url = btn.dataset.url || btn.dataset.videoUrl;
        const title = btn.dataset.title || btn.dataset.videoTitle || 'Ad-Free Stream';
        if (url && (url.includes('youtube') || url.includes('youtu.be') || url.includes('vimeo') || url.includes('dailymotion') || url.match(/\.(mp4|webm|mp3|ogg)(\?|$)/i))) {
          e.preventDefault();
          const embedUrl = getEmbedURL(url);
          if (titleEl) titleEl.textContent = title;
          if (iframe) iframe.src = embedUrl;
          player.classList.remove('minimized');
          player.style.display = 'block';
        }
      }
    });
  }

  // --- Mobile Bottom Navigation ---
  function initMobileBottomNav() {
    if (document.querySelector('.mobile-bottom-nav')) return;

    const nav = document.createElement('nav');
    nav.className = 'mobile-bottom-nav';

    const currentPath = window.location.pathname;

    const items = [
      { path: '/', label: 'Home', icon: '🪐' },
      { path: '/dorks', label: 'Dorks', icon: '🎯' },
      { path: '/sherlock', label: 'Sherlock', icon: '🕵️' },
      { path: '/scrub', label: 'Scrub', icon: '🛡️' },
      { path: '/settings', label: 'Settings', icon: '⚙️' }
    ];

    nav.innerHTML = items.map(item => {
      let isActive = false;
      if (item.path === '/' && (currentPath === '/' || currentPath === '/search')) {
        isActive = true;
      } else if (item.path !== '/' && currentPath.startsWith(item.path)) {
        isActive = true;
      }
      return `
        <a href="${item.path}" class="mobile-nav-item ${isActive ? 'active' : ''}">
          <span class="mobile-nav-icon">${item.icon}</span>
          <span class="mobile-nav-label">${item.label}</span>
        </a>
      `;
    }).join('');

    document.body.appendChild(nav);
  }

  // --- Mobile Touch Gestures & Category Swiping ---
  function initMobileTouchGestures() {
    // 1. Smoothly scroll active category tab to center on mobile load
    const activeTab = document.querySelector('.results-tabs .tab-link.active');
    const tabsContainer = document.querySelector('.results-tabs');
    if (activeTab && tabsContainer) {
      setTimeout(() => {
        const offset = activeTab.offsetLeft - (tabsContainer.clientWidth / 2) + (activeTab.clientWidth / 2);
        tabsContainer.scrollTo({ left: Math.max(0, offset), behavior: 'smooth' });
      }, 100);
    }

    // 2. Touch swipe listener across search results
    const resultsContainer = document.querySelector('.results-container, .main-results, .results-wrapper, body');
    if (!resultsContainer || !document.querySelector('.results-tabs')) return;

    let startX = 0;
    let startY = 0;
    let startTime = 0;
    let isSwiping = false;

    // Toast element for visual feedback
    let toast = document.querySelector('.swipe-indicator-toast');
    if (!toast) {
      toast = document.createElement('div');
      toast.className = 'swipe-indicator-toast';
      document.body.appendChild(toast);
    }

    function showSwipeToast(text) {
      toast.textContent = text;
      toast.classList.add('show');
      setTimeout(() => {
        toast.classList.remove('show');
      }, 800);
    }

    document.addEventListener('touchstart', function(e) {
      if (e.touches.length !== 1) return;
      const target = e.target;
      // Skip gesture if interacting with input, map, or player
      if (target.closest('input, textarea, select, button, .leaflet-container, #player-body, .suggestions-dropdown')) {
        isSwiping = false;
        return;
      }
      startX = e.touches[0].clientX;
      startY = e.touches[0].clientY;
      startTime = Date.now();
      isSwiping = true;
    }, { passive: true });

    document.addEventListener('touchend', function(e) {
      if (!isSwiping || e.changedTouches.length !== 1) return;
      isSwiping = false;

      const endX = e.changedTouches[0].clientX;
      const endY = e.changedTouches[0].clientY;
      const deltaX = endX - startX;
      const deltaY = endY - startY;
      const elapsed = Date.now() - startTime;

      // Minimum swipe distance of 60px, max vertical deviation of 50px, under 600ms
      if (Math.abs(deltaX) > 60 && Math.abs(deltaY) < 50 && elapsed < 600) {
        const tabs = Array.from(document.querySelectorAll('.results-tabs .tab-link'));
        if (tabs.length === 0) return;

        const currentIdx = tabs.findIndex(t => t.classList.contains('active'));
        if (currentIdx === -1) return;

        let targetIdx = -1;
        if (deltaX < 0 && currentIdx < tabs.length - 1) {
          // Swipe Left -> Next Tab
          targetIdx = currentIdx + 1;
        } else if (deltaX > 0 && currentIdx > 0) {
          // Swipe Right -> Prev Tab
          targetIdx = currentIdx - 1;
        }

        if (targetIdx !== -1 && tabs[targetIdx]) {
          const nextTab = tabs[targetIdx];
          showSwipeToast(`Switching to ${nextTab.textContent.trim()} ➔`);
          setTimeout(() => {
            window.location.href = nextTab.href;
          }, 150);
        }
      }
    }, { passive: true });
  }

  // Safe DOM ready initialization
  function initAll() {
    initTheme();
    initServiceWorker();
    initUserPreferencesOnLoad();
    initSearchBox();
    initFilterToolbar();
    initLeafletMap();
    initVideoModal();
    initMagnetButtons();
    initVimKeybindings();
    initCommandPalette();
    initCustomBangs();
    initEncryptedVault();
    initInfiniteScroll();
    initSettingsPage();
    initBookmarks();
    initTopicClusters();
    initReverseImageSearch();
    initFloatingPlayer();
    initMobileBottomNav();
    initMobileTouchGestures();
    initNavDropdown();
  }

  // --- Tools Nav Dropdown ---
  function initNavDropdown() {
    const dropdowns = document.querySelectorAll('.nav-dropdown');
    dropdowns.forEach(dropdown => {
      const btn = dropdown.querySelector('.nav-dropdown-trigger');
      const panel = dropdown.querySelector('.nav-dropdown-panel');
      if (!btn || !panel) return;

      function openDropdown() {
        dropdown.classList.add('open');
        btn.setAttribute('aria-expanded', 'true');
      }
      function closeDropdown() {
        dropdown.classList.remove('open');
        btn.setAttribute('aria-expanded', 'false');
      }
      function toggleDropdown() {
        dropdown.classList.contains('open') ? closeDropdown() : openDropdown();
      }

      btn.addEventListener('click', e => {
        e.stopPropagation();
        dropdowns.forEach(other => { if (other !== dropdown) other.classList.remove('open'); });
        toggleDropdown();
      });

      document.addEventListener('click', e => {
        if (!dropdown.contains(e.target)) closeDropdown();
      });

      document.addEventListener('keydown', e => {
        if (e.key === 'Escape') closeDropdown();
      });

      panel.addEventListener('keydown', e => {
        const items = [...panel.querySelectorAll('.dropdown-item')];
        const idx = items.indexOf(document.activeElement);
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          items[(idx + 1) % items.length]?.focus();
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          items[(idx - 1 + items.length) % items.length]?.focus();
        }
      });
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAll);
  } else {
    initAll();
  }
})();

// ================================================================
// 🎯 Search Goggles — Client-Side Domain Block/Boost Filter
// Reads from localStorage and filters search result cards live
// ================================================================
(function initGogglesFilter() {
  function getGoggles() {
    try { return JSON.parse(localStorage.getItem('searxgo_goggles') || '{"blocklist":[],"boostlist":[]}'); }
    catch { return { blocklist: [], boostlist: [] }; }
  }

  function applyGoggles() {
    const g = getGoggles();
    if (!g.blocklist.length && !g.boostlist.length) return;

    const resultCards = document.querySelectorAll('.result-card, .result-item, [data-result-url]');
    let blockedCount = 0;
    let boostedCount = 0;

    resultCards.forEach(card => {
      // Try to get the URL from the card
      let resultURL = card.dataset.resultUrl || '';
      if (!resultURL) {
        const link = card.querySelector('a[href^="http"]');
        if (link) resultURL = link.href;
      }
      if (!resultURL) return;

      let domain = '';
      try {
        const u = new URL(resultURL);
        domain = u.hostname.replace(/^www\./, '').toLowerCase();
      } catch { return; }

      // Block
      if (g.blocklist.some(b => domain === b || domain.endsWith('.' + b))) {
        card.style.display = 'none';
        blockedCount++;
        return;
      }

      // Boost — move to top and highlight
      if (g.boostlist.some(b => domain === b || domain.endsWith('.' + b))) {
        card.style.outline = '1px solid rgba(16,185,129,0.4)';
        card.style.background = 'rgba(16,185,129,0.04)';
        if (!card.querySelector('.goggle-boost-badge')) {
          const badge = document.createElement('span');
          badge.className = 'goggle-boost-badge';
          badge.textContent = '⬆️ Boosted';
          badge.style.cssText = 'font-size:0.68rem;padding:0.1rem 0.4rem;background:rgba(16,185,129,0.2);color:#10b981;border-radius:4px;font-weight:700;margin-left:0.35rem;';
          const titleEl = card.querySelector('h2 a, .result-title a, h3 a');
          if (titleEl) titleEl.parentNode.insertBefore(badge, titleEl.nextSibling);
        }
        boostedCount++;
      }
    });

    // Show Goggles status banner if anything was filtered
    if ((blockedCount > 0 || boostedCount > 0) && !document.getElementById('goggles-banner')) {
      const banner = document.createElement('div');
      banner.id = 'goggles-banner';
      banner.style.cssText = `
        background: rgba(99,102,241,0.1);
        border: 1px solid rgba(99,102,241,0.25);
        border-radius: 8px;
        padding: 0.6rem 1rem;
        margin-bottom: 1rem;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 0.75rem;
        font-size: 0.82rem;
        color: var(--text-secondary);
        flex-wrap: wrap;
      `;
      banner.innerHTML = `
        <span>🎯 <strong>Goggles active</strong> —
          ${blockedCount > 0 ? `<span style="color:#ef4444;">${blockedCount} blocked</span>` : ''}
          ${blockedCount > 0 && boostedCount > 0 ? ' · ' : ''}
          ${boostedCount > 0 ? `<span style="color:#10b981;">${boostedCount} boosted</span>` : ''}
          &nbsp;·&nbsp; <a href="/graph" style="color:var(--accent-primary); text-decoration:none;">Manage in Graph Explorer</a>
        </span>
        <button type="button" id="goggles-banner-close" style="background:none;border:none;color:var(--text-muted);cursor:pointer;font-size:1rem;">✕</button>
      `;
      const container = document.querySelector('.results-container, main, .container');
      if (container) container.insertBefore(banner, container.firstChild);
      document.getElementById('goggles-banner-close')?.addEventListener('click', () => banner.remove());
    }
  }

  // Global Tools Nav Dropdown toggle handler
  function initNavDropdowns() {
    document.addEventListener('click', function (e) {
      const trigger = e.target.closest('.nav-dropdown-trigger');
      if (trigger) {
        e.preventDefault();
        e.stopPropagation();
        const dropdown = trigger.closest('.nav-dropdown');
        if (!dropdown) return;
        const isOpen = dropdown.classList.contains('open');
        document.querySelectorAll('.nav-dropdown.open').forEach(d => {
          if (d !== dropdown) d.classList.remove('open');
        });
        dropdown.classList.toggle('open', !isOpen);
        trigger.setAttribute('aria-expanded', !isOpen ? 'true' : 'false');
        return;
      }
      if (!e.target.closest('.nav-dropdown-panel')) {
        document.querySelectorAll('.nav-dropdown.open').forEach(d => {
          d.classList.remove('open');
          const t = d.querySelector('.nav-dropdown-trigger');
          if (t) t.setAttribute('aria-expanded', 'false');
        });
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      applyGoggles();
      initNavDropdowns();
    });
  } else {
    applyGoggles();
    initNavDropdowns();
  }
})();
