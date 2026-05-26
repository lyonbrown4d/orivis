(function () {
  var STORAGE_PREFIX = "orivis.monitor.ui.";
  var STORAGE_VERSION = "v1";
  var STORAGE_TTL = 1000 * 60 * 60 * 24 * 14;
  var MAX_ANIMATION_CARD_COUNT = 22;
  var DEFAULT_SORT = "checked-desc";
  var FILTER_HINT_CLEAR_DISABLED = "filters already cleared";
  var FILTER_SUMMARY_DEFAULT = "No filters";
  var FILTER_SUMMARY_PREFIX = "Filters:";
  var SEARCH_SHORTCUT_HINT = "Press / to focus search";

  var statusFilterLabel = {
    success: "Healthy",
    warning: "Unknown",
    danger: "Down",
    secondary: "Other",
  };

  var statusSortOrder = {
    success: 0,
    warning: 1,
    danger: 2,
    secondary: 3,
  };

  function getStorageKey(pathname) {
    return STORAGE_PREFIX + STORAGE_VERSION + ":" + (pathname || "/");
  }

  function readPersistedState(pathname) {
    var fallback = {
      q: "",
      s: {},
      sort: DEFAULT_SORT,
    };

    var raw = null;
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
      var parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== "object") {
        return fallback;
      }
      var q = typeof parsed.q === "string" ? parsed.q : "";
      var sort = typeof parsed.sort === "string" ? parsed.sort : DEFAULT_SORT;
      var statusObj = parsed.s;
      var normalized = {};
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
      var payload = {
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

  function normalize(value) {
    return (value || "").toString().toLowerCase();
  }

  function parseIntOrZero(value) {
    var parsed = parseInt(value, 10);
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
    var hasFilter = Object.keys(state.s).length > 0;
    var chips = scope.querySelectorAll("[data-status-filter]");
    Array.prototype.forEach.call(chips, function (chip) {
      var value = normalize(chip.getAttribute("data-status-filter"));
      var active = value === "all" ? !hasFilter : state.s[value] === true;
      setActiveState(chip, active);
      chip.setAttribute("aria-pressed", active ? "true" : "false");
    });
  }

  function matchCard(card, state, query) {
    var status = parseStatusKey(card);
    var hasStatusFilters = Object.keys(state.s).length > 0;
    var matchStatus = true;
    if (hasStatusFilters) {
      matchStatus = !!state.s[status];
    }

    var haystack = [
      normalize(card.getAttribute("data-monitor-name")),
      normalize(card.getAttribute("data-monitor-target")),
      normalize(card.getAttribute("data-monitor-group")),
      normalize(card.getAttribute("data-monitor-environment")),
      normalize(card.getAttribute("data-monitor-source")),
    ].join(" ");
    var matchQuery = haystack.indexOf(query) !== -1;
    return matchStatus && matchQuery;
  }

  function statusOrder(value) {
    return Object.prototype.hasOwnProperty.call(statusSortOrder, value) ? statusSortOrder[value] : 99;
  }

  function comparator(sortValue) {
    return function (left, right) {
      var leftStatus = parseStatusKey(left);
      var rightStatus = parseStatusKey(right);
      var leftChecked = parseIntOrZero(left.getAttribute("data-monitor-checked-at"));
      var rightChecked = parseIntOrZero(right.getAttribute("data-monitor-checked-at"));
      var leftLatency = parseIntOrZero(left.getAttribute("data-monitor-latency-ms"));
      var rightLatency = parseIntOrZero(right.getAttribute("data-monitor-latency-ms"));
      var leftName = normalize(left.getAttribute("data-monitor-name"));
      var rightName = normalize(right.getAttribute("data-monitor-name"));

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

      var statusCompare = statusOrder(leftStatus) - statusOrder(rightStatus);
      if (statusCompare !== 0) {
        return statusCompare;
      }
      if (leftName < rightName) return -1;
      if (leftName > rightName) return 1;
      return 0;
    };
  }

  function setCountText(scope, total, visible) {
    var counter = scope.querySelector("#orivis-monitor-visible-count");
    if (!counter) {
      return;
    }
    counter.textContent = visible + " / " + total;
  }

  function showOrHideEmptyState(scope, root, visibleCount, totalCount) {
    if (!root) {
      return;
    }
    var emptyState = root.querySelector("[data-orivis-monitor-empty-all]");
    var emptyFilterState = root.querySelector("[data-orivis-monitor-empty-filter]");
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
    var names = [];
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
      var scopedIndicator = scope.querySelector("#orivis-refresh-indicator");
      if (scopedIndicator) {
        return scopedIndicator;
      }
    }
    return document.querySelector("#orivis-refresh-indicator");
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
    var searchInput = scope.querySelector("#orivis-monitor-search");
    if (!searchInput) {
      return;
    }

    setSearchHint(searchInput);
    var timer;
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
    var indicator = getRefreshIndicator(scope);
    if (!indicator) {
      return;
    }

    var baseText = indicator.getAttribute("data-orivis-refresh-idle") || indicator.textContent || "Refresh";
    if (refreshing) {
      indicator.textContent = baseText + "...";
      indicator.classList.add("is-refreshing");
      return;
    }

    indicator.textContent = baseText;
    indicator.classList.remove("is-refreshing");
  }

  function animateValueFromZero(node, targetText) {
    if (!node || typeof targetText !== "string") {
      return;
    }
    var target = parseInt(targetText.replace(/[^\d-]/g, ""), 10);
    if (!target && target !== 0) {
      return;
    }

    var from = 0;
    var start = null;
    var duration = 420;
    var durationMs = Math.min(Math.max(Math.abs(target), 0), 1600);
    function tick(timestamp) {
      if (!start) {
        start = timestamp;
      }
      var ratio = Math.min(1, (timestamp - start) / (duration + durationMs));
      var current = Math.round(from + (target - from) * ratio);
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
    var i = 0;
    var total = cards && cards.length ? cards.length : 0;
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

  function isMainScope(target) {
    return !!(target && target.matches && target.matches("main"));
  }

  function updateFilterSummary(scope, state, visibleCount, totalCount) {
    if (!scope) {
      return;
    }

    var summary = scope.querySelector("#orivis-monitor-filter-summary");
    if (!summary) {
      return;
    }

    var query = normalize(state.q || "");
    var filters = [];
    if (query.length > 0) {
      filters.push("Keyword: " + state.q);
    }

    var selectedStatuses = normalizeStatusFilters(state);
    if (selectedStatuses.length > 0) {
      var labeled = [];
      for (var i = 0; i < selectedStatuses.length; i++) {
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

    var root = getMonitorRoot(scope);
    if (!root) {
      return;
    }

    var grid = root.querySelector(".orivis-monitor-grid");
    if (!grid) {
      return;
    }

    var cards = collectCards(grid);
    if (cards.length === 0) {
      setCountText(scope, 0, 0);
      showOrHideEmptyState(scope, root, 0, 0);
      return;
    }

    var query = normalize(state.q);
    var visibleCards = [];
    var hiddenCards = [];
    for (var i = 0; i < cards.length; i++) {
      var card = cards[i];
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

    var total = Number(root.getAttribute("data-monitor-total") || cards.length || "0");
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
    var searchInput = scope.querySelector("#orivis-monitor-search");
    if (searchInput) {
      searchInput.value = "";
    }

    var sortSelect = scope.querySelector("#orivis-monitor-sort");
    if (sortSelect) {
      sortSelect.value = DEFAULT_SORT;
      state.sort = DEFAULT_SORT;
    }
  }

  function refreshFilterControls(scope, state) {
    if (!scope) {
      return;
    }

    var clearButton = scope.querySelector("#orivis-clear-filters");
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

    var root = scope.querySelector("[data-orivis-monitor-root]");
    if (!root) {
      return;
    }

    var state = readPersistedState(window.location.pathname);
    if (!state.at || Date.now() - Number(state.at || 0) > STORAGE_TTL) {
      state = {
        q: "",
        s: {},
        sort: DEFAULT_SORT,
      };
    }

    var chips = scope.querySelectorAll("[data-status-filter]");
    var searchInput = scope.querySelector("#orivis-monitor-search");
    var sortSelect = scope.querySelector("#orivis-monitor-sort");
    var clearButton = scope.querySelector("#orivis-clear-filters");

    if (searchInput) {
      searchInput.value = state.q || "";
      searchInput.addEventListener("input", function () {
        state.q = this.value || "";
        applyMonitorFilters(scope, state, false);
        writePersistedState(window.location.pathname, state);
      });
      searchInput.addEventListener("keydown", function (event) {
        if (event.key === "Escape") {
          state.q = "";
          searchInput.value = "";
          applyMonitorFilters(scope, state, false);
          writePersistedState(window.location.pathname, state);
        }
      });
    }

    Array.prototype.forEach.call(chips, function (chip) {
      chip.addEventListener("click", function () {
        var value = normalize(chip.getAttribute("data-status-filter") || "all");
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
    var target = event && event.detail && event.detail.target ? event.detail.target : null;
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
    setRefreshState(event.detail.target, true);
  });
  document.addEventListener("htmx:afterRequest", function (event) {
    if (!event || !event.detail || !event.detail.target || !isMainScope(event.detail.target)) {
      return;
    }
    setRefreshState(event.detail.target, false);
  });
  document.addEventListener("htmx:afterSettle", function (event) {
    var target = event && event.detail && event.detail.target ? event.detail.target : null;
    if (!target) {
      return;
    }
    if (target.matches && target.matches("main")) {
      ensureBound(target);
    }
  });

  document.addEventListener("htmx:responseError", function (event) {
    var detail = event.detail || {};
    var target = detail.target;
    if (!target) {
      return;
    }
    setRefreshState(target, false);

    var status = detail.xhr && detail.xhr.status ? " (" + detail.xhr.status + ")" : "";
    var alert = document.createElement("div");
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
