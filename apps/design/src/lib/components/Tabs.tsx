import { motion } from 'motion/react';
import type { LucideIcon } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Tabs.css';

export interface Tab<T extends string> {
	id: T;
	label: string;
	icon?: LucideIcon;
}

/* Tabs with one underline that travels. */
export function Tabs<T extends string>({
	tabs,
	value,
	onChange,
	className
}: {
	tabs: Tab<T>[];
	value: T;
	onChange: (id: T) => void;
	className?: string;
}) {
	return (
		<div className={cx('tabs chrome', className)} role="tablist">
			{tabs.map((t) => (
				<button
					key={t.id}
					type="button"
					role="tab"
					aria-selected={t.id === value}
					className={cx('tab', t.id === value && 'tab-active')}
					onClick={() => onChange(t.id)}
				>
					{t.icon && <Icon icon={t.icon} />}
					<span>{t.label}</span>
					{t.id === value && (
						<motion.span
							className="tab-indicator"
							layoutId="tab-indicator"
							transition={{ type: 'spring', stiffness: 500, damping: 40 }}
						/>
					)}
				</button>
			))}
		</div>
	);
}

/* The same choice, as a segmented control: one thumb sliding in a track. */
export function Segmented<T extends string>({
	tabs,
	value,
	onChange,
	className
}: {
	tabs: Tab<T>[];
	value: T;
	onChange: (id: T) => void;
	className?: string;
}) {
	return (
		<div className={cx('segmented chrome', className)} role="tablist">
			{tabs.map((t) => (
				<button
					key={t.id}
					type="button"
					role="tab"
					aria-selected={t.id === value}
					className={cx('segment', t.id === value && 'segment-active')}
					onClick={() => onChange(t.id)}
				>
					{t.id === value && (
						<motion.span
							className="segment-thumb"
							layoutId="segment-thumb"
							transition={{ type: 'spring', stiffness: 500, damping: 40 }}
						/>
					)}
					{t.icon && <Icon icon={t.icon} />}
					<span className="segment-label">{t.label}</span>
				</button>
			))}
		</div>
	);
}
