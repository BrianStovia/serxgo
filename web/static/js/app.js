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

  // --- Vim Navigation Shortcuts ---
  function initVimKeybindings() {
    let currentIndex = -1;
    const items = document.querySelectorAll('.result-item');
    const input = document.querySelector('.search-input');

    document.addEventListener('keydown', function (e) {
      if (document.activeElement === input || document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA') {
        return;
      }

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
        highlightResult(currentIndex);
      } else if (e.key === 'k' || e.key === 'ArrowUp') {
        if (items.length === 0) return;
        e.preventDefault();
        currentIndex = Math.max(currentIndex - 1, 0);
        highlightResult(currentIndex);
      } else if (e.key === 'Enter') {
        if (currentIndex >= 0 && currentIndex < items.length) {
          const link = items[currentIndex].querySelector('.result-title a');
          if (link) link.click();
        }
      } else if (e.key === 'c') {
        if (currentIndex >= 0 && currentIndex < items.length) {
          const cached = items[currentIndex].querySelector('.cached-link');
          if (cached) cached.click();
        }
      }
    });

    function highlightResult(index) {
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
    initInfiniteScroll();
    initSettingsPage();
    initBookmarks();
    initTopicClusters();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAll);
  } else {
    initAll();
  }
})();


