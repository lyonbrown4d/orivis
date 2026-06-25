(function () {
  "use strict";

  const STORAGE_KEY = "orivis.monitor.ui.v1:theme";
  const MODES = { light: true, dark: true, system: true };
  const mediaQuery = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

  function controller() {
    if (window.OrivisTheme && typeof window.OrivisTheme.apply === "function") {
      return window.OrivisTheme;
    }

    return {
      read: readMode,
      apply: function (mode) {
        let root = document.documentElement;
        let nextMode = normalizeTheme(mode);
        let effectiveMode = nextMode === "dark" || (nextMode === "system" && prefersDark()) ? "dark" : "light";
        root.setAttribute("data-orivis-theme", nextMode);
        root.setAttribute("data-orivis-effective-theme", effectiveMode);
        root.classList.toggle("dark", effectiveMode === "dark");
        root.style.colorScheme = effectiveMode;
      },
    };
  }

  function normalizeTheme(mode) {
    return MODES[mode] ? mode : "system";
  }

  function prefersDark() {
    return !!(mediaQuery && mediaQuery.matches);
  }

  function readMode() {
    try {
      let value = window.localStorage ? window.localStorage.getItem(STORAGE_KEY) : "";
      return normalizeTheme(value);
    } catch (_) {
      return "system";
    }
  }

  function writeMode(mode) {
    try {
      if (window.localStorage) {
        window.localStorage.setItem(STORAGE_KEY, normalizeTheme(mode));
      }
    } catch (_) {
      return;
    }
  }

  function currentMode() {
    return normalizeTheme(document.documentElement.getAttribute("data-orivis-theme") || readMode());
  }

  function applyMode(mode, persist) {
    let nextMode = normalizeTheme(mode);
    if (persist) {
      writeMode(nextMode);
    }
    controller().apply(nextMode);
    updateThemeSwitches(document);
  }

  function updateThemeSwitches(scope) {
    let root = scope || document;
    let mode = currentMode();
    let effectiveMode = document.documentElement.getAttribute("data-orivis-effective-theme") || "light";
    let switches = root.querySelectorAll ? root.querySelectorAll("[data-orivis-theme-switch]") : [];

    Array.prototype.forEach.call(switches, function (switcher) {
      switcher.setAttribute("data-orivis-current-theme", mode);
      switcher.setAttribute("data-orivis-effective-theme", effectiveMode);
      let options = switcher.querySelectorAll("[data-orivis-theme-option]");
      Array.prototype.forEach.call(options, function (option) {
        let active = option.getAttribute("data-orivis-theme-option") === mode;
        option.classList.toggle("is-active", active);
        option.setAttribute("aria-pressed", active ? "true" : "false");
      });
    });
  }

  function bindThemeSwitches(scope) {
    let root = scope || document;
    let switches = root.querySelectorAll ? root.querySelectorAll("[data-orivis-theme-switch]") : [];

    Array.prototype.forEach.call(switches, function (switcher) {
      if (switcher.__orivis_theme_bound__) {
        return;
      }
      switcher.__orivis_theme_bound__ = true;
      switcher.addEventListener("click", function (event) {
        let option = event.target && event.target.closest ? event.target.closest("[data-orivis-theme-option]") : null;
        if (!option || !switcher.contains(option)) {
          return;
        }
        applyMode(option.getAttribute("data-orivis-theme-option"), true);
      });
    });

    updateThemeSwitches(root);
  }

  function syncSystemTheme() {
    if (currentMode() === "system") {
      applyMode("system", false);
    }
  }

  if (mediaQuery) {
    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", syncSystemTheme);
    } else if (typeof mediaQuery.addListener === "function") {
      mediaQuery.addListener(syncSystemTheme);
    }
  }

  window.OrivisThemeSwitch = {
    apply: applyMode,
    bind: bindThemeSwitches,
    read: currentMode,
  };

  document.addEventListener("DOMContentLoaded", function () {
    applyMode(readMode(), false);
    bindThemeSwitches(document);
  });

  document.addEventListener("htmx:afterSwap", function (event) {
    let target = event && event.detail && event.detail.target ? event.detail.target : document;
    bindThemeSwitches(target);
    updateThemeSwitches(document);
  });
}());
(function () {
  const STORAGE_PREFIX = "orivis.monitor.ui.";
  const STORAGE_VERSION = "v1";
  const STORAGE_TTL = 1000 * 60 * 60 * 24 * 14;
  const REFRESH_PAUSED_KEY = STORAGE_PREFIX + STORAGE_VERSION + ":refresh-paused";
  const MAX_ANIMATION_CARD_COUNT = 22;
  const SEARCH_DEBOUNCE_MS = 90;
  const DEFAULT_SORT = "checked-desc";
  const FILTER_HINT_CLEAR_DISABLED = "filters already cleared";
  const FILTER_SUMMARY_DEFAULT = "No filters";
  const FILTER_SUMMARY_PREFIX = "Filters:";
  const SEARCH_SHORTCUT_HINT = "Press / to focus search";
  const CARD_INTERACTIVE_SELECTOR = "a, button, input, select, textarea, label, summary, [data-orivis-ignore-card-click]";
  const TOOLTIP_OFFSET = 14;

  const statusFilterLabel = {
    success: "Healthy",
    warning: "Unknown",
    danger: "Down",
    secondary: "Other",
  };

  const statusSortOrder = {
    success: 0,
    warning: 1,
    danger: 2,
    secondary: 3,
  };

  function getStorageKey(pathname) {
    return STORAGE_PREFIX + STORAGE_VERSION + ":" + (pathname || "/");
  }

  function readPersistedState(pathname) {
    let fallback = {
      q: "",
      s: {},
      sort: DEFAULT_SORT,
    };

    let raw = null;
    if (!window.localStorage) {
      return fallback;
    }

    try {
      raw = window.localStorage.getItem(getStorageKey(pathname));
    } catch (_) {
      return fallback;
    }
    if (!raw) {
      return fallback;
    }

    try {
      let parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== "object") {
        return fallback;
      }
      let q = typeof parsed.q === "string" ? parsed.q : "";
      let sort = typeof parsed.sort === "string" ? parsed.sort : DEFAULT_SORT;
      let statusObj = parsed.s;
      let normalized = {};
      if (statusObj && typeof statusObj === "object") {
        Object.keys(statusObj).forEach(function (key) {
          if (statusObj[key]) {
            normalized[key] = true;
          }
        });
      }
      return {
        q: q,
        s: normalized,
        sort: sort,
        at: parsed.at || 0,
      };
    } catch (_) {
      return fallback;
    }
  }

  function writePersistedState(pathname, state) {
    if (!window.localStorage) {
      return;
    }

    try {
      let payload = {
        q: state.q || "",
        s: state.s || {},
        sort: state.sort || DEFAULT_SORT,
        at: Date.now(),
      };
      window.localStorage.setItem(getStorageKey(pathname), JSON.stringify(payload));
    } catch (_) {
      return;
    }
  }

  function readRefreshPaused() {
    if (!window.localStorage) {
      return false;
    }

    try {
      return window.localStorage.getItem(REFRESH_PAUSED_KEY) === "1";
    } catch (_) {
      return false;
    }
  }

  function writeRefreshPaused(paused) {
    if (!window.localStorage) {
      return;
    }

    try {
      window.localStorage.setItem(REFRESH_PAUSED_KEY, paused ? "1" : "0");
    } catch (_) {
      return;
    }
  }

  function normalize(value) {
    return (value || "").toString().toLowerCase();
  }

  function parseIntOrZero(value) {
    let parsed = parseInt(value, 10);
    if (!parsed || parsed < 0) {
      return 0;
    }
    return parsed;
  }

  function hasTextFilter(state) {
    return normalize(state.q || "").length > 0;
  }

  function hasStatusFilter(state) {
    return Object.keys(state.s || {}).length > 0;
  }

  function hasAnyFilter(state) {
    return hasTextFilter(state) || hasStatusFilter(state);
  }

  function parseStatusKey(card) {
    return normalize(card && card.getAttribute ? card.getAttribute("data-monitor-status") : "");
  }

  function collectCards(scope) {
    if (!scope) {
      return [];
    }
    return Array.prototype.slice.call(scope.querySelectorAll("[data-monitor-card]"));
  }

  function setActiveState(element, active) {
    if (!element) {
      return;
    }
    element.classList.toggle("is-active", active);
  }

  function updateChipState(scope, state) {
    let hasFilter = Object.keys(state.s).length > 0;
    let chips = scope.querySelectorAll("[data-status-filter]");
    Array.prototype.forEach.call(chips, function (chip) {
      let value = normalize(chip.getAttribute("data-status-filter"));
      let active = value === "all" ? !hasFilter : state.s[value] === true;
      setActiveState(chip, active);
      chip.setAttribute("aria-pressed", active ? "true" : "false");
    });
  }

  function matchCard(card, state, query) {
    let status = parseStatusKey(card);
    let hasStatusFilters = Object.keys(state.s).length > 0;
    let matchStatus = true;
    if (hasStatusFilters) {
      matchStatus = !!state.s[status];
    }

    let haystack = [
      normalize(card.getAttribute("data-monitor-name")),
      normalize(card.getAttribute("data-monitor-target")),
      normalize(card.getAttribute("data-monitor-group")),
      normalize(card.getAttribute("data-monitor-environment")),
      normalize(card.getAttribute("data-monitor-source")),
    ].join(" ");
    let matchQuery = haystack.indexOf(query) !== -1;
    return matchStatus && matchQuery;
  }

  function statusOrder(value) {
    return Object.prototype.hasOwnProperty.call(statusSortOrder, value) ? statusSortOrder[value] : 99;
  }

  function comparator(sortValue) {
    return function (left, right) {
      let leftStatus = parseStatusKey(left);
      let rightStatus = parseStatusKey(right);
      let leftChecked = parseIntOrZero(left.getAttribute("data-monitor-checked-at"));
      let rightChecked = parseIntOrZero(right.getAttribute("data-monitor-checked-at"));
      let leftLatency = parseIntOrZero(left.getAttribute("data-monitor-latency-ms"));
      let rightLatency = parseIntOrZero(right.getAttribute("data-monitor-latency-ms"));
      let leftName = normalize(left.getAttribute("data-monitor-name"));
      let rightName = normalize(right.getAttribute("data-monitor-name"));

      switch (sortValue) {
        case "name-asc":
          if (leftName < rightName) return -1;
          if (leftName > rightName) return 1;
          break;
        case "name-desc":
          if (leftName < rightName) return 1;
          if (leftName > rightName) return -1;
          break;
        case "latency-asc":
          if (leftLatency < rightLatency) return -1;
          if (leftLatency > rightLatency) return 1;
          break;
        case "latency-desc":
          if (leftLatency < rightLatency) return 1;
          if (leftLatency > rightLatency) return -1;
          break;
        case "checked-asc":
          if (leftChecked < rightChecked) return -1;
          if (leftChecked > rightChecked) return 1;
          break;
        case "checked-desc":
        default:
          if (leftChecked < rightChecked) return 1;
          if (leftChecked > rightChecked) return -1;
          break;
      }

      let statusCompare = statusOrder(leftStatus) - statusOrder(rightStatus);
      if (statusCompare !== 0) {
        return statusCompare;
      }
      if (leftName < rightName) return -1;
      if (leftName > rightName) return 1;
      return 0;
    };
  }

  function setCountText(scope, total, visible) {
    let counter = scope.querySelector("#orivis-monitor-visible-count");
    if (!counter) {
      return;
    }
    counter.textContent = visible + " / " + total;
  }

  function showOrHideEmptyState(scope, root, visibleCount, totalCount) {
    if (!root) {
      return;
    }
    let emptyState = root.querySelector("[data-orivis-monitor-empty-all]");
    let emptyFilterState = root.querySelector("[data-orivis-monitor-empty-filter]");
    if (!emptyState || !emptyFilterState) {
      return;
    }

    if (totalCount === 0) {
      emptyFilterState.classList.add("is-hidden");
      emptyState.classList.remove("is-hidden");
      return;
    }

    if (visibleCount === 0) {
      emptyState.classList.add("is-hidden");
      emptyFilterState.textContent = emptyFilterState.getAttribute("data-orivis-monitor-no-matches") || emptyFilterState.textContent;
      emptyFilterState.classList.remove("is-hidden");
      return;
    }

    emptyFilterState.classList.add("is-hidden");
    emptyState.classList.add("is-hidden");
  }

  function normalizeStatusFilters(state) {
    let names = [];
    if (!state || !state.s) {
      return names;
    }
    Object.keys(state.s).forEach(function (key) {
      if (state.s[key] && key) {
        names.push(key);
      }
    });
    return names.sort();
  }

  function getMonitorRoot(scope) {
    if (!scope) {
      return null;
    }
    return scope.querySelector("[data-orivis-monitor-root]");
  }

  function getRefreshIndicator(scope) {
    if (scope && scope.querySelector) {
      let scopedIndicator = scope.querySelector("#orivis-refresh-indicator");
      if (scopedIndicator) {
        return scopedIndicator;
      }
    }
    return document.querySelector("#orivis-refresh-indicator");
  }

  function getRefreshToggle(scope) {
    if (scope && scope.querySelector) {
      let scopedToggle = scope.querySelector("#orivis-refresh-toggle");
      if (scopedToggle) {
        return scopedToggle;
      }
    }
    return document.querySelector("#orivis-refresh-toggle");
  }

  function setSearchHint(searchInput) {
    if (!searchInput) {
      return;
    }
    if (!searchInput.getAttribute("aria-description")) {
      searchInput.setAttribute("aria-description", SEARCH_SHORTCUT_HINT);
    }
  }

  function enableSearchShortcut(scope) {
    let searchInput = scope.querySelector("#orivis-monitor-search");
    if (!searchInput) {
      return;
    }

    setSearchHint(searchInput);
    let timer;
    document.addEventListener("keydown", function (event) {
      if (event.defaultPrevented) {
        return;
      }
      if ((event.key || "").toLowerCase() !== "/") {
        return;
      }
      if (
        event.target &&
        (event.target.tagName === "INPUT" || event.target.tagName === "TEXTAREA" || event.target.isContentEditable)
      ) {
        return;
      }

      event.preventDefault();
      searchInput.focus({ preventScroll: true });
      if (timer) {
        clearTimeout(timer);
      }
      searchInput.classList.add("is-active");
      timer = setTimeout(function () {
        searchInput.classList.remove("is-active");
      }, 900);
    });
  }

  function setRefreshState(scope, refreshing) {
    if (readRefreshPaused()) {
      updateRefreshPausedUI(scope, true);
      return;
    }

    let indicator = getRefreshIndicator(scope);
    if (!indicator) {
      return;
    }

    let baseText = indicator.getAttribute("data-orivis-refresh-idle") || indicator.textContent || "Refresh";
    if (refreshing) {
      indicator.textContent = baseText + "...";
      indicator.classList.add("is-refreshing");
      return;
    }

    indicator.textContent = baseText;
    indicator.classList.remove("is-refreshing");
  }

  function updateRefreshPausedUI(scope, paused) {
    let indicator = getRefreshIndicator(scope);
    if (indicator) {
      let idleText = indicator.getAttribute("data-orivis-refresh-idle") || indicator.textContent || "Refresh";
      let pausedText = indicator.getAttribute("data-orivis-refresh-paused") || "Refresh paused";
      indicator.textContent = paused ? pausedText : idleText;
      indicator.classList.toggle("is-paused", paused);
      indicator.classList.remove("is-refreshing");
    }

    let toggle = getRefreshToggle(scope);
    if (!toggle) {
      return;
    }
    let pauseText = toggle.getAttribute("data-orivis-refresh-pause") || "Pause refresh";
    let resumeText = toggle.getAttribute("data-orivis-refresh-resume") || "Resume refresh";
    toggle.textContent = paused ? resumeText : pauseText;
    toggle.setAttribute("aria-pressed", paused ? "true" : "false");
    toggle.classList.toggle("is-paused", paused);
  }

  function bindRefreshToggle(scope) {
    let toggle = getRefreshToggle(scope);
    if (!toggle || toggle.__orivis_refresh_bound__) {
      updateRefreshPausedUI(scope, readRefreshPaused());
      return;
    }

    toggle.__orivis_refresh_bound__ = true;
    updateRefreshPausedUI(scope, readRefreshPaused());
    toggle.addEventListener("click", function () {
      let next = !readRefreshPaused();
      writeRefreshPaused(next);
      updateRefreshPausedUI(scope, next);
    });
  }

  function animateValueFromZero(node, targetText) {
    if (!node || typeof targetText !== "string") {
      return;
    }
    let target = parseInt(targetText.replace(/[^\d-]/g, ""), 10);
    if (!target && target !== 0) {
      return;
    }

    let from = 0;
    let start = null;
    let duration = 420;
    let durationMs = Math.min(Math.max(Math.abs(target), 0), 1600);
    function tick(timestamp) {
      if (!start) {
        start = timestamp;
      }
      let ratio = Math.min(1, (timestamp - start) / (duration + durationMs));
      let current = Math.round(from + (target - from) * ratio);
      node.textContent = String(current);
      if (ratio < 1) {
        requestAnimationFrame(tick);
        return;
      }
      node.textContent = targetText;
    }

    requestAnimationFrame(tick);
  }

  function animateMonitorCards(cards) {
    let i = 0;
    let total = cards && cards.length ? cards.length : 0;
    if (total > MAX_ANIMATION_CARD_COUNT) {
      total = MAX_ANIMATION_CARD_COUNT;
    }

    for (i = 0; i < cards.length; i++) {
      cards[i].classList.remove("orivis-monitor-card-enter");
      cards[i].style.animationDelay = "";
    }
    for (i = 0; i < total; i++) {
      cards[i].style.animationDelay = i * 28 + "ms";
      cards[i].classList.add("orivis-monitor-card-enter");
    }
  }

  function isInteractiveTarget(target, card) {
    let node = target;
    while (node && node !== card) {
      if (node.matches && node.matches(CARD_INTERACTIVE_SELECTOR)) {
        return true;
      }
      node = node.parentNode;
    }
    return false;
  }

  function navigateMonitorCard(card, event, newTab) {
    if (!card) {
      return;
    }
    let url = card.getAttribute("data-monitor-url");
    if (!url) {
      return;
    }
    if (event) {
      event.preventDefault();
    }
    if (newTab) {
      window.open(url, "_blank", "noopener");
      return;
    }
    window.location.href = url;
  }

  function bindMonitorCardNavigation(scope) {
    let cards = scope.querySelectorAll("[data-monitor-card][data-monitor-url]");
    Array.prototype.forEach.call(cards, function (card) {
      if (card.__orivis_card_nav_bound__) {
        return;
      }
      card.__orivis_card_nav_bound__ = true;
      card.addEventListener("click", function (event) {
        if (isInteractiveTarget(event.target, card)) {
          return;
        }
        navigateMonitorCard(card, event, event.metaKey || event.ctrlKey);
      });
      card.addEventListener("auxclick", function (event) {
        if (event.button !== 1 || isInteractiveTarget(event.target, card)) {
          return;
        }
        navigateMonitorCard(card, event, true);
      });
      card.addEventListener("keydown", function (event) {
        if (event.defaultPrevented || isInteractiveTarget(event.target, card)) {
          return;
        }
        if (event.key === "Enter" || event.key === " ") {
          navigateMonitorCard(card, event, false);
        }
      });
    });
  }

  function tooltipNode() {
    let existing = document.querySelector("[data-orivis-floating-tooltip]");
    if (existing) {
      return existing;
    }
    let node = document.createElement("div");
    node.className = "orivis-floating-tooltip";
    node.setAttribute("data-orivis-floating-tooltip", "");
    node.setAttribute("role", "tooltip");
    node.setAttribute("aria-hidden", "true");
    document.body.appendChild(node);
    return node;
  }

  function setTooltipPosition(node, x, y) {
    let left = x + TOOLTIP_OFFSET;
    let top = y - TOOLTIP_OFFSET;
    let rect = node.getBoundingClientRect();
    let maxLeft = window.innerWidth - rect.width - 12;
    let maxTop = window.innerHeight - rect.height - 12;
    node.style.left = Math.max(12, Math.min(left, maxLeft)) + "px";
    node.style.top = Math.max(12, Math.min(top, maxTop)) + "px";
  }

  function showTooltip(target, event) {
    let text = target.getAttribute("data-orivis-tooltip") || target.getAttribute("title") || "";
    if (!text) {
      return;
    }
    let node = tooltipNode();
    node.textContent = text;
    node.classList.add("is-visible");
    node.setAttribute("aria-hidden", "false");
    setTooltipPosition(node, event.clientX || 20, event.clientY || 20);
  }

  function hideTooltip() {
    let node = document.querySelector("[data-orivis-floating-tooltip]");
    if (!node) {
      return;
    }
    node.classList.remove("is-visible");
    node.setAttribute("aria-hidden", "true");
  }

  function bindStatusLightTooltips(scope) {
    let lights = scope.querySelectorAll(".orivis-status-light");
    Array.prototype.forEach.call(lights, function (light) {
      if (light.__orivis_tooltip_bound__) {
        return;
      }
      let title = light.getAttribute("title");
      if (title) {
        light.setAttribute("data-orivis-tooltip", title);
        light.setAttribute("aria-label", title);
        light.removeAttribute("title");
      }
      if (!light.getAttribute("data-orivis-tooltip")) {
        return;
      }
      light.__orivis_tooltip_bound__ = true;
      if (!light.hasAttribute("tabindex")) {
        light.setAttribute("tabindex", "0");
      }
      light.addEventListener("pointerenter", function (event) {
        showTooltip(light, event);
      });
      light.addEventListener("pointermove", function (event) {
        let node = tooltipNode();
        if (node.classList.contains("is-visible")) {
          setTooltipPosition(node, event.clientX, event.clientY);
        }
      });
      light.addEventListener("pointerleave", hideTooltip);
      light.addEventListener("focus", function () {
        let rect = light.getBoundingClientRect();
        showTooltip(light, { clientX: rect.left + rect.width / 2, clientY: rect.top });
      });
      light.addEventListener("blur", hideTooltip);
    });
  }

  function isMainScope(target) {
    return !!(target && target.matches && target.matches("main"));
  }

  function updateFilterSummary(scope, state, visibleCount, totalCount) {
    if (!scope) {
      return;
    }

    let summary = scope.querySelector("#orivis-monitor-filter-summary");
    if (!summary) {
      return;
    }

    let query = normalize(state.q || "");
    let filters = [];
    if (query.length > 0) {
      filters.push("Keyword: " + state.q);
    }

    let selectedStatuses = normalizeStatusFilters(state);
    if (selectedStatuses.length > 0) {
      let labeled = [];
      for (let i = 0; i < selectedStatuses.length; i++) {
        labeled.push(statusFilterLabel[selectedStatuses[i]] || selectedStatuses[i]);
      }
      filters.push("Status: " + labeled.join(", "));
    }

    if (!filters.length) {
      summary.textContent = FILTER_SUMMARY_DEFAULT;
      if (totalCount <= 0) {
        summary.textContent = FILTER_SUMMARY_DEFAULT + " · no data";
      }
      summary.className = "text-slate-500 orivis-filter-summary";
      return;
    }

    summary.textContent = FILTER_SUMMARY_PREFIX + " " + filters.join(" · ");
    summary.className = "text-slate-600 orivis-filter-summary";
  }

  function applyMonitorFilters(scope, state, animate) {
    if (!scope) {
      return;
    }

    let root = getMonitorRoot(scope);
    if (!root) {
      return;
    }

    let grid = root.querySelector(".orivis-monitor-grid");
    if (!grid) {
      return;
    }

    let cards = collectCards(grid);
    if (cards.length === 0) {
      setCountText(scope, 0, 0);
      showOrHideEmptyState(scope, root, 0, 0);
      return;
    }

    let query = normalize(state.q);
    let visibleCards = [];
    let hiddenCards = [];
    for (let i = 0; i < cards.length; i++) {
      let card = cards[i];
      if (matchCard(card, state, query)) {
        card.classList.remove("is-hidden");
        visibleCards.push(card);
      } else {
        card.classList.add("is-hidden");
        hiddenCards.push(card);
      }
    }

    visibleCards.sort(comparator(state.sort || DEFAULT_SORT));
    for (i = 0; i < visibleCards.length; i++) {
      grid.appendChild(visibleCards[i]);
    }
    for (i = 0; i < hiddenCards.length; i++) {
      grid.appendChild(hiddenCards[i]);
    }

    let total = Number(root.getAttribute("data-monitor-total") || cards.length || "0");
    setCountText(scope, total, visibleCards.length);

    updateChipState(scope, state);
    showOrHideEmptyState(scope, root, visibleCards.length, total);
    refreshFilterControls(scope, state);
    updateFilterSummary(scope, state, visibleCards.length, total);
    animateValueFromZero(
      scope.querySelector("#orivis-monitor-visible-count"),
      visibleCards.length + " / " + total
    );
    if (animate) {
      animateMonitorCards(visibleCards);
    }
  }

  function resetFilterUI(scope, state) {
    state.q = "";
    state.s = {};
    let searchInput = scope.querySelector("#orivis-monitor-search");
    if (searchInput) {
      searchInput.value = "";
    }

    let sortSelect = scope.querySelector("#orivis-monitor-sort");
    if (sortSelect) {
      sortSelect.value = DEFAULT_SORT;
      state.sort = DEFAULT_SORT;
    }
  }

  function refreshFilterControls(scope, state) {
    if (!scope) {
      return;
    }

    let clearButton = scope.querySelector("#orivis-clear-filters");
    if (!clearButton) {
      return;
    }

    if (!hasAnyFilter(state)) {
      clearButton.disabled = true;
      clearButton.setAttribute("title", FILTER_HINT_CLEAR_DISABLED);
      clearButton.classList.add("is-disabled");
      return;
    }

    clearButton.disabled = false;
    clearButton.removeAttribute("title");
    clearButton.classList.remove("is-disabled");
  }

  function bind(scope) {
    if (!scope) {
      return;
    }

    if (scope.__orivis_monitor_bound__) {
      return;
    }
    scope.__orivis_monitor_bound__ = true;

    let root = scope.querySelector("[data-orivis-monitor-root]");
    if (!root) {
      return;
    }

    let state = readPersistedState(window.location.pathname);
    if (!state.at || Date.now() - Number(state.at || 0) > STORAGE_TTL) {
      state = {
        q: "",
        s: {},
        sort: DEFAULT_SORT,
      };
    }

    let chips = scope.querySelectorAll("[data-status-filter]");
    let searchInput = scope.querySelector("#orivis-monitor-search");
    let sortSelect = scope.querySelector("#orivis-monitor-sort");
    let clearButton = scope.querySelector("#orivis-clear-filters");
    let filterTimer = 0;

    function scheduleApply(animate) {
      if (filterTimer) {
        clearTimeout(filterTimer);
      }
      filterTimer = setTimeout(function () {
        applyMonitorFilters(scope, state, animate);
      }, SEARCH_DEBOUNCE_MS);
    }

    if (searchInput) {
      searchInput.value = state.q || "";
      searchInput.addEventListener("input", function () {
        state.q = this.value || "";
        scheduleApply(false);
        writePersistedState(window.location.pathname, state);
      });
      searchInput.addEventListener("keydown", function (event) {
        if (event.key === "Escape") {
          if (filterTimer) {
            clearTimeout(filterTimer);
          }
          state.q = "";
          searchInput.value = "";
          applyMonitorFilters(scope, state, false);
          writePersistedState(window.location.pathname, state);
        }
      });
    }

    Array.prototype.forEach.call(chips, function (chip) {
      chip.addEventListener("click", function () {
        let value = normalize(chip.getAttribute("data-status-filter") || "all");
        if (value === "all") {
          state.s = {};
        } else {
          if (state.s[value]) {
            delete state.s[value];
          } else {
            state.s[value] = true;
          }
        }
        applyMonitorFilters(scope, state, false);
        writePersistedState(window.location.pathname, state);
      });
    });

    if (sortSelect) {
      sortSelect.value = state.sort || DEFAULT_SORT;
      sortSelect.addEventListener("change", function () {
        state.sort = sortSelect.value || DEFAULT_SORT;
        applyMonitorFilters(scope, state, false);
        writePersistedState(window.location.pathname, state);
      });
    }

    enableSearchShortcut(scope);
    bindRefreshToggle(scope);
    bindMonitorCardNavigation(scope);
    bindStatusLightTooltips(scope);

    if (clearButton) {
      clearButton.disabled = !hasAnyFilter(state);
      clearButton.addEventListener("click", function () {
        resetFilterUI(scope, state);
        applyMonitorFilters(scope, state, false);
        writePersistedState(window.location.pathname, state);
      });
    }

    applyMonitorFilters(scope, state, true);
  }

  function ensureBound(scope) {
    if (!scope || scope === document) {
      bind(document);
      return;
    }
    bind(scope);
    bind(document);
  }

  document.addEventListener("DOMContentLoaded", function () {
    ensureBound(document);
  });
  document.addEventListener("htmx:afterSwap", function (event) {
    let target = event && event.detail && event.detail.target ? event.detail.target : null;
    if (!target) {
      return;
    }
    if (target.matches && (target.matches("main") || target.matches("[data-orivis-monitor-root]") || target.querySelector && target.querySelector("[data-orivis-monitor-root]"))) {
      ensureBound(target);
      setRefreshState(target, false);
    }
  });
  document.addEventListener("htmx:beforeRequest", function (event) {
    if (!event || !event.detail || !event.detail.target || !isMainScope(event.detail.target)) {
      return;
    }
    if (readRefreshPaused()) {
      event.preventDefault();
      updateRefreshPausedUI(event.detail.target, true);
      return;
    }
    setRefreshState(event.detail.target, true);
  });
  document.addEventListener("htmx:afterRequest", function (event) {
    if (!event || !event.detail || !event.detail.target || !isMainScope(event.detail.target)) {
      return;
    }
    setRefreshState(event.detail.target, false);
  });
  document.addEventListener("htmx:afterSettle", function (event) {
    let target = event && event.detail && event.detail.target ? event.detail.target : null;
    if (!target) {
      return;
    }
    if (target.matches && target.matches("main")) {
      ensureBound(target);
    }
  });

  document.addEventListener("htmx:responseError", function (event) {
    let detail = event.detail || {};
    let target = detail.target;
    if (!target) {
      return;
    }
    setRefreshState(target, false);

    let status = detail.xhr && detail.xhr.status ? " (" + detail.xhr.status + ")" : "";
    let alert = document.createElement("div");
    alert.className =
      "orivis-hx-error rounded-xl px-4 py-3 text-center text-sm font-medium shadow-sm";
    alert.setAttribute("role", "alert");
    alert.textContent = "Unable to refresh this section" + status + ".";

    while (target.firstChild) {
      target.removeChild(target.firstChild);
    }
    target.appendChild(alert);
  });
})();

