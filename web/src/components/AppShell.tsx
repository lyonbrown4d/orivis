import { Activity, Database, LogOut, Server, ShieldCheck } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Link, NavLink } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { PreferenceControls } from '@/components/PreferenceControls';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { logout, type DashboardSnapshot } from '@/lib/api';
import { cn } from '@/lib/utils';

export function AppShell({ snapshot, children }: { snapshot?: DashboardSnapshot; children: React.ReactNode }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSettled: async () => {
      await queryClient.invalidateQueries();
      window.location.href = '/login';
    }
  });

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,_rgba(20,184,166,0.16),_transparent_30%),linear-gradient(135deg,_#f8fafc,_#eef2ff_48%,_#ecfeff)] text-slate-950 dark:bg-[radial-gradient(circle_at_top_left,_rgba(45,212,191,0.12),_transparent_30%),linear-gradient(135deg,_#020617,_#0f172a_48%,_#111827)] dark:text-slate-50">
      <div className="grid min-h-screen lg:grid-cols-[280px_1fr]">
        <aside className="border-b border-white/50 bg-white/70 p-5 shadow-xl shadow-slate-200/40 backdrop-blur-xl dark:border-slate-800/80 dark:bg-slate-950/75 dark:shadow-black/20 lg:sticky lg:top-0 lg:h-screen lg:overflow-y-auto lg:border-b-0 lg:border-r">
          <Link to="/" className="flex items-center gap-3">
            <div className="grid h-11 w-11 place-items-center rounded-2xl bg-slate-950 text-white shadow-lg shadow-teal-500/20 dark:bg-white dark:text-slate-950">
              <Activity className="h-5 w-5" />
            </div>
            <div>
              <p className="text-lg font-black tracking-tight">Orivis</p>
              <p className="text-xs uppercase tracking-[0.28em] text-slate-500 dark:text-slate-400">Uptime</p>
            </div>
          </Link>

          <nav className="mt-8 space-y-2">
            <NavLink
              to="/"
              className={({ isActive }) =>
                cn(
                  'flex items-center gap-3 rounded-2xl px-4 py-3 text-sm font-semibold transition',
                  isActive ? 'bg-slate-950 text-white shadow-lg dark:bg-white dark:text-slate-950' : 'text-slate-600 hover:bg-white/80 dark:text-slate-300 dark:hover:bg-slate-900'
                )
              }
            >
              <Activity className="h-4 w-4" />
              {t('dashboard')}
            </NavLink>
          </nav>

          <div className="mt-8 space-y-3">
            <p className="text-xs font-bold uppercase tracking-[0.22em] text-slate-400">{t('groups')}</p>
            <Link
              to="/"
              className="flex items-center justify-between rounded-2xl border border-slate-200 bg-white/70 px-4 py-3 text-sm font-semibold text-slate-700 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200"
            >
              {t('allServices')}
              <span>{snapshot?.all_monitors ?? 0}</span>
            </Link>
            {snapshot?.groups.length === 0 && (
              <div className="rounded-2xl border border-dashed border-slate-300 px-4 py-3 text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">{t('noGroups')}</div>
            )}
            {snapshot?.groups.map((group) => (
              <Link
                key={group.slug}
                to={`/${group.slug}`}
                className="group flex items-center justify-between rounded-2xl border border-slate-200 bg-white/50 px-4 py-3 text-sm transition hover:-translate-y-0.5 hover:bg-white hover:shadow-lg dark:border-slate-800 dark:bg-slate-900/50 dark:hover:bg-slate-900"
              >
                <span>
                  <span className="block font-semibold text-slate-700 dark:text-slate-200">{group.name}</span>
                  <span className="mt-1 block text-xs text-slate-500 dark:text-slate-400">
                    {group.up}/{group.count} {t('up')}
                  </span>
                </span>
                <span className="rounded-full bg-slate-100 px-2 py-1 text-xs font-black text-slate-600 dark:bg-slate-800 dark:text-slate-300">{group.count}</span>
              </Link>
            ))}
          </div>
        </aside>

        <main className="min-w-0 p-5 lg:p-8">
          <header className="mb-6 flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
            <div>
              <p className="text-sm font-semibold text-teal-700 dark:text-teal-300">{snapshot?.env || t('server')}</p>
              <h1 className="mt-1 text-3xl font-black tracking-tight md:text-5xl">{t('healthOverview')}</h1>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
              <PreferenceControls />
              {snapshot?.auth_enabled && (
                <Button variant="outline" className="rounded-full bg-white/70 dark:bg-slate-950/60" onClick={() => logoutMutation.mutate()}>
                  <LogOut className="mr-2 h-4 w-4" />
                  {t('signOut')}
                </Button>
              )}
            </div>
          </header>

          <div className="mb-6 grid gap-3 md:grid-cols-3">
            <MetaCard icon={<Server className="h-4 w-4" />} label={t('server')} value={snapshot?.name || 'orivis'} />
            <MetaCard icon={<Database className="h-4 w-4" />} label={t('storage')} value={snapshot?.database.driver || 'sqlite'} />
            <MetaCard icon={<ShieldCheck className="h-4 w-4" />} label={t('authRequired')} value={snapshot?.auth_enabled ? t('up') : t('down')} />
          </div>

          {children}
        </main>
      </div>
    </div>
  );
}

function MetaCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <Card className="border-white/60 bg-white/70 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/55">
      <CardContent className="flex items-center gap-3 p-4">
        <div className="grid h-10 w-10 place-items-center rounded-2xl bg-slate-950 text-white dark:bg-white dark:text-slate-950">{icon}</div>
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.18em] text-slate-400">{label}</p>
          <p className="text-sm font-semibold text-slate-800 dark:text-slate-100">{value}</p>
        </div>
      </CardContent>
    </Card>
  );
}
