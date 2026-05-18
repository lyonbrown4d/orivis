import { useEffect, useMemo, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, BellRing, CircleAlert, Cpu, RadioTower, Timer } from 'lucide-react';
import { AppShell } from '@/components/AppShell';
import { StatusBadge, StatusLights } from '@/components/status';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { errorMessage, fetchSnapshot, isUnauthorized, type DashboardSnapshot, type Monitor, type NotificationDelivery, type Result } from '@/lib/api';

export default function MonitorDetailPage() {
  const { t, i18n } = useTranslation();
  const { monitorId = '' } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const snapshotQuery = useQuery({
    queryKey: ['dashboard-snapshot', 'monitor-detail'],
    queryFn: () => fetchSnapshot(),
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
    return <DetailScreen message={t('loading')} />;
  }
  if (snapshotQuery.isError) {
    return <DetailScreen message={errorMessage(snapshotQuery.error, t('unknownError'))} />;
  }

  const snapshot = snapshotQuery.data;
  const monitor = snapshot?.monitors.find((item) => item.id === monitorId);
  if (!snapshot || !monitor) {
    return (
      <AppShell snapshot={snapshot}>
        <NotFoundCard />
      </AppShell>
    );
  }

  const results = snapshot.recent_results.filter((item) => item.monitor_id === monitor.id);
  const notifications = snapshot.notifications.filter((item) => item.monitor_id === monitor.id);

  return (
    <AppShell snapshot={snapshot}>
      <MonitorDetailView monitor={monitor} results={results} notifications={notifications} snapshot={snapshot} formatTime={formatTime} />
    </AppShell>
  );
}

function MonitorDetailView({
  monitor,
  results,
  notifications,
  snapshot,
  formatTime
}: {
  monitor: Monitor;
  results: Result[];
  notifications: NotificationDelivery[];
  snapshot: DashboardSnapshot;
  formatTime: (value?: string) => string;
}) {
  const { t } = useTranslation();
  const latest = monitor.latest;
  const lights = results
    .slice()
    .reverse()
    .map((result) => ({
      monitor_name: monitor.name,
      status: result.status,
      latency_ms: result.latency_ms,
      checked_at: result.checked_at
    }));

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <Button asChild variant="outline" className="w-fit rounded-full bg-white/70 dark:bg-slate-950/60">
          <Link to="/">
            <ArrowLeft className="mr-2 h-4 w-4" />
            {t('backToDashboard')}
          </Link>
        </Button>
        <Badge className="w-fit rounded-full px-3 py-1 text-xs uppercase tracking-wide">
          {t('updated')}: {formatTime(snapshot.generated_at)}
        </Badge>
      </div>

      <section className="grid gap-6 xl:grid-cols-[1fr_360px]">
        <Card className="overflow-hidden border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardContent className="p-0">
            <div className="bg-gradient-to-br from-slate-950 via-slate-800 to-teal-700 p-6 text-white dark:from-white dark:via-slate-100 dark:to-teal-200 dark:text-slate-950 md:p-8">
              <p className="text-xs font-bold uppercase tracking-[0.22em] opacity-70">{t('monitorDetails')}</p>
              <div className="mt-4 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                <div>
                  <h2 className="text-3xl font-black tracking-tight md:text-5xl">{monitor.name}</h2>
                  <p className="mt-3 max-w-3xl break-all text-sm opacity-75">{monitor.target}</p>
                </div>
                <StatusBadge status={latest?.status || 'unknown'} label={t(latest?.status || 'unknown')} />
              </div>
            </div>
            <div className="grid gap-3 p-5 md:grid-cols-4">
              <DetailMetric icon={<Cpu className="h-4 w-4" />} label={t('environment')} value={monitor.environment_code || '-'} />
              <DetailMetric icon={<RadioTower className="h-4 w-4" />} label={t('source')} value={monitor.source || monitor.discovery_source || '-'} />
              <DetailMetric icon={<Timer className="h-4 w-4" />} label={t('latency')} value={latest ? `${latest.latency_ms}ms` : '-'} />
              <DetailMetric icon={<BellRing className="h-4 w-4" />} label={t('notifications')} value={String(notifications.length)} />
            </div>
          </CardContent>
        </Card>

        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="text-xl font-black">{t('latestCheck')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Metric label={t('checkedAt')} value={formatTime(latest?.checked_at)} />
            <Metric label={t('interval')} value={`${Math.round(monitor.interval_ms / 1000)}s`} />
            <Metric label={t('timeout')} value={`${Math.round(monitor.timeout_ms / 1000)}s`} />
            <Metric label={t('retryCount')} value={monitor.retry_count} />
            {latest?.error_message && (
              <div className="flex items-start gap-2 rounded-2xl bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:bg-rose-950 dark:text-rose-300">
                <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" />
                {latest.error_message}
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
        <CardHeader>
          <CardTitle className="text-xl font-black">{t('probeHistory')}</CardTitle>
        </CardHeader>
        <CardContent>
          <StatusLights lights={lights} empty={t('emptyHistory')} formatTime={formatTime} />
        </CardContent>
      </Card>

      <section className="grid gap-6 xl:grid-cols-2">
        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="text-xl font-black">{t('recentResults')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {results.length === 0 ? <EmptyState text={t('noResults')} /> : results.map((result) => <DetailResultRow key={result.id} result={result} formatTime={formatTime} />)}
          </CardContent>
        </Card>

        <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
          <CardHeader>
            <CardTitle className="text-xl font-black">{t('notificationDeliveries')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {notifications.length === 0 ? <EmptyState text={t('noNotifications')} /> : notifications.map((item) => <DetailNotificationRow key={item.id} item={item} formatTime={formatTime} />)}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}

function DetailResultRow({ result, formatTime }: { result: Result; formatTime: (value?: string) => string }) {
  const { t } = useTranslation();

  return (
    <div className="rounded-3xl border border-slate-200 bg-white/75 p-4 dark:border-slate-800 dark:bg-slate-900/70">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-black">{formatTime(result.checked_at)}</p>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {result.agent_name || result.agent_id || '-'} / {result.region_code || '-'} / {result.environment_code || '-'}
          </p>
        </div>
        <StatusBadge status={result.status} label={t(result.status)} />
      </div>
      <div className="mt-4 grid grid-cols-2 gap-2 text-sm">
        <Metric label={t('latency')} value={`${result.latency_ms}ms`} />
        <Metric label={t('createdAt')} value={formatTime(result.created_at)} />
      </div>
      {result.error_message && <p className="mt-3 rounded-2xl bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:bg-rose-950 dark:text-rose-300">{result.error_message}</p>}
    </div>
  );
}

function DetailNotificationRow({ item, formatTime }: { item: NotificationDelivery; formatTime: (value?: string) => string }) {
  const { t } = useTranslation();

  return (
    <div className="rounded-3xl border border-slate-200 bg-white/75 p-4 dark:border-slate-800 dark:bg-slate-900/70">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-black">{item.channel}</p>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{item.event}</p>
        </div>
        <Badge className={`rounded-full ${item.status === 'success' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' : 'bg-rose-50 text-rose-700 dark:bg-rose-950 dark:text-rose-300'}`}>{t(item.status)}</Badge>
      </div>
      <div className="mt-4 grid grid-cols-2 gap-2 text-sm">
        <Metric label={t('attempt')} value={`${item.attempt}/${item.max_attempts}`} />
        <Metric label={t('httpStatus')} value={item.http_status || '-'} />
        <Metric label={t('duration')} value={`${item.duration_ms}ms`} />
        <Metric label={t('sentAt')} value={formatTime(item.created_at)} />
      </div>
      {item.error_message && <p className="mt-3 rounded-2xl bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:bg-rose-950 dark:text-rose-300">{item.error_message}</p>}
    </div>
  );
}

function DetailMetric({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-3xl border border-slate-200 bg-white/75 p-4 dark:border-slate-800 dark:bg-slate-900/70">
      <div className="flex items-center gap-3">
        <span className="grid h-10 w-10 place-items-center rounded-2xl bg-slate-950 text-white dark:bg-white dark:text-slate-950">{icon}</span>
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.18em] text-slate-400">{label}</p>
          <p className="mt-1 break-all text-sm font-black text-slate-900 dark:text-white">{value}</p>
        </div>
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: ReactNode }) {
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

function NotFoundCard() {
  const { t } = useTranslation();

  return (
    <Card className="border-white/60 bg-white/80 shadow-sm backdrop-blur dark:border-slate-800 dark:bg-slate-950/65">
      <CardContent className="p-8 text-center">
        <p className="text-xl font-black">{t('monitorNotFound')}</p>
        <Button asChild className="mt-5 rounded-full">
          <Link to="/">{t('backToDashboard')}</Link>
        </Button>
      </CardContent>
    </Card>
  );
}

function DetailScreen({ message }: { message: string }) {
  return (
    <main className="grid min-h-screen place-items-center bg-slate-50 text-slate-700 dark:bg-slate-950 dark:text-slate-200">
      <div className="rounded-3xl border border-slate-200 bg-white px-6 py-5 shadow-xl dark:border-slate-800 dark:bg-slate-900">{message}</div>
    </main>
  );
}
