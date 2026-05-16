(function () {
  function jsonFromNode(id, fallback) {
    const node = document.getElementById(id);
    if (!node) return fallback;
    try {
      return JSON.parse(node.textContent || '');
    } catch {
      return fallback;
    }
  }

  window.switchLang = function switchLang(code) {
    localStorage.setItem('orivis_lang', code);
    const params = new URLSearchParams(window.location.search);
    params.set('lang', code);
    const hash = window.location.hash || '#overview';
    window.location.replace(`${window.location.pathname}?${params.toString()}${hash}`);
  };

  function redirectPreferredLanguage() {
    const preferred = localStorage.getItem('orivis_lang');
    const params = new URLSearchParams(window.location.search);
    if (!params.get('lang') && preferred) {
      params.set('lang', preferred);
      const hash = window.location.hash || '#overview';
      window.location.replace(`${window.location.pathname}?${params.toString()}${hash}`);
    }
  }

  window.applyTheme = function applyTheme(value) {
    const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    const theme = value || localStorage.getItem('orivis_theme') || 'auto';
    const useDark = theme === 'dark' || (theme === 'auto' && prefersDark);
    document.documentElement.classList.toggle('dark', useDark);
    document.documentElement.dataset.theme = theme;
    document.querySelectorAll('.theme-button').forEach(function (button) {
      button.dataset.active = String(button.dataset.themeValue === theme);
    });
  };

  window.switchTheme = function switchTheme(value) {
    if (value === 'auto') {
      localStorage.removeItem('orivis_theme');
    } else {
      localStorage.setItem('orivis_theme', value);
    }
    window.applyTheme(value);
  };

  function bindSystemTheme() {
    if (!window.matchMedia) return;
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function () {
      if (!localStorage.getItem('orivis_theme')) {
        window.applyTheme('auto');
      }
    });
  }

  function bindLoginForm() {
    const form = document.getElementById('orivis-login-form');
    if (!form) return;
    form.addEventListener('submit', function (event) {
      event.preventDefault();
      const error = document.getElementById('orivis-login-error');
      if (error) error.classList.add('hidden');

      fetch('/login' + window.location.search, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: form.elements.username.value,
          password: form.elements.password.value
        })
      }).then(function (response) {
        if (!response.ok) throw new Error('login failed');
        window.location.replace(form.dataset.redirect || '/');
      }).catch(function () {
        if (error) error.classList.remove('hidden');
      });
    });
  }

  function renderStatusChart() {
    const chart = document.getElementById('orivis-status-chart');
    const empty = document.getElementById('orivis-status-chart-empty');
    const detail = document.getElementById('orivis-status-chart-detail');
    const points = jsonFromNode('orivis-status-chart-data', []);
    const labels = jsonFromNode('orivis-dashboard-labels', {});

    if (!chart || !window.uPlot || !Array.isArray(points) || points.length === 0) {
      if (chart) chart.classList.add('hidden');
      if (empty) empty.classList.remove('hidden');
      return;
    }

    const times = points.map((point) => Date.parse(point.time) / 1000);
    const scores = points.map((point) => point.score);
    const statusLabel = function (value) {
      if (value >= 0.9) return labels.up;
      if (value >= 0.5) return labels.degraded;
      if (value > 0.05) return labels.unknown;
      return labels.down;
    };
    const setDetail = function (index) {
      const point = points[index];
      if (!point || !detail) return;
      const name = point.monitor_name || '-';
      detail.textContent = `${point.label} / ${name} / ${labels.status}: ${statusLabel(point.score)} / ${labels.latency}: ${point.latency_ms}ms`;
    };

    const plot = new uPlot({
      width: Math.max(chart.clientWidth, 320),
      height: 280,
      padding: [12, 16, 0, 0],
      scales: {
        x: { time: true },
        y: { range: [-0.08, 1.08] }
      },
      axes: [
        {
          stroke: '#64748b',
          grid: { stroke: '#e2e8f0' },
          values: function (_plot, values) {
            return values.map((value) => new Date(value * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
          }
        },
        {
          stroke: '#64748b',
          grid: { stroke: '#e2e8f0' },
          values: function (_plot, values) {
            return values.map(statusLabel);
          }
        }
      ],
      series: [
        {},
        {
          label: labels.status,
          stroke: '#10b981',
          width: 3,
          points: {
            show: true,
            size: 7,
            stroke: '#10221f',
            fill: '#f8fafc'
          }
        }
      ],
      cursor: { points: { size: 9 } },
      hooks: {
        setCursor: [
          function (plotRef) {
            if (Number.isInteger(plotRef.cursor.idx)) {
              setDetail(plotRef.cursor.idx);
            }
          }
        ]
      }
    }, [times, scores], chart);

    setDetail(points.length - 1);
    if (window.ResizeObserver) {
      new ResizeObserver(function () {
        plot.setSize({ width: Math.max(chart.clientWidth, 320), height: 280 });
      }).observe(chart);
    }
  }

  document.addEventListener('DOMContentLoaded', function () {
    redirectPreferredLanguage();
    window.applyTheme(document.documentElement.dataset.theme || 'auto');
    bindSystemTheme();
    bindLoginForm();
    renderStatusChart();
  });
})();
