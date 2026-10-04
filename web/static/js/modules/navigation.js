/**
 * SearXGo Module: Navigation, Command Palette, Modals, Gestures & Keybindings
 */
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
  let vimKeybindingsInitialized = false;
  function initVimKeybindings() {
    if (vimKeybindingsInitialized) return;
    vimKeybindingsInitialized = true;

    let currentIndex = -1;

    function getItems() {
      return document.querySelectorAll('.result-item, .video-card, .image-card');
    }

    document.addEventListener('keydown', function (e) {
      const savedHotkeys = getCookie('searxgo_hotkeys');
      if (savedHotkeys === 'off') return;

      const input = document.querySelector('.search-input');
      if (document.activeElement === input || (document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA' || document.activeElement.isContentEditable))) {
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
  let cmdPaletteInitialized = false;
  function initCommandPalette() {
    let backdrop = document.getElementById('searxgo-cmd-palette');
    if (!backdrop) {
      backdrop = document.createElement('div');
      backdrop.id = 'searxgo-cmd-palette';
      backdrop.setAttribute('data-hx-preserve', 'true');
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
      { category: 'Navigation', icon: '🚨', title: 'Live CVE & Zero-Day Vulnerability Feed', action: () => window.location.href = '/cve' },
      { category: 'Navigation', icon: '🌐', title: 'IP Intelligence, ASN & BGP Route Visualizer', action: () => window.location.href = '/ip-intel' },
      { category: 'Navigation', icon: '⚡', title: 'In-Browser API & cURL Playground', action: () => window.location.href = '/api-tester' },
      { category: 'Navigation', icon: '☁️', title: 'Cloud Bucket & Public Storage Recon', action: () => window.location.href = '/cloud-recon' },
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
      { category: 'Appearance', icon: '🌙', title: 'Theme: Neutral Dark (Default)', action: () => { window.setTheme('dark'); showToast('Switched to Neutral Dark theme'); } },
      { category: 'Appearance', icon: '⚪', title: 'Theme: Neutral Palette (Slate & Zinc)', action: () => { window.setTheme('neutral'); showToast('Switched to Neutral theme'); } },
      { category: 'Appearance', icon: '☀️', title: 'Theme: Light Minimal', action: () => { window.setTheme('light'); showToast('Switched to Light Minimal theme'); } },
      { category: 'Appearance', icon: '🖤', title: 'Theme: OLED Pitch Black', action: () => { window.setTheme('black'); showToast('Switched to OLED Black theme'); } },
      { category: 'Appearance', icon: '🧛', title: 'Theme: Dracula Theme', action: () => { window.setTheme('dracula'); showToast('Switched to Dracula theme'); } },
      { category: 'Appearance', icon: '⚡', title: 'Theme: Cyberpunk Neon', action: () => { window.setTheme('cyberpunk'); showToast('Switched to Cyberpunk Neon theme'); } },

      // Tools & Intelligence Suite
      { category: 'Tools & Intelligence', icon: '🚨', title: 'Live CVE & Zero-Day Feed (/cve)', action: () => window.location.href = '/cve' },
      { category: 'Tools & Intelligence', icon: '🌐', title: 'IP Intelligence, ASN & BGP Route Visualizer (/ip-intel)', action: () => window.location.href = '/ip-intel' },
      { category: 'Tools & Intelligence', icon: '☁️', title: 'Cloud Bucket & Storage Recon (/cloud-recon)', action: () => window.location.href = '/cloud-recon' },
      { category: 'Tools & Intelligence', icon: '🧪', title: 'In-Browser API & cURL Playground (/api-tester)', action: () => window.location.href = '/api-tester' },
      { category: 'Tools & Intelligence', icon: '🕵️', title: 'Sherlock OSINT Username Recon (/sherlock)', action: () => window.location.href = '/sherlock' },
      { category: 'Tools & Intelligence', icon: '⚡', title: 'Tech Stack Inspector (/tech)', action: () => window.location.href = '/tech' },
      { category: 'Tools & Intelligence', icon: '🔬', title: 'Domain Recon & SSL Audit (/recon)', action: () => window.location.href = '/recon' },
      { category: 'Tools & Intelligence', icon: '🛡️', title: 'Threat Scanner & Phishing Check (/threat)', action: () => window.location.href = '/threat' },
      { category: 'Tools & Intelligence', icon: '📰', title: 'World News Pulse Live Portals (/news-hub)', action: () => window.location.href = '/news-hub' },
      { category: 'Tools & Intelligence', icon: '⛅', title: 'Weather Radar & 7-Day Forecast (/weather)', action: () => window.location.href = '/weather' },
      { category: 'Tools & Intelligence', icon: '💱', title: 'Currency & Crypto Exchange Converter (/currency)', action: () => window.location.href = '/currency' },
      { category: 'Tools & Intelligence', icon: '🔓', title: 'Paywall Bypass Clean Mirror Reader (/bypass)', action: () => window.location.href = '/bypass' },
      { category: 'Tools & Intelligence', icon: '📥', title: 'Media Downloader (/media)', action: () => window.location.href = '/media' },
      { category: 'Tools & Intelligence', icon: '📱', title: 'QR & WiFi Studio Generator (/qr)', action: () => window.location.href = '/qr' },
      { category: 'Tools & Intelligence', icon: '📖', title: 'Distraction-Free Reader View (/reader)', action: () => window.location.href = '/reader' },
      { category: 'Tools & Intelligence', icon: '🕸️', title: 'Knowledge Graph Explorer (/graph)', action: () => window.location.href = '/graph' },
      { category: 'Tools & Intelligence', icon: '🧹', title: 'EXIF & GPS Metadata Stripper (/scrub)', action: () => window.location.href = '/scrub' },
      { category: 'Tools & Intelligence', icon: '⚖️', title: 'Split Compare Dual Engine Search (/split)', action: () => window.location.href = '/split' },

      // Quick Actions
      { category: 'Actions', icon: '⚙️', title: 'Open Search Preferences & Settings (/settings)', action: () => window.location.href = '/settings' },
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

    if (cmdPaletteInitialized) return;
    cmdPaletteInitialized = true;

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


  // --- Mobile Bottom Navigation ---
  function initMobileBottomNav() {
    let nav = document.querySelector('.mobile-bottom-nav');
    const currentPath = window.location.pathname;

    const items = [
      { path: '/', label: 'Home', icon: '🪐' },
      { path: '/dorks', label: 'Dorks', icon: '🎯' },
      { path: '/sherlock', label: 'Sherlock', icon: '🕵️' },
      { path: '/scrub', label: 'Scrub', icon: '🛡️' },
      { path: '/settings', label: 'Settings', icon: '⚙️' }
    ];

    if (!nav) {
      nav = document.createElement('nav');
      nav.className = 'mobile-bottom-nav';
      document.body.appendChild(nav);
    }

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
  }

  // --- Mobile Touch Gestures & Category Swiping ---
  let touchGesturesInitialized = false;
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

    if (touchGesturesInitialized) return;
    touchGesturesInitialized = true;

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


  // --- Tools Nav Dropdown (Consolidated, Accessible & Delegated) ---
  let navDropdownInitialized = false;
  function initNavDropdown() {
    if (navDropdownInitialized) return;
    navDropdownInitialized = true;

    document.addEventListener('click', function (e) {
      const trigger = e.target.closest('.nav-dropdown-trigger');
      if (trigger) {
        e.preventDefault();
        e.stopPropagation();
        const dropdown = trigger.closest('.nav-dropdown');
        if (!dropdown) return;
        const isOpen = dropdown.classList.contains('open');
        document.querySelectorAll('.nav-dropdown.open').forEach(d => {
          if (d !== dropdown) {
            d.classList.remove('open');
            d.querySelector('.nav-dropdown-trigger')?.setAttribute('aria-expanded', 'false');
          }
        });
        dropdown.classList.toggle('open', !isOpen);
        trigger.setAttribute('aria-expanded', !isOpen ? 'true' : 'false');
        return;
      }

      if (e.target.closest('.dropdown-item')) {
        document.querySelectorAll('.nav-dropdown.open').forEach(d => {
          d.classList.remove('open');
          d.querySelector('.nav-dropdown-trigger')?.setAttribute('aria-expanded', 'false');
        });
        return;
      }

      if (!e.target.closest('.nav-dropdown-panel')) {
        document.querySelectorAll('.nav-dropdown.open').forEach(d => {
          d.classList.remove('open');
          d.querySelector('.nav-dropdown-trigger')?.setAttribute('aria-expanded', 'false');
        });
      }
    });

    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') {
        document.querySelectorAll('.nav-dropdown.open').forEach(d => {
          d.classList.remove('open');
          d.querySelector('.nav-dropdown-trigger')?.setAttribute('aria-expanded', 'false');
        });
      }
    });
  }


