import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Activity, ArrowLeft, BellRing, CircleAlert, Cpu, ExternalLink, RadioTower, Timer } from 'lucide-react';
import { AppShell } from '@/components/AppShell';
import { PreferenceControls } from '@/components/PreferenceControls';
import { StatusBadge, StatusLights } from '@/components/status';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { errorMessage, fetchSnapshot, isUnauthorized, type DashboardSnapshot, type Monitor, type NotificationDelivery, type Result } from '@/lib/api';

export default function DashboardPage({ statusPage = false }: { statusPage?: boolean }) {
  const { t, i18n } = useTranslation();
  const { group } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const selectedGroup = statusPage ? group : undefined;
  const snapshotQuery = useQuery({
    queryKey: ['dashboard-snapshot', selectedGroup || 'all'],
    queryFn: () => fetchSnapshot(selectedGroup),
    refetchInterval: 15_000,
    retry: (failureCount, error) => !isUnauthorized(error) && failureCount < 2
  });

  useEffect(() => {
    if (snapshotQuery.error && isUnauthorized(snapshotQuery.error)) {
      navigate(`/login?redirect=${encodeURIComponent(location.pathname)}`, { replace: true });
    }
  }, [location.pathname, navigate, snapshotQuery.error]);

  const formatTime = useMemo(
    () => (value?: string) => {
      if (!value) return '-';
      return new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value));
    },
    [i18n.language]
  );

  if (snapshotQuery.isLoading) {
    return <LoadingScreen message={t('loading')} />;
  }

  if (snapshotQuery.isError) {
    return <ErrorScreen message={errorMessage(snapshotQuery.error, t('unknownError'))} />;
  }

  const snapshot = snapshotQuery.data;
  if (!snapshot) {
    return <LoadingScreen message={t('loading')} />;
  }

  if (statusPage) {
    return <StatusPageView snapshot={snapshot} formatTime={formatTime} />;
  }

  return (
    <AppShell snapshot={snapshot}>
      <DashboardView snapshot={snapshot} formatTime={formatTime} isFetching={snapshotQuery.isFetching} />
    </AppShell>
  );
}

