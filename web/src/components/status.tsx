import * as Tooltip from '@radix-ui/react-tooltip';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import type { Status } from '@/lib/api';
import { cn } from '@/lib/utils';

const maxStatusLights = 90;

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
	const { t } = useTranslation();

	if (lights.length === 0) {
		return (
			<div className="rounded-3xl border border-dashed border-slate-300 bg-slate-50/70 p-6 text-center text-sm text-slate-500 dark:border-slate-800 dark:bg-slate-900/50 dark:text-slate-400">
				{empty}
			</div>
		);
	}

	const visibleLights = lights.slice(-maxStatusLights);
	const summary = summarizeStatusLights(visibleLights);

	return (
		<Tooltip.Provider delayDuration={80} skipDelayDuration={0}>
			<div className="space-y-3 py-1">
				<div className="flex flex-wrap items-center justify-between gap-2 text-xs font-semibold text-slate-500 dark:text-slate-400">
					<span>{visibleLights.length} {t('checks')}</span>
					<div className="flex flex-wrap gap-2">
						<StatusMiniStat label={t('up')} value={summary.up} className="bg-emerald-500" />
						<StatusMiniStat label={t('down')} value={summary.down} className="bg-rose-500" />
						<StatusMiniStat label={t('unknown')} value={summary.unknown} className="bg-slate-400" />
					</div>
				</div>
				<div className="max-w-full overflow-x-hidden rounded-3xl border border-slate-200 bg-slate-100/70 p-2 dark:border-slate-800 dark:bg-slate-900/70">
					<div className="grid w-full grid-cols-[repeat(90,minmax(0,1fr))] gap-1">
						{visibleLights.map((light, index) => (
							<Tooltip.Root key={`${light.monitor_name}-${light.checked_at}-${index}`}>
								<Tooltip.Trigger asChild>
									<button
										type="button"
										aria-label={`${light.monitor_name} · ${light.status} · ${formatTime(light.checked_at)} · ${light.latency_ms}ms`}
										className={cn('h-8 w-full min-w-1 rounded-full shadow-sm transition hover:scale-y-110 hover:shadow-md focus:outline-none focus:ring-2 focus:ring-ring sm:h-10', statusTone(light.status).light)}
									/>
								</Tooltip.Trigger>
								<Tooltip.Portal>
									<Tooltip.Content
										side="top"
										align="center"
										sideOffset={8}
										className="z-50 max-w-80 rounded-2xl bg-slate-950 px-3 py-2 text-left text-xs font-semibold text-white shadow-2xl shadow-slate-900/20 dark:bg-white dark:text-slate-950"
									>
										<span className="block max-w-72 truncate">{light.monitor_name}</span>
										<span className="mt-1 block whitespace-nowrap font-medium opacity-75">
											{formatTime(light.checked_at)} · {light.status} · {light.latency_ms}ms
										</span>
										<Tooltip.Arrow className="fill-slate-950 dark:fill-white" />
									</Tooltip.Content>
								</Tooltip.Portal>
							</Tooltip.Root>
						))}
					</div>
				</div>
			</div>
		</Tooltip.Provider>
	);
}

function summarizeStatusLights(lights: Array<{ status: Status }>) {
	return lights.reduce(
		(out, light) => {
			if (light.status === 'up') out.up++;
			else if (light.status === 'down' || light.status === 'degraded') out.down++;
			else out.unknown++;
			return out;
		},
		{ up: 0, down: 0, unknown: 0 }
	);
}

function StatusMiniStat({ label, value, className }: { label: string; value: number; className: string }) {
	return (
		<span className="inline-flex items-center gap-1.5 rounded-full bg-white/75 px-2.5 py-1 text-slate-600 shadow-sm ring-1 ring-slate-200 dark:bg-slate-950/70 dark:text-slate-300 dark:ring-slate-800">
			<span className={cn('h-2 w-2 rounded-full', className)} />
			<span className="uppercase tracking-wide">{label}</span>
			<span className="font-black text-slate-900 dark:text-white">{value}</span>
		</span>
	);
}
