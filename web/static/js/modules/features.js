/**
 * SearXGo Module: Bookmarks, Goggles, Topic Clusters, Floating Player & Visual Search
 */
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
      if (btn.dataset.bmInit) return;
      btn.dataset.bmInit = 'true';
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
      if (btn.dataset.saveSearchInit) return;
      btn.dataset.saveSearchInit = 'true';
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
      if (btn.dataset.bmOpenInit) return;
      btn.dataset.bmOpenInit = 'true';
      btn.addEventListener('click', function (e) {
        e.preventDefault();
        renderModalLists();
        if (modal) modal.style.display = 'flex';
      });
    });

    if (modal && !modal.dataset.bmModalInit) {
      modal.dataset.bmModalInit = 'true';
      // Close Modal
      function closeModal() {
        if (modal) modal.style.display = 'none';
      }
      if (closeBtn) closeBtn.addEventListener('click', closeModal);
      modal.addEventListener('click', function (e) {
        if (e.target === modal) closeModal();
      });
      document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape' && modal && modal.style.display === 'flex') {
          closeModal();
        }
      });
    }

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
    if (clusterContainer.dataset.clusterInit) return;
    clusterContainer.dataset.clusterInit = 'true';

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
    if (modal.dataset.imageSearchInit) return;
    modal.dataset.imageSearchInit = 'true';

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
  let floatingPlayerClickInit = false;
  function initFloatingPlayer() {
    const player = document.getElementById('floating-media-player');
    if (!player) return;

    const iframe = document.getElementById('player-iframe');
    const titleEl = document.getElementById('player-title');
    const closeBtn = document.getElementById('btn-player-close');
    const minBtn = document.getElementById('btn-player-minimize');

    if (!player.dataset.playerInit) {
      player.dataset.playerInit = 'true';
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

    if (!floatingPlayerClickInit) {
      floatingPlayerClickInit = true;
      document.addEventListener('click', function(e) {
        const btn = e.target.closest('.btn-play-trigger, .video-thumb-wrap');
        if (btn) {
          const url = btn.dataset.url || btn.dataset.videoUrl;
          const title = btn.dataset.title || btn.dataset.videoTitle || 'Ad-Free Stream';
          if (url && (url.includes('youtube') || url.includes('youtu.be') || url.includes('vimeo') || url.includes('dailymotion') || url.match(/\.(mp4|webm|mp3|ogg)(\?|$)/i))) {
            e.preventDefault();
            const embedUrl = getEmbedURL(url);
            const activePlayer = document.getElementById('floating-media-player');
            const activeTitle = document.getElementById('player-title');
            const activeIframe = document.getElementById('player-iframe');
            if (activeTitle) activeTitle.textContent = title;
            if (activeIframe) activeIframe.src = embedUrl;
            if (activePlayer) {
              activePlayer.classList.remove('minimized');
              activePlayer.style.display = 'block';
            }
          }
        }
      });
    }
  }


  // ================================================================
  // 🎯 Search Goggles — Client-Side Domain Block/Boost Filter
  // Reads from localStorage and filters search result cards live
  // ================================================================
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


