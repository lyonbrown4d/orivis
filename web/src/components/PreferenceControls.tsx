import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { setLanguage } from '@/lib/i18n';
import { cn } from '@/lib/utils';

type ThemeChoice = 'light' | 'dark' | 'auto';

const themeKey = 'orivis_theme';

function applyTheme(theme: ThemeChoice) {
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  const dark = theme === 'dark' || (theme === 'auto' && prefersDark);
  document.documentElement.classList.toggle('dark', dark);
}

export function PreferenceControls({ compact = false }: { compact?: boolean }) {
  const { i18n, t } = useTranslation();
  const [theme, setTheme] = useState<ThemeChoice>(() => {
    const saved = localStorage.getItem(themeKey);
    return saved === 'light' || saved === 'dark' || saved === 'auto' ? saved : 'auto';
  });

  useEffect(() => {
    applyTheme(theme);
    localStorage.setItem(themeKey, theme);

    const query = window.matchMedia('(prefers-color-scheme: dark)');
    const listener = () => theme === 'auto' && applyTheme('auto');
    query.addEventListener('change', listener);

    return () => query.removeEventListener('change', listener);
  }, [theme]);

  const themes = useMemo<Array<{ code: ThemeChoice; label: string }>>(
    () => [
      { code: 'auto', label: t('auto') },
      { code: 'light', label: t('light') },
      { code: 'dark', label: t('dark') }
    ],
    [t]
  );

  const languages = useMemo(
    () => [
      { code: 'en' as const, label: 'EN' },
      { code: 'zh' as const, label: '中文' }
    ],
    []
  );

  return (
    <div className={cn('flex flex-wrap items-center gap-2', compact && 'justify-center')}>
      <div className="flex rounded-full border border-slate-200 bg-white/75 p-1 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/70">
        {themes.map((item) => (
          <Button
            key={item.code}
            type="button"
            variant={theme === item.code ? 'default' : 'ghost'}
            size="sm"
            className="h-7 rounded-full px-3 text-xs"
            onClick={() => setTheme(item.code)}
          >
            {item.label}
          </Button>
        ))}
      </div>

      <div className="flex rounded-full border border-slate-200 bg-white/75 p-1 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/70">
        {languages.map((item) => (
          <Button
            key={item.code}
            type="button"
            variant={i18n.language.startsWith(item.code) ? 'default' : 'ghost'}
            size="sm"
            className="h-7 rounded-full px-3 text-xs"
            onClick={() => void setLanguage(item.code)}
          >
            {item.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
