/**
 * SearXGo Module: Settings, Custom Bangs & Encrypted Vault
 */
  // --- Custom Bangs & Custom Engines Builder ---
  let customBangsSubmitInitialized = false;
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
    if (addBtn && !addBtn.dataset.bangAddInit) {
      addBtn.dataset.bangAddInit = 'true';
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

    if (!customBangsSubmitInitialized) {
      customBangsSubmitInitialized = true;
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

    if (exportBtn && !exportBtn.dataset.vaultInit) {
      exportBtn.dataset.vaultInit = 'true';
      exportBtn.addEventListener('click', exportEncryptedVault);
    }

    if (importInput && !importInput.dataset.vaultInit) {
      importInput.dataset.vaultInit = 'true';
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


  // --- Settings Page Tabs & Operations ---
  function initSettingsPage() {
    const form = document.querySelector('#settings-form');
    if (!form) return;

    // Apply engine ping bar percentages automatically
    document.querySelectorAll('.engine-ping-bar[data-pct]').forEach(function (el) {
      const pct = el.getAttribute('data-pct');
      if (pct) el.style.width = pct + '%';
    });

    if (form.dataset.settingsInit === 'true') return;
    form.dataset.settingsInit = 'true';

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

    const proxyToggle = document.getElementById('proxy-toggle');
    if (proxyToggle) {
      const savedProxy = getCookie('searxgo_proxy');
      if (savedProxy !== null) proxyToggle.checked = (savedProxy === 'true');
    }

    const favSelect = document.getElementById('favicon-resolver');
    if (favSelect) {
      const savedFav = getCookie('searxgo_favicon_resolver');
      if (savedFav !== null) favSelect.value = savedFav;
    }

    const centerToggle = document.getElementById('center-align-toggle');
    if (centerToggle) {
      const savedCenter = getCookie('searxgo_center_alignment');
      if (savedCenter !== null) centerToggle.checked = (savedCenter === 'true');
    }

    const hotkeysSelect = document.getElementById('hotkeys-select');
    if (hotkeysSelect) {
      const saved = getCookie('searxgo_hotkeys');
      if (saved) hotkeysSelect.value = saved;
    }

    const urlFormatSelect = document.getElementById('url-formatting-select');
    if (urlFormatSelect) {
      const saved = getCookie('searxgo_url_formatting');
      if (saved) urlFormatSelect.value = saved;
    }

    const uiLocaleSelect = document.getElementById('ui-locale');
    if (uiLocaleSelect) {
      const saved = getCookie('searxgo_ui_locale');
      if (saved) uiLocaleSelect.value = saved;
    }

    const engineTokensInput = document.getElementById('engine-tokens');
    if (engineTokensInput) {
      const saved = getCookie('searxgo_tokens');
      if (saved) engineTokensInput.value = saved;
    }

    const unitConverterToggle = document.getElementById('unit-converter-toggle');
    if (unitConverterToggle) {
      const saved = getCookie('searxgo_unit_converter');
      if (saved !== null) unitConverterToggle.checked = (saved === 'true');
    }

    const doiRewriteToggle = document.getElementById('doi-rewrite-toggle');
    if (doiRewriteToggle) {
      const saved = getCookie('searxgo_doi_rewrite');
      if (saved !== null) doiRewriteToggle.checked = (saved === 'true');
    }

    const cachedLinksToggle = document.getElementById('cached-links-toggle');
    if (cachedLinksToggle) {
      const saved = getCookie('searxgo_cached_links');
      if (saved !== null) cachedLinksToggle.checked = (saved === 'true');
    }

    const spamGuardToggle = document.getElementById('spam-guard-toggle');
    if (spamGuardToggle) {
      const saved = getCookie('searxgo_spam_guard');
      if (saved !== null) spamGuardToggle.checked = (saved === 'true');
    }

    const searchOnCatToggle = document.getElementById('search-on-cat-toggle');
    if (searchOnCatToggle) {
      const saved = getCookie('searxgo_search_on_cat');
      if (saved !== null) searchOnCatToggle.checked = (saved === 'true');
    }

    const queryInTitleToggle = document.getElementById('query-in-title-toggle');
    if (queryInTitleToggle) {
      const saved = getCookie('searxgo_query_in_title');
      if (saved !== null) queryInTitleToggle.checked = (saved === 'true');
    }

    const savedCats = getCookie('searxgo_categories') || (function(){
      try { return localStorage.getItem('searxgo_categories'); } catch(e){ return null; }
    })();
    if (savedCats) {
      const activeCats = savedCats.split(',').map(s => s.trim().toLowerCase()).filter(Boolean);
      form.querySelectorAll('input[name="default_categories"]').forEach(cb => {
        const isChecked = activeCats.includes(cb.value.toLowerCase());
        cb.checked = isChecked;
        const box = cb.closest('.category-toggle-box');
        if (box) {
          if (isChecked) box.classList.add('active');
          else box.classList.remove('active');
        }
      });
    } else {
      // Sync active class with server-rendered checked state
      form.querySelectorAll('input[name="default_categories"]').forEach(cb => {
        const box = cb.closest('.category-toggle-box');
        if (box) {
          if (cb.checked) box.classList.add('active');
          else box.classList.remove('active');
        }
      });
    }

    // Toggle box change listener to sync .active class immediately
    form.querySelectorAll('input[name="default_categories"]').forEach(cb => {
      cb.addEventListener('change', function() {
        const box = cb.closest('.category-toggle-box');
        if (box) {
          if (cb.checked) box.classList.add('active');
          else box.classList.remove('active');
        }
      });
    });

    // Select All / Clear All category buttons
    const btnSelectAll = document.getElementById('btn-select-all-cats');
    if (btnSelectAll) {
      btnSelectAll.addEventListener('click', function() {
        form.querySelectorAll('input[name="default_categories"]').forEach(cb => {
          cb.checked = true;
          const box = cb.closest('.category-toggle-box');
          if (box) box.classList.add('active');
        });
      });
    }
    const btnClearAll = document.getElementById('btn-clear-all-cats');
    if (btnClearAll) {
      btnClearAll.addEventListener('click', function() {
        form.querySelectorAll('input[name="default_categories"]').forEach(cb => {
          cb.checked = false;
          const box = cb.closest('.category-toggle-box');
          if (box) box.classList.remove('active');
        });
      });
    }

    // JSON Export
    const btnExport = document.getElementById('btn-export-json');
    if (btnExport) {
      btnExport.addEventListener('click', function () {
        const engines = [];
        form.querySelectorAll('input[name^="engine_"]:checked').forEach(cb => engines.push(cb.value));
        const categories = [];
        form.querySelectorAll('input[name="default_categories"]:checked').forEach(cb => categories.push(cb.value));
        const data = {
          theme: themeSelect ? themeSelect.value : 'dark',
          safesearch: safeSearchSelect ? safeSearchSelect.value : '0',
          language: langSelect ? langSelect.value : '',
          infinite_scroll: infiniteToggle ? infiniteToggle.checked : false,
          new_tab: newtabToggle ? newtabToggle.checked : true,
          method: methodSelect ? methodSelect.value : 'GET',
          hotkeys: hotkeysSelect ? hotkeysSelect.value : 'vim',
          url_formatting: urlFormatSelect ? urlFormatSelect.value : 'pretty',
          ui_locale: uiLocaleSelect ? uiLocaleSelect.value : 'en',
          engine_tokens: engineTokensInput ? engineTokensInput.value : '',
          unit_converter: unitConverterToggle ? unitConverterToggle.checked : true,
          doi_rewrite: doiRewriteToggle ? doiRewriteToggle.checked : true,
          cached_links: cachedLinksToggle ? cachedLinksToggle.checked : true,
          spam_guard: spamGuardToggle ? spamGuardToggle.checked : true,
          search_on_category: searchOnCatToggle ? searchOnCatToggle.checked : true,
          query_in_title: queryInTitleToggle ? queryInTitleToggle.checked : true,
          categories: categories,
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
            if (conf.hotkeys && hotkeysSelect) hotkeysSelect.value = conf.hotkeys;
            if (conf.url_formatting && urlFormatSelect) urlFormatSelect.value = conf.url_formatting;
            if (conf.ui_locale && uiLocaleSelect) uiLocaleSelect.value = conf.ui_locale;
            if (conf.engine_tokens !== undefined && engineTokensInput) engineTokensInput.value = conf.engine_tokens;
            if (conf.unit_converter !== undefined && unitConverterToggle) unitConverterToggle.checked = conf.unit_converter;
            if (conf.doi_rewrite !== undefined && doiRewriteToggle) doiRewriteToggle.checked = conf.doi_rewrite;
            if (conf.cached_links !== undefined && cachedLinksToggle) cachedLinksToggle.checked = conf.cached_links;
            if (conf.spam_guard !== undefined && spamGuardToggle) spamGuardToggle.checked = conf.spam_guard;
            if (conf.search_on_category !== undefined && searchOnCatToggle) searchOnCatToggle.checked = conf.search_on_category;
            if (conf.query_in_title !== undefined && queryInTitleToggle) queryInTitleToggle.checked = conf.query_in_title;

            if (Array.isArray(conf.categories)) {
              form.querySelectorAll('input[name="default_categories"]').forEach(cb => {
                cb.checked = conf.categories.includes(cb.value);
              });
            }
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
          setCookie('searxgo_new_tab', '', -1);
          setCookie('searxgo_method', '', -1);
          setCookie('searxgo_redirects', '', -1);
          setCookie('searxgo_proxy', '', -1);
          setCookie('searxgo_tracker_remover', '', -1);
          setCookie('searxgo_favicon_resolver', '', -1);
          setCookie('searxgo_doi_resolver', '', -1);
          setCookie('searxgo_center_alignment', '', -1);
          setCookie('searxgo_categories', '', -1);
          setCookie('searxgo_hotkeys', '', -1);
          setCookie('searxgo_url_formatting', '', -1);
          setCookie('searxgo_ui_locale', '', -1);
          setCookie('searxgo_tokens', '', -1);
          setCookie('searxgo_unit_converter', '', -1);
          setCookie('searxgo_doi_rewrite', '', -1);
          setCookie('searxgo_cached_links', '', -1);
          setCookie('searxgo_spam_guard', '', -1);
          setCookie('searxgo_search_on_cat', '', -1);
          setCookie('searxgo_query_in_title', '', -1);
          localStorage.removeItem('searxgo_theme');
          window.location.reload();
        }
      });
    }

    // Form Save Handler
    form.addEventListener('submit', function (e) {
      // 1. Immediately collect and persist all preferences to document.cookie & localStorage
      const checkedEngines = [];
      form.querySelectorAll('input[name^="engine_"]:checked').forEach(cb => {
        checkedEngines.push(cb.value);
      });
      setCookie('searxgo_engines', checkedEngines.join(','), 365);
      try { localStorage.setItem('searxgo_engines', checkedEngines.join(',')); } catch(e){}

      if (safeSearchSelect) {
        setCookie('searxgo_safesearch', safeSearchSelect.value, 365);
        try { localStorage.setItem('searxgo_safesearch', safeSearchSelect.value); } catch(e){}
      }
      if (langSelect) {
        setCookie('searxgo_language', langSelect.value, 365);
        try { localStorage.setItem('searxgo_language', langSelect.value); } catch(e){}
      }
      if (themeSelect) {
        window.setTheme(themeSelect.value);
        setCookie('searxgo_theme', themeSelect.value, 365);
      }

      if (infiniteToggle) setCookie('searxgo_infinite_scroll', infiniteToggle.checked ? 'true' : 'false', 365);
      if (newtabToggle) {
        setCookie('searxgo_newtab', newtabToggle.checked ? 'true' : 'false', 365);
        setCookie('searxgo_new_tab', newtabToggle.checked ? 'true' : 'false', 365);
      }
      if (methodSelect) setCookie('searxgo_method', methodSelect.value, 365);
      if (hotkeysSelect) setCookie('searxgo_hotkeys', hotkeysSelect.value, 365);
      if (urlFormatSelect) setCookie('searxgo_url_formatting', urlFormatSelect.value, 365);
      if (uiLocaleSelect) setCookie('searxgo_ui_locale', uiLocaleSelect.value, 365);
      if (engineTokensInput) setCookie('searxgo_tokens', engineTokensInput.value.trim(), 365);

      if (unitConverterToggle) setCookie('searxgo_unit_converter', unitConverterToggle.checked ? 'true' : 'false', 365);
      if (doiRewriteToggle) setCookie('searxgo_doi_rewrite', doiRewriteToggle.checked ? 'true' : 'false', 365);
      if (cachedLinksToggle) setCookie('searxgo_cached_links', cachedLinksToggle.checked ? 'true' : 'false', 365);
      if (spamGuardToggle) setCookie('searxgo_spam_guard', spamGuardToggle.checked ? 'true' : 'false', 365);
      if (searchOnCatToggle) setCookie('searxgo_search_on_cat', searchOnCatToggle.checked ? 'true' : 'false', 365);
      if (queryInTitleToggle) setCookie('searxgo_query_in_title', queryInTitleToggle.checked ? 'true' : 'false', 365);

      const redirectsToggle = document.getElementById('redirects-toggle');
      if (redirectsToggle) setCookie('searxgo_redirects', redirectsToggle.checked ? 'true' : 'false', 365);

      const proxyToggleEl = document.getElementById('proxy-toggle');
      if (proxyToggleEl) setCookie('searxgo_proxy', proxyToggleEl.checked ? 'true' : 'false', 365);

      const acSelectEl = document.getElementById('autocomplete-provider');
      if (acSelectEl) setCookie('searxgo_autocomplete', acSelectEl.value, 365);

      const favSelectEl = document.getElementById('favicon-resolver');
      if (favSelectEl) setCookie('searxgo_favicon_resolver', favSelectEl.value, 365);

      const trackerToggleEl = document.getElementById('tracker-stripper-toggle');
      if (trackerToggleEl) setCookie('searxgo_tracker_remover', trackerToggleEl.checked ? 'true' : 'false', 365);

      const centerToggleEl = document.getElementById('center-align-toggle');
      if (centerToggleEl) setCookie('searxgo_center_alignment', centerToggleEl.checked ? 'true' : 'false', 365);

      const doiSelectEl = document.getElementById('doi-resolver-select');
      if (doiSelectEl) setCookie('searxgo_doi_resolver', doiSelectEl.value, 365);

      // Collect and save default categories
      const checkedCats = [];
      form.querySelectorAll('input[name="default_categories"]:checked').forEach(cb => {
        const val = cb.value.toLowerCase().trim();
        if (val) checkedCats.push(val);
      });
      const catVal = checkedCats.length > 0 ? checkedCats.join(',') : 'general';
      setCookie('searxgo_categories', catVal, 365);
      try { localStorage.setItem('searxgo_categories', catVal); } catch(e){}

      // Instant tactile visual feedback on submit button
      const saveBtn = document.getElementById('btn-save-preferences') || form.querySelector('button[type="submit"]');
      if (saveBtn) {
        const origText = saveBtn.innerHTML;
        saveBtn.innerHTML = '✓ Saved Successfully!';
        saveBtn.style.background = 'linear-gradient(135deg, #10b981, #059669)';
        saveBtn.style.color = '#fff';
        saveBtn.style.boxShadow = '0 0 16px rgba(16, 185, 129, 0.4)';
        setTimeout(() => {
          saveBtn.innerHTML = origText;
          saveBtn.style.background = '';
          saveBtn.style.color = '';
          saveBtn.style.boxShadow = '';
        }, 2500);
      }

      const saveStatus = document.getElementById('save-status-text');
      if (saveStatus) {
        saveStatus.style.display = 'inline';
        setTimeout(() => { saveStatus.style.display = 'none'; }, 2500);
      }

      const msg = document.getElementById('save-feedback');
      if (msg) {
        msg.textContent = '✓ Preferences successfully saved to local browser cookies!';
        msg.style.display = 'block';
        setTimeout(() => { msg.style.display = 'none'; }, 3500);
      }

      // If HTMX is active on this form, let HTMX handle the network submission
      if (form.hasAttribute('hx-post')) {
        return;
      }

      // Fallback for non-HTMX form submission
      e.preventDefault();
      try {
        const formData = new FormData(form);
        const searchParams = new URLSearchParams();
        for (const [key, value] of formData.entries()) {
          searchParams.append(key, value);
        }
        fetch(form.action || '/preferences', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'X-Requested-With': 'XMLHttpRequest'
          },
          body: searchParams.toString()
        }).catch(() => {});
      } catch (err) {}
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

    // Apply Query In Title preference
    const savedQueryInTitle = getCookie('searxgo_query_in_title');
    if (savedQueryInTitle === 'false') {
      document.title = 'SearXGo Search';
    }

    // Reflect default category on search home page category tabs
    const savedCats = getCookie('searxgo_categories');
    if (savedCats) {
      const activeList = savedCats.split(',').map(s => s.trim().toLowerCase()).filter(Boolean);
      if (activeList.length > 0) {
        const primaryCat = activeList[0];
        const categoryInputs = document.querySelectorAll('.category-tabs input[name="category"]');
        if (categoryInputs.length > 0) {
          categoryInputs.forEach(input => {
            const isMatch = input.value.toLowerCase() === primaryCat;
            input.checked = isMatch;
            const parentLabel = input.closest('.category-tab');
            if (parentLabel) {
              if (isMatch) {
                parentLabel.classList.add('active');
              } else {
                parentLabel.classList.remove('active');
              }
            }
          });
        }
      }
    }
  }