function DashboardView({
  snapshot,
  formatTime,
  isFetching
}: {
  snapshot: DashboardSnapshot;
  formatTime: (value?: string) => string;
  isFetching: boolean;
}) {
  const { t } = useTranslation();

  return (
    <div className="grid gap-6 xl:grid-cols-[1fr_360px]">
      <section className="space-y-6">
        <div className="grid gap-4 md:grid-cols-4">
          <SummaryCard label={t('monitors')} value={snapshot.summary.monitors} tone="slate" />
          <SummaryCard label={t('up')} value={snapshot.summary.up} tone="emerald" />
          <SummaryCard label={t('down')} value={snapshot.summary.down} tone="rose" />
          <SummaryCard label={t('unknown')} value={snapshot.summary.unknown} tone="amber" />
        </div>

        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div>
              <CardTitle className="text-xl font-black">{t('recentUptime')}</CardTitle>
              <p className="text-sm text-slate-500 dark:text-slate-400">{isFetching ? t('refreshing') : `${t('updated')}: ${formatTime(snapshot.generated_at)}`}</p>
            </div>
            <Badge className="w-fit rounded-full px-3 py-1 text-xs uppercase tracking-wide">
              {snapshot.status_lights.length} {t('total')}
            </Badge>
          </CardHeader>
          <CardContent>
            <StatusLights lights={snapshot.status_lights} empty={t('emptyHistory')} formatTime={formatTime} />
          </CardContent>
        </Card>

        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="text-xl font-black">{t('monitors')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {snapshot.monitors.length === 0 ? <EmptyState text={t('noMonitors')} /> : snapshot.monitors.map((monitor) => <MonitorRow key={monitor.id} monitor={monitor} formatTime={formatTime} />)}
          </CardContent>
        </Card>

        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="text-xl font-black">{t('recentResults')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {snapshot.recent_results.length === 0 ? <EmptyState text={t('noResults')} /> : snapshot.recent_results.slice(0, 12).map((result) => <ResultRow key={result.id} result={result} formatTime={formatTime} />)}
          </CardContent>
        </Card>
      </section>

      <aside className="space-y-6">
        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="text-xl font-black">{t('groups')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {snapshot.groups.map((group) => (
              <Link key={group.slug} to={`/${group.slug}`} className="block rounded-3xl border border-slate-200 bg-white/75 p-4 transition hover:-translate-y-0.5 hover:shadow-lg dark:border-slate-800 dark:bg-slate-900/70">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-black text-slate-900 dark:text-white">{group.name}</p>
                    <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{group.count} monitors</p>
                  </div>
                  <ExternalLink className="h-4 w-4 text-slate-400" />
                </div>
                <div className="mt-4 grid grid-cols-3 gap-2 text-center text-xs">
                  <Metric label={t('up')} value={group.up} />
                  <Metric label={t('down')} value={group.down} />
                  <Metric label={t('unknown')} value={group.unknown} />
                </div>
              </Link>
            ))}
          </CardContent>
        </Card>

        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="text-xl font-black">{t('agents')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {snapshot.agents.length === 0 ? (
              <EmptyState text={t('noAgents')} />
            ) : (
              snapshot.agents.map((agent) => (
                <div key={agent.id} className="rounded-3xl border border-slate-200 bg-white/75 p-4 dark:border-slate-800 dark:bg-slate-900/70">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="font-black">{agent.name}</p>
                      <p className="text-sm text-slate-500 dark:text-slate-400">{agent.runtime_type || '-'}</p>
                    </div>
                    <Badge className="rounded-full">
                      {agent.status || 'online'}
                    </Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <Badge className="rounded-full">
                      {agent.region_code || '-'}
                    </Badge>
                    {agent.environment_codes.map((code) => (
                      <Badge key={code} className="rounded-full">
                        {code}
                      </Badge>
                    ))}
                  </div>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl font-black">
              <BellRing className="h-5 w-5 text-teal-600 dark:text-teal-300" />
              {t('notifications')}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {snapshot.notifications.length === 0 ? <EmptyState text={t('noNotifications')} /> : snapshot.notifications.slice(0, 8).map((item) => <NotificationRow key={item.id} item={item} formatTime={formatTime} />)}
          </CardContent>
        </Card>
      </aside>
    </div>
  );
}

function StatusPageView({ snapshot, formatTime }: { snapshot: DashboardSnapshot; formatTime: (value?: string) => string }) {
  const { t } = useTranslation();
  const groupName = snapshot.selected_group || snapshot.group_slug || t('allServices');

  return (
    <main className="min-h-screen bg-[linear-gradient(160deg,_#f8fafc,_#ecfeff_42%,_#fefce8)] p-5 text-slate-950 dark:bg-[linear-gradient(160deg,_#020617,_#0f172a_48%,_#111827)] dark:text-white">
      <div className="mx-auto max-w-5xl space-y-6">
        <header className="flex flex-col gap-4 py-5 md:flex-row md:items-center md:justify-between">
          <Link to="/" className="inline-flex items-center gap-2 text-sm font-semibold text-slate-600 hover:text-slate-950 dark:text-slate-300 dark:hover:text-white">
            <ArrowLeft className="h-4 w-4" />
            {t('backToDashboard')}
          </Link>
          <PreferenceControls />
        </header>

        <section className="rounded-[2rem] border border-white/70 bg-white/80 p-6 shadow-2xl shadow-teal-900/10 backdrop-blur-xl dark:border-slate-800 dark:bg-slate-950/75 md:p-10">
          <div className="flex flex-col gap-6 md:flex-row md:items-start md:justify-between">
            <div>
              <p className="text-sm font-bold uppercase tracking-[0.22em] text-teal-700 dark:text-teal-300">{t('statusPage')}</p>
              <h1 className="mt-3 text-4xl font-black tracking-tight md:text-6xl">{groupName}</h1>
              <p className="mt-4 text-sm text-slate-500 dark:text-slate-400">
                {t('updated')}: {formatTime(snapshot.generated_at)}
              </p>
            </div>
            <div className="grid grid-cols-3 gap-3 text-center">
              <SummaryPill label={t('up')} value={snapshot.summary.up} />
              <SummaryPill label={t('down')} value={snapshot.summary.down} />
              <SummaryPill label={t('unknown')} value={snapshot.summary.unknown} />
            </div>
          </div>

          <div className="mt-8">
            <StatusLights lights={snapshot.status_lights} empty={t('emptyHistory')} formatTime={formatTime} />
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-2">
          {snapshot.monitors.map((monitor) => (
            <Card key={monitor.id} className="border-white/70 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/70">
              <CardContent className="p-5">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <p className="text-lg font-black">{monitor.name}</p>
                    <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{monitor.target}</p>
                  </div>
                  <StatusBadge status={monitor.latest?.status || 'unknown'} label={t(monitor.latest?.status || 'unknown')} />
                </div>
                <div className="mt-5 grid grid-cols-2 gap-3 text-sm">
                  <Metric label={t('environment')} value={monitor.environment_code || '-'} />
                  <Metric label={t('latency')} value={monitor.latest ? `${monitor.latest.latency_ms}ms` : '-'} />
                  <Metric label={t('source')} value={monitor.source || monitor.discovery_source || '-'} />
                  <Metric label={t('checkedAt')} value={formatTime(monitor.latest?.checked_at)} />
                </div>
              </CardContent>
            </Card>
          ))}
        </section>
      </div>
    </main>
  );
}

function SummaryCard({ label, value, tone }: { label: string; value: number; tone: 'slate' | 'emerald' | 'rose' | 'amber' }) {
  const tones = {
    slate: 'from-slate-900 to-slate-700 text-white dark:from-slate-100 dark:to-white dark:text-slate-950',
    emerald: 'from-emerald-500 to-teal-500 text-white',
    rose: 'from-rose-500 to-red-500 text-white',
    amber: 'from-amber-300 to-orange-400 text-slate-950'
  };

  return (
    <Card className="overflow-hidden border-white/60 bg-white/70 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/60">
      <CardContent className={`bg-gradient-to-br p-5 ${tones[tone]}`}>
        <p className="text-xs font-bold uppercase tracking-[0.22em] opacity-75">{label}</p>
        <p className="mt-4 text-4xl font-black">{value}</p>
      </CardContent>
    </Card>
  );
}

function SummaryPill({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-3xl border border-slate-200 bg-white/75 p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900/75">
      <p className="text-3xl font-black">{value}</p>
      <p className="mt-1 text-xs font-bold uppercase tracking-[0.18em] text-slate-400">{label}</p>
    </div>
  );
}

function MonitorRow({ monitor, formatTime }: { monitor: Monitor; formatTime: (value?: string) => string }) {
  const { t } = useTranslation();

  return (
    <div className="rounded-3xl border border-slate-200 bg-white/75 p-4 dark:border-slate-800 dark:bg-slate-900/70">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <p className="font-black text-slate-950 dark:text-white">{monitor.name}</p>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{monitor.target}</p>
        </div>
        <StatusBadge status={monitor.latest?.status || 'unknown'} label={t(monitor.latest?.status || 'unknown')} />
      </div>
      <div className="mt-4 grid gap-3 text-sm md:grid-cols-4">
        <IconMetric icon={<Cpu className="h-4 w-4" />} label={t('environment')} value={monitor.environment_code || '-'} />
        <IconMetric icon={<RadioTower className="h-4 w-4" />} label={t('source')} value={monitor.source || monitor.discovery_source || '-'} />
        <IconMetric icon={<Timer className="h-4 w-4" />} label={t('latency')} value={monitor.latest ? `${monitor.latest.latency_ms}ms` : '-'} />
        <IconMetric icon={<Activity className="h-4 w-4" />} label={t('checkedAt')} value={formatTime(monitor.latest?.checked_at)} />
      </div>
      {monitor.latest?.error_message && (
        <div className="mt-4 flex items-start gap-2 rounded-2xl bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:bg-rose-950 dark:text-rose-300">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" />
          {monitor.latest.error_message}
        </div>
      )}
    </div>
  );
}

function ResultRow({ result, formatTime }: { result: Result; formatTime: (value?: string) => string }) {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col gap-3 rounded-3xl border border-slate-200 bg-white/75 p-4 dark:border-slate-800 dark:bg-slate-900/70 md:flex-row md:items-center md:justify-between">
      <div>
        <p className="font-black">{result.monitor_name}</p>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          {result.agent_name} · {result.environment_code || '-'} · {formatTime(result.checked_at)}
        </p>
        {result.error_message && <p className="mt-1 text-sm text-rose-600 dark:text-rose-300">{result.error_message}</p>}
      </div>
      <div className="flex items-center gap-3">
        <span className="text-sm font-semibold text-slate-500 dark:text-slate-400">{result.latency_ms}ms</span>
        <StatusBadge status={result.status} label={t(result.status)} />
      </div>
    </div>
  );
}

function NotificationRow({ item, formatTime }: { item: NotificationDelivery; formatTime: (value?: string) => string }) {
  const { t } = useTranslation();

  return (
    <div className="rounded-3xl border border-slate-200 bg-white/75 p-4 dark:border-slate-800 dark:bg-slate-900/70">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-black text-slate-950 dark:text-white">{item.monitor_name || item.monitor_id || '-'}</p>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {item.channel} · {item.event}
          </p>
        </div>
        <Badge className={`rounded-full ${notificationStatusClass(item.status)}`}>{t(item.status)}</Badge>
      </div>
      <div className="mt-4 grid grid-cols-2 gap-2 text-xs">
        <Metric label={t('attempt')} value={`${item.attempt}/${item.max_attempts}`} />
        <Metric label={t('httpStatus')} value={item.http_status || '-'} />
        <Metric label={t('duration')} value={`${item.duration_ms}ms`} />
        <Metric label={t('sentAt')} value={formatTime(item.created_at)} />
      </div>
      {item.error_message && (
        <div className="mt-3 flex items-start gap-2 rounded-2xl bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:bg-rose-950 dark:text-rose-300">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{item.error_message}</span>
        </div>
      )}
    </div>
  );
}

function notificationStatusClass(status: string) {
  return status === 'success'
    ? 'bg-emerald-50 text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-950 dark:text-emerald-300'
    : 'bg-rose-50 text-rose-700 hover:bg-rose-100 dark:bg-rose-950 dark:text-rose-300';
}

function IconMetric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-center gap-2 text-slate-600 dark:text-slate-300">
      <span className="grid h-8 w-8 place-items-center rounded-2xl bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-300">{icon}</span>
      <span>
        <span className="block text-xs uppercase tracking-[0.14em] text-slate-400">{label}</span>
        <span className="font-semibold">{value}</span>
      </span>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="rounded-2xl bg-slate-100/80 px-3 py-2 dark:bg-slate-800/80">
      <p className="text-xs uppercase tracking-[0.14em] text-slate-400">{label}</p>
      <p className="mt-1 font-bold text-slate-800 dark:text-slate-100">{value}</p>
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return <div className="rounded-3xl border border-dashed border-slate-300 p-6 text-center text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">{text}</div>;
}

function LoadingScreen({ message }: { message: string }) {
  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 text-slate-700 dark:bg-slate-950 dark:text-slate-200">
      <div className="rounded-3xl border border-slate-200 bg-white px-6 py-5 shadow-xl dark:border-slate-800 dark:bg-slate-900">{message}</div>
    </main>
  );
}

function ErrorScreen({ message }: { message: string }) {
  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 p-5 text-slate-700 dark:bg-slate-950 dark:text-slate-200">
      <div className="max-w-md rounded-3xl border border-rose-200 bg-white px-6 py-5 text-center shadow-xl dark:border-rose-950 dark:bg-slate-900">
        <p className="text-lg font-black text-rose-600 dark:text-rose-300">{message}</p>
        <Button asChild className="mt-4 rounded-full">
          <Link to="/login">Login</Link>
        </Button>
      </div>
    </main>
  );
}
