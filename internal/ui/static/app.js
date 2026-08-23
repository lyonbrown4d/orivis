(() => {
  "use strict";

  const doc = document;
  const root = doc.documentElement;
  const media = window.matchMedia?.("(prefers-color-scheme: dark)");
  const themeModes = ["system", "light", "dark"];
  const statePrefix = "orivis.monitor.ui.v1:";
  const refreshKey = statePrefix + "refresh-paused";
  const stateTTL = 14 * 24 * 60 * 60 * 1000;
  const defaultSort = "checked-desc";
  const validStatuses = new Set(["success", "danger", "warning", "secondary"]);
  const validSorts = new Set(["checked-desc", "checked-asc", "name-asc", "name-desc", "latency-desc", "latency-asc"]);
  const interactiveSelector = "a,button,input,select,textarea,label,summary,[data-orivis-ignore-card-click]";
  let filterTimer = 0;
  let swapFocus = null;

  const storageGet = key => {
    try { return window.localStorage?.getItem(key) || ""; } catch (_) { return ""; }
  };
  const storageSet = (key, value) => {
    try { window.localStorage?.setItem(key, value); } catch (_) { /* Optional storage. */ }
  };
  const normalize = value => String(value || "").toLocaleLowerCase();
  const numberValue = value => Math.max(0, Number.parseInt(value, 10) || 0);
  const currentMain = scope => scope?.matches?.("main") ? scope : scope?.querySelector?.("main") || doc.querySelector("main");
  const isAutoMain = target => Boolean(target?.matches?.("main[data-orivis-auto-refresh]"));
  const mainFromDetail = detail => {
    if (isAutoMain(detail?.target)) return detail.target;
    if (isAutoMain(detail?.elt)) return detail.elt;
    return detail?.elt?.closest?.("main[data-orivis-auto-refresh]") || null;
  };
  const refreshPaused = () => storageGet(refreshKey) === "1";
  const stateKey = () => statePrefix + (window.location.pathname || "/");

  const readState = () => {
    const fallback = { q: "", statuses: new Set(), sort: defaultSort };
    try {
      const parsed = JSON.parse(storageGet(stateKey()));
      if (!parsed || Date.now() - numberValue(parsed.at) > stateTTL) return fallback;
      const statuses = new Set(Array.isArray(parsed.statuses) ? parsed.statuses.filter(value => validStatuses.has(value)) : []);
      return { q: typeof parsed.q === "string" ? parsed.q : "", statuses, sort: validSorts.has(parsed.sort) ? parsed.sort : defaultSort };
    } catch (_) {
      return fallback;
    }
  };
  const writeState = state => storageSet(stateKey(), JSON.stringify({ q: state.q, statuses: [...state.statuses], sort: state.sort, at: Date.now() }));

  const applyTheme = (mode, persist = false) => {
    const next = themeModes.includes(mode) ? mode : "system";
    if (persist) storageSet(window.OrivisTheme?.key || statePrefix + "theme", next);
    window.OrivisTheme?.apply(next);
    doc.querySelectorAll("[data-orivis-theme-switch]").forEach(switcher => {
      switcher.dataset.orivisCurrentTheme = next;
      switcher.querySelectorAll("[data-orivis-theme-option]").forEach(option => {
        const active = option.dataset.orivisThemeOption === next;
        option.classList.toggle("is-active", active);
        option.setAttribute("aria-pressed", String(active));
      });
    });
  };

  const monitorState = main => {
    if (!main) return null;
    if (!main.__orivisMonitorState) main.__orivisMonitorState = readState();
    return main.__orivisMonitorState;
  };
  const cardsFor = main => [...(main?.querySelectorAll("[data-monitor-card]") || [])];
  const compareCards = sort => (left, right) => {
    const aName = normalize(left.dataset.monitorName);
    const bName = normalize(right.dataset.monitorName);
    const aChecked = numberValue(left.dataset.monitorCheckedAt);
    const bChecked = numberValue(right.dataset.monitorCheckedAt);
    const aLatency = numberValue(left.dataset.monitorLatencyMs);
    const bLatency = numberValue(right.dataset.monitorLatencyMs);
    if (sort === "name-asc" || sort === "name-desc") return (aName.localeCompare(bName) || aChecked - bChecked) * (sort === "name-desc" ? -1 : 1);
    if (sort === "latency-asc" || sort === "latency-desc") return (aLatency - bLatency || aName.localeCompare(bName)) * (sort === "latency-desc" ? -1 : 1);
    return (aChecked - bChecked || aName.localeCompare(bName)) * (sort === "checked-desc" ? -1 : 1);
  };
  const matchesCard = (card, state) => {
    const query = normalize(state.q);
    const statusMatch = state.statuses.size === 0 || state.statuses.has(normalize(card.dataset.monitorStatus));
    const text = [card.dataset.monitorName, card.dataset.monitorTarget, card.dataset.monitorGroup, card.dataset.monitorEnvironment, card.dataset.monitorSource].map(normalize).join(" ");
    return statusMatch && text.includes(query);
  };

  const applyFilters = (main, persist = false) => {
    const state = monitorState(main);
    const monitorRoot = main?.querySelector("[data-orivis-monitor-root]");
    const grid = monitorRoot?.querySelector(".orivis-monitor-grid");
    if (!state || !monitorRoot || !grid) return;
    const cards = cardsFor(main);
    const visible = [];
    const hidden = [];
    cards.forEach(card => (matchesCard(card, state) ? visible : hidden).push(card));
    visible.sort(compareCards(state.sort));
    const hiddenSet = new Set(hidden);
    [...visible, ...hidden].forEach(card => {
      card.classList.toggle("is-hidden", hiddenSet.has(card));
      grid.append(card);
    });
    const total = numberValue(monitorRoot.dataset.monitorTotal || cards.length);
    const counter = main.querySelector("#orivis-monitor-visible-count");
    const summary = main.querySelector("#orivis-monitor-filter-summary");
    if (counter) counter.textContent = visible.length + " / " + total;
    if (summary) summary.textContent = (visible.length + " / " + total + " " + (monitorRoot.dataset.orivisMonitorLabel || "")).trim();
    main.querySelectorAll("[data-status-filter]").forEach(chip => {
      const value = chip.dataset.statusFilter;
      const active = value === "all" ? state.statuses.size === 0 : state.statuses.has(value);
      chip.classList.toggle("is-active", active);
      chip.setAttribute("aria-pressed", String(active));
    });
    const emptyAll = monitorRoot.querySelector("[data-orivis-monitor-empty-all]");
    const emptyFilter = monitorRoot.querySelector("[data-orivis-monitor-empty-filter]");
    emptyAll?.classList.toggle("is-hidden", total !== 0);
    emptyFilter?.classList.toggle("is-hidden", total === 0 || visible.length !== 0);
    const clear = main.querySelector("#orivis-clear-filters");
    if (clear) clear.disabled = !state.q && state.statuses.size === 0;
    if (persist) writeState(state);
  };

  const updateRefreshUI = main => {
    if (!main) return;
    const paused = refreshPaused();
    const indicator = main.querySelector("#orivis-refresh-indicator");
    const toggle = main.querySelector("#orivis-refresh-toggle");
    if (indicator) {
      indicator.textContent = paused ? indicator.dataset.orivisRefreshPaused : indicator.dataset.orivisRefreshIdle;
      indicator.classList.toggle("is-paused", paused);
    }
    if (toggle) {
      toggle.textContent = paused ? toggle.dataset.orivisRefreshResume : toggle.dataset.orivisRefreshPause;
      toggle.setAttribute("aria-pressed", String(paused));
    }
  };
  const setBusy = (main, busy) => {
    if (!isAutoMain(main)) return;
    main.setAttribute("aria-busy", String(busy));
    updateRefreshUI(main);
  };
  const hideError = () => {
    const error = doc.querySelector("[data-orivis-hx-error]");
    if (error) {
      error.hidden = true;
      error.textContent = "";
    }
  };
  const showError = detail => {
    const main = mainFromDetail(detail);
    if (!main) return;
    setBusy(main, false);
    const error = doc.querySelector("[data-orivis-hx-error]");
    if (!error) return;
    const label = main.querySelector("#orivis-refresh-indicator")?.dataset.orivisRefreshIdle || "Refresh";
    const status = detail?.xhr?.status ? " · " + detail.xhr.status : "";
    error.textContent = label + status;
    error.hidden = false;
  };

  const hydrate = scope => {
    applyTheme(root.dataset.orivisTheme || window.OrivisTheme?.read?.() || "system");
    const main = currentMain(scope);
    if (!main) return;
    const state = monitorState(main);
    const search = main.querySelector("#orivis-monitor-search");
    const sort = main.querySelector("#orivis-monitor-sort");
    if (search && state) search.value = state.q;
    if (sort && state) sort.value = state.sort;
    main.querySelectorAll(".orivis-status-light").forEach(light => {
      const text = light.dataset.orivisTooltip || light.title;
      if (!text) return;
      light.dataset.orivisTooltip = text;
      light.removeAttribute("title");
      light.setAttribute("aria-label", text);
      if (!light.hasAttribute("tabindex")) light.tabIndex = 0;
    });
    applyFilters(main);
    updateRefreshUI(main);
    setBusy(main, false);
  };

  const cardFromEvent = event => event.target?.closest?.("[data-monitor-card][data-monitor-url]");
  const eventHitsControl = (event, card) => {
    const control = event.target?.closest?.(interactiveSelector);
    return Boolean(control && control !== card);
  };
  const openCard = (card, newTab = false) => {
    const url = card?.dataset.monitorUrl;
    if (!url) return;
    if (newTab) window.open(url, "_blank", "noopener,noreferrer");
    else window.location.assign(url);
  };

  const tooltip = () => {
    let node = doc.querySelector("[data-orivis-floating-tooltip]");
    if (node) return node;
    node = doc.createElement("div");
    node.className = "orivis-floating-tooltip";
    node.dataset.orivisFloatingTooltip = "";
    node.setAttribute("role", "tooltip");
    node.setAttribute("aria-hidden", "true");
    doc.body.append(node);
    return node;
  };
  const positionTooltip = (node, x, y) => {
    const rect = node.getBoundingClientRect();
    node.style.left = Math.max(8, Math.min(x + 12, window.innerWidth - rect.width - 8)) + "px";
    node.style.top = Math.max(8, Math.min(y - rect.height - 8, window.innerHeight - rect.height - 8)) + "px";
  };
  const showTooltip = (light, x, y) => {
    const text = light?.dataset.orivisTooltip;
    if (!text) return;
    const node = tooltip();
    node.textContent = text;
    node.classList.add("is-visible");
    node.setAttribute("aria-hidden", "false");
    positionTooltip(node, x, y);
  };
  const hideTooltip = () => {
    const node = doc.querySelector("[data-orivis-floating-tooltip]");
    node?.classList.remove("is-visible");
    node?.setAttribute("aria-hidden", "true");
  };

  doc.addEventListener("click", event => {
    const theme = event.target.closest?.("[data-orivis-theme-option]");
    if (theme) {
      applyTheme(theme.dataset.orivisThemeOption, true);
      return;
    }
    const password = event.target.closest?.("[data-orivis-password-toggle]");
    if (password) {
      const input = password.closest(".orivis-password-field")?.querySelector("input");
      if (!input) return;
      const show = input.type === "password";
      input.type = show ? "text" : "password";
      const label = show ? password.dataset.orivisPasswordHide : password.dataset.orivisPasswordShow;
      password.textContent = label;
      password.setAttribute("aria-label", label);
      password.setAttribute("aria-pressed", String(show));
      return;
    }
    const main = event.target.closest?.("main") || doc.querySelector("main");
    const state = monitorState(main);
    const chip = event.target.closest?.("[data-status-filter]");
    if (chip && state) {
      const value = chip.dataset.statusFilter;
      if (value === "all") state.statuses.clear();
      else if (validStatuses.has(value)) state.statuses.has(value) ? state.statuses.delete(value) : state.statuses.add(value);
      applyFilters(main, true);
      return;
    }
    if (event.target.closest?.("#orivis-clear-filters") && state) {
      state.q = "";
      state.statuses.clear();
      state.sort = defaultSort;
      const search = main.querySelector("#orivis-monitor-search");
      const sort = main.querySelector("#orivis-monitor-sort");
      if (search) search.value = "";
      if (sort) sort.value = defaultSort;
      applyFilters(main, true);
      return;
    }
    if (event.target.closest?.("#orivis-refresh-toggle")) {
      const paused = !refreshPaused();
      storageSet(refreshKey, paused ? "1" : "0");
      updateRefreshUI(main);
      if (!paused && isAutoMain(main)) window.htmx?.trigger(main, "refresh");
      return;
    }
    const card = cardFromEvent(event);
    if (card && !eventHitsControl(event, card)) {
      event.preventDefault();
      openCard(card, event.ctrlKey || event.metaKey);
    }
  });

  doc.addEventListener("input", event => {
    if (event.target.id !== "orivis-monitor-search") return;
    const main = event.target.closest("main");
    const state = monitorState(main);
    if (!state) return;
    state.q = event.target.value;
    window.clearTimeout(filterTimer);
    filterTimer = window.setTimeout(() => applyFilters(main, true), 120);
  });
  doc.addEventListener("change", event => {
    if (event.target.id !== "orivis-monitor-sort") return;
    const main = event.target.closest("main");
    const state = monitorState(main);
    if (!state || !validSorts.has(event.target.value)) return;
    state.sort = event.target.value;
    applyFilters(main, true);
  });
  doc.addEventListener("keydown", event => {
    if (event.altKey && event.shiftKey && event.key.toLowerCase() === "t") {
      event.preventDefault();
      const current = themeModes.includes(root.dataset.orivisTheme) ? root.dataset.orivisTheme : "system";
      applyTheme(themeModes[(themeModes.indexOf(current) + 1) % themeModes.length], true);
      return;
    }
    if (event.key === "/" && !event.metaKey && !event.ctrlKey && !event.altKey && !event.target.matches("input,textarea,[contenteditable]")) {
      const search = doc.querySelector("#orivis-monitor-search");
      if (search) {
        event.preventDefault();
        search.focus();
      }
      return;
    }
    if (event.target.id === "orivis-monitor-search" && event.key === "Escape") {
      const main = event.target.closest("main");
      const state = monitorState(main);
      if (state) {
        state.q = "";
        event.target.value = "";
        applyFilters(main, true);
      }
      return;
    }
    const card = cardFromEvent(event);
    if (card && !eventHitsControl(event, card) && (event.key === "Enter" || event.key === " ")) {
      event.preventDefault();
      openCard(card);
    }
  });
  doc.addEventListener("auxclick", event => {
    const card = cardFromEvent(event);
    if (event.button === 1 && card && !eventHitsControl(event, card)) {
      event.preventDefault();
      openCard(card, true);
    }
  });
  doc.addEventListener("pointerover", event => {
    const light = event.target.closest?.(".orivis-status-light");
    if (light) showTooltip(light, event.clientX, event.clientY);
  });
  doc.addEventListener("pointermove", event => {
    const light = event.target.closest?.(".orivis-status-light");
    const node = doc.querySelector("[data-orivis-floating-tooltip].is-visible");
    if (light && node) positionTooltip(node, event.clientX, event.clientY);
  });
  doc.addEventListener("pointerout", event => {
    if (event.target.closest?.(".orivis-status-light")) hideTooltip();
  });
  doc.addEventListener("focusin", event => {
    const light = event.target.closest?.(".orivis-status-light");
    if (!light) return;
    const rect = light.getBoundingClientRect();
    showTooltip(light, rect.left + rect.width / 2, rect.top);
  });
  doc.addEventListener("focusout", event => {
    if (event.target.closest?.(".orivis-status-light")) hideTooltip();
  });

  doc.addEventListener("htmx:beforeRequest", event => {
    const main = mainFromDetail(event.detail);
    if (!main) return;
    if (refreshPaused()) {
      event.preventDefault();
      updateRefreshUI(main);
      return;
    }
    hideError();
    setBusy(main, true);
  });
  doc.addEventListener("htmx:afterRequest", event => {
    const main = mainFromDetail(event.detail);
    if (main) setBusy(main, false);
  });
  doc.addEventListener("htmx:beforeSwap", event => {
    const main = mainFromDetail(event.detail);
    if (!main) return;
    const search = doc.activeElement?.id === "orivis-monitor-search" ? doc.activeElement : null;
    swapFocus = search ? { focused: true, start: search.selectionStart, end: search.selectionEnd } : null;
  });
  doc.addEventListener("htmx:afterSwap", event => {
    const swappedMain = mainFromDetail(event.detail);
    if (!swappedMain) return;
    const main = currentMain(doc);
    hydrate(main);
    hideError();
    if (swapFocus?.focused) {
      const search = main.querySelector("#orivis-monitor-search");
      search?.focus({ preventScroll: true });
      if (search && swapFocus.start !== null) search.setSelectionRange(swapFocus.start, swapFocus.end);
    }
    swapFocus = null;
  });
  ["htmx:responseError", "htmx:sendError", "htmx:timeout", "htmx:swapError"].forEach(name => doc.addEventListener(name, event => showError(event.detail)));

  media?.addEventListener?.("change", () => {
    if (root.dataset.orivisTheme === "system") applyTheme("system");
  });
  window.OrivisThemeSwitch = { apply: (mode, persist = true) => applyTheme(mode, persist), bind: hydrate, read: () => root.dataset.orivisTheme };
  if (doc.readyState === "loading") doc.addEventListener("DOMContentLoaded", () => hydrate(doc), { once: true });
  else hydrate(doc);
})();
