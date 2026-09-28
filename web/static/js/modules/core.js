/**
 * SearXGo Module: Core Orchestrator & HTMX Lifecycle Hooks
 */
  // Safe DOM ready initialization
  function initAll() {
    initTheme();
    initServiceWorker();
    initUserPreferencesOnLoad();
    initCategoryTabs();
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

  // Expose global namespace for debugging and extensions
  window.SearXGo = {
    initAll,
    applyGoggles,
    getGoggles,
    initTheme,
    setTheme: window.setTheme,
    toggleTheme: window.toggleTheme,
    showToast,
    showShortcutsModal
  };

  // Safe initial execution
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      initAll();
      applyGoggles();
    });
  } else {
    initAll();
    applyGoggles();
  }

  // HTMX Lifecycle Hooks: Ensure full interactivity and goggles filtering on boosted transitions
  document.addEventListener('htmx:load', function () {
    initAll();
    applyGoggles();
  });

  document.addEventListener('htmx:historyRestore', function () {
    initAll();
    applyGoggles();
  });
