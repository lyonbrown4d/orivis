import { Badge } from '@/components/ui/badge';
import type { Status } from '@/lib/api';
import { cn } from '@/lib/utils';

export function statusTone(status: Status) {
  switch (status) {
    case 'up':
      return {
        light: 'bg-emerald-500 shadow-emerald-500/30',
        badge: 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300'
      };
    case 'down':
      return {
        light: 'bg-rose-500 shadow-rose-500/30',
        badge: 'border-rose-300 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-300'
      };
    case 'degraded':
      return {
        light: 'bg-amber-400 shadow-amber-400/30',
        badge: 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300'
      };
    default:
      return {
        light: 'bg-slate-300 shadow-slate-300/30 dark:bg-slate-600 dark:shadow-slate-600/30',
        badge: 'border-slate-300 bg-slate-50 text-slate-600 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-300'
      };
  }
}

export function StatusBadge({ status, label }: { status: Status; label: string }) {
  return (
    <Badge className={cn('rounded-full px-2.5 py-1 text-xs font-semibold uppercase tracking-wide', statusTone(status).badge)}>
      {label}
    </Badge>
  );
}

export function StatusLights({
  lights,
  empty,
  formatTime
}: {
  lights: Array<{ monitor_name: string; status: Status; latency_ms: number; checked_at: string }>;
  empty: string;
  formatTime: (value?: string) => string;
}) {
  if (lights.length === 0) {
    return <p className="text-sm text-slate-500 dark:text-slate-400">{empty}</p>;
  }

  return (
    <div className="flex flex-wrap gap-1.5">
      {lights.map((light, index) => (
        <span
          key={`${light.monitor_name}-${light.checked_at}-${index}`}
          title={`${light.monitor_name} · ${light.status} · ${formatTime(light.checked_at)} · ${light.latency_ms}ms`}
          className={cn('h-4 w-8 rounded-full shadow-sm transition hover:scale-110 hover:shadow-md', statusTone(light.status).light)}
        />
      ))}
    </div>
  );
}
