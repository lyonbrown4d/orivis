window.tailwind = window.tailwind || {};
window.tailwind.config = { darkMode: 'class' };

(function applyInitialTheme() {
  const stored = localStorage.getItem('orivis_theme');
  const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  const useDark = stored ? stored === 'dark' : prefersDark;
  document.documentElement.classList.toggle('dark', useDark);
  document.documentElement.dataset.theme = stored || 'auto';
})();
