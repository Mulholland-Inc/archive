import { cx } from '../cx';
import './Meter.css';

export type MeterTone = 'running' | 'done' | 'failed' | 'idle';

/* A share of one as a row of bars. The lit ones fill from the left, the
   last lit one breathes while the thing is still running. */
export function Meter({
	value,
	tone = 'running',
	segments = 24,
	label,
	className
}: {
	value: number;
	tone?: MeterTone;
	segments?: number;
	label?: string;
	className?: string;
}) {
	const pct = Math.max(0, Math.min(1, value));
	const lit = Math.round(pct * segments);
	return (
		<span
			className={cx('meter', `meter-${tone}`, className)}
			role="progressbar"
			aria-valuenow={Math.round(pct * 100)}
			aria-valuemin={0}
			aria-valuemax={100}
			aria-label={label}
		>
			{Array.from({ length: segments }, (_, i) => (
				<span
					key={i}
					className={cx(
						'meter-bar',
						i < lit && 'meter-bar-on',
						tone === 'running' && i === lit - 1 && 'meter-bar-head'
					)}
					style={{ '--i': i } as React.CSSProperties}
				/>
			))}
		</span>
	);
}
