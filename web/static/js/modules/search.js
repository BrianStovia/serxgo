/**
 * SearXGo Module: Search, Autocomplete, Filters, Media Modals & Infinite Scroll
 */
  // --- Autocomplete & Search Box ---
  let searchBoxDropdownClickInit = false;
  function initSearchBox() {
    const input = document.querySelector('.search-input');
    const clearBtn = document.querySelector('.btn-clear');
    const dropdown = document.querySelector('.suggestions-dropdown');
    const form = document.querySelector('.search-form');

    if (!input || !dropdown) return;
    if (input.dataset.searchBoxInit === 'true') return;
    input.dataset.searchBoxInit = 'true';

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

    if (!searchBoxDropdownClickInit) {
      searchBoxDropdownClickInit = true;
      document.addEventListener('click', function (e) {
        const dd = document.querySelector('.suggestions-dropdown');
        const inp = document.querySelector('.search-input');
        if (dd && inp && !inp.contains(e.target) && !dd.contains(e.target)) {
          dd.classList.remove('active');
          dd.innerHTML = '';
        }
      });
    }

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

    if (!timeSel && !safeSel && !langSel) return;
    if (timeSel && timeSel.dataset.filterInit) return;
    if (timeSel) timeSel.dataset.filterInit = 'true';
    if (safeSel) safeSel.dataset.filterInit = 'true';
    if (langSel) langSel.dataset.filterInit = 'true';

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
    if (!mapEl || typeof L === 'undefined' || mapEl._leaflet_id) return;

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
      if (wrap.dataset.videoThumbInit) return;
      wrap.dataset.videoThumbInit = 'true';
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

    if (modal.dataset.videoModalInit) return;
    modal.dataset.videoModalInit = 'true';

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
      if (btn.dataset.magnetInit) return;
      btn.dataset.magnetInit = 'true';
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


  // --- Dynamic Infinite Scroll & Unlimited Search ---
  function initInfiniteScroll() {
    const resultsList = document.querySelector('.results-list');
    const imageGrid = document.querySelector('.image-grid');
    const videoGrid = document.querySelector('.video-grid');
    const pagination = document.querySelector('.pagination');
    const loadMoreBtn = document.getElementById('btn-load-more');

    const container = resultsList || imageGrid || videoGrid;
    if (!container) return;
    if (container.dataset.infiniteScrollInit) return;
    container.dataset.infiniteScrollInit = 'true';

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

    if (loadMoreBtn && !loadMoreBtn.hasAttribute('hx-get')) {
      loadMoreBtn.addEventListener('click', function () {
        fetchNextPage();
      });
    }

    if (isAutoInfinite && (!loadMoreBtn || !loadMoreBtn.hasAttribute('hx-get'))) {
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


  // --- Category Tabs Switcher (Search Homepage) ---
  function initCategoryTabs() {
    document.querySelectorAll('.category-tab').forEach(tab => {
      if (tab.dataset.tabInit) return;
      tab.dataset.tabInit = 'true';
      tab.addEventListener('click', function () {
        document.querySelectorAll('.category-tab').forEach(t => t.classList.remove('active'));
        this.classList.add('active');
        const inp = this.querySelector('input');
        if (inp) inp.checked = true;
      });
    });
  }


