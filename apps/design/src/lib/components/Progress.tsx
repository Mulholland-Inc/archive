import { useId } from 'react';
import { cx } from '../cx';
import './Progress.css';

/* A bar that fills to a share of one. The filled part is a wave that
   keeps rolling while there is still work; the rest is a flat line with
   a small gap before it. Flat is there for the quiet places. */
export function Progress({
	value,
	label,
	wave = true,
	className
}: {
	value: number;
	label?: string;
	wave?: boolean;
	className?: string;
}) {
	const pct = Math.max(0, Math.min(1, value));
	const id = useId();
	if (!wave)
		return (
			<div
				className={cx('progress progress-flat', className)}
				role="progressbar"
				aria-valuenow={Math.round(pct * 100)}
				aria-valuemin={0}
				aria-valuemax={100}
				aria-label={label}
			>
				<span className="progress-bar" style={{ width: `${pct * 100}%` }} />
			</div>
		);
	// The wave is drawn in a 0..1000 by 0..12 box; one period is 40 units,
	// two full widths of it slide left by one period on a loop.
	const period = 40;
	const amp = 3;
	const mid = 6;
	const width = 1000;
	let d = `M -${width} ${mid}`;
	for (let x = -width; x <= width; x += period) {
		d += ` Q ${x + period / 4} ${mid - amp * 2} ${x + period / 2} ${mid} Q ${x + (period * 3) / 4} ${mid + amp * 2} ${x + period} ${mid}`;
	}
	const filled = pct * width;
	const gap = 12;
	const done = pct >= 1;
	return (
		<div
			className={cx('progress', done && 'progress-done', className)}
			role="progressbar"
			aria-valuenow={Math.round(pct * 100)}
			aria-valuemin={0}
			aria-valuemax={100}
			aria-label={label}
		>
			<svg
				className="progress-wave"
				viewBox={`0 0 ${width} 12`}
				preserveAspectRatio="none"
				aria-hidden="true"
			>
				<defs>
					<clipPath id={id}>
						<rect x="0" y="0" width={filled} height="12" />
					</clipPath>
				</defs>
				{filled < width - gap && (
					<line
						className="progress-rest"
						x1={Math.min(width, filled + gap)}
						y1={mid}
						x2={width}
						y2={mid}
					/>
				)}
				<g clipPath={`url(#${id})`}>
					<path
						className={cx('progress-fill', !done && 'progress-fill-rolling')}
						d={d}
						style={{ '--period': period } as React.CSSProperties}
					/>
				</g>
			</svg>
		</div>
	);
}