(function () {
  "use strict";

  function bindPasswordToggles(scope) {
    let root = scope || document;
    let toggles = root.querySelectorAll("[data-orivis-password-toggle]");

    toggles.forEach(function (toggle) {
      if (toggle.dataset.orivisBound === "true") {
        return;
      }

      toggle.dataset.orivisBound = "true";
      toggle.addEventListener("click", function () {
        let field = toggle.closest(".orivis-password-field");
        let input = field ? field.querySelector("input") : null;

        if (!input) {
          return;
        }

        let isHidden = input.type === "password";
        let label = isHidden ? toggle.dataset.orivisPasswordHide : toggle.dataset.orivisPasswordShow;

        input.type = isHidden ? "text" : "password";
        toggle.textContent = label;
        toggle.setAttribute("aria-label", label);
        toggle.setAttribute("aria-pressed", isHidden ? "true" : "false");
      });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      bindPasswordToggles(document);
    });
    return;
  }

  bindPasswordToggles(document);
}());


(function () {
  "use strict";

  const POINTER_SELECTOR = ".orivis-command-center, .orivis-public-header, .orivis-control-panel, .orivis-panel, .orivis-monitor-card, .orivis-metric-card";

  function bindPointerLight(scope) {
    let root = scope || document;
    let nodes = root.querySelectorAll ? root.querySelectorAll(POINTER_SELECTOR) : [];
    Array.prototype.forEach.call(nodes, function (node) {
      if (node.__orivis_pointer_light_bound__) {
        return;
      }
      node.__orivis_pointer_light_bound__ = true;
      node.addEventListener("pointermove", function (event) {
        let rect = node.getBoundingClientRect();
        if (!rect.width || !rect.height) {
          return;
        }
        let x = ((event.clientX - rect.left) / rect.width) * 100;
        let y = ((event.clientY - rect.top) / rect.height) * 100;
        node.style.setProperty("--orivis-pointer-x", x.toFixed(2) + "%");
        node.style.setProperty("--orivis-pointer-y", y.toFixed(2) + "%");
      });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () {
      bindPointerLight(document);
    });
  } else {
    bindPointerLight(document);
  }

  document.addEventListener("htmx:afterSwap", function (event) {
    let target = event && event.detail && event.detail.target ? event.detail.target : document;
    bindPointerLight(target);
  });
}());

(() => {
  "use strict";

  const PRESSABLE_SELECTOR = "button, a, [role='button'], [data-monitor-card]";
  const PRESSING_CLASS = "is-pressing";
  const THEME_MODES = ["system", "light", "dark"];

  const pressableNodes = scope => Array.from((scope || document).querySelectorAll(PRESSABLE_SELECTOR));
  const normalizeThemeMode = mode => THEME_MODES.includes(mode) ? mode : "system";

  const clearPressing = element => {
    element.classList.remove(PRESSING_CLASS);
  };

  const setPressing = element => {
    element.classList.add(PRESSING_CLASS);
  };

  const bindPressStates = scope => {
    for (const element of pressableNodes(scope)) {
      if (element.dataset.orivisPressBound === "true") {
        continue;
      }
      element.dataset.orivisPressBound = "true";
      element.addEventListener("pointerdown", () => setPressing(element));
      element.addEventListener("pointerup", () => clearPressing(element));
      element.addEventListener("pointercancel", () => clearPressing(element));
      element.addEventListener("pointerleave", () => clearPressing(element));
      element.addEventListener("keydown", event => {
        if (event.key === "Enter" || event.key === " ") {
          setPressing(element);
        }
      });
      element.addEventListener("keyup", () => clearPressing(element));
      element.addEventListener("blur", () => clearPressing(element));
    }
  };

  const cycleThemeMode = () => {
    const root = document.documentElement;
    const current = normalizeThemeMode(root.getAttribute("data-orivis-theme"));
    const next = THEME_MODES[(THEME_MODES.indexOf(current) + 1) % THEME_MODES.length];
    const control = document.querySelector(`[data-orivis-theme-option="${next}"]`);
    control?.click();
  };

  const bindThemeShortcut = () => {
    if (document.documentElement.dataset.orivisThemeShortcutBound === "true") {
      return;
    }
    document.documentElement.dataset.orivisThemeShortcutBound = "true";
    document.addEventListener("keydown", event => {
      if (event.defaultPrevented || !event.altKey || !event.shiftKey || event.key.toLowerCase() !== "t") {
        return;
      }
      event.preventDefault();
      cycleThemeMode();
    });
  };

  const bindInteractions = scope => {
    bindPressStates(scope || document);
    bindThemeShortcut();
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => bindInteractions(document));
  } else {
    bindInteractions(document);
  }

  document.addEventListener("htmx:afterSwap", event => {
    bindInteractions(event.detail?.target || document);
  });
})();
