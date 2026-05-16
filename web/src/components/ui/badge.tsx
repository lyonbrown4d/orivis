import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const badgeVariants = cva('inline-flex items-center rounded-full px-2.5 py-1 text-xs font-bold ring-1 ring-inset', {
  variants: {
    variant: {
      default: 'bg-slate-100 text-slate-700 ring-slate-200 dark:bg-white/10 dark:text-slate-200 dark:ring-white/10',
      up: 'bg-emerald-100 text-emerald-700 ring-emerald-200',
      down: 'bg-rose-100 text-rose-700 ring-rose-200',
      degraded: 'bg-amber-100 text-amber-700 ring-amber-200',
      unknown: 'bg-slate-100 text-slate-600 ring-slate-200 dark:bg-white/10 dark:text-slate-300 dark:ring-white/10'
    }
  },
  defaultVariants: { variant: 'default' }
});

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant, className }))} {...props} />;
}
