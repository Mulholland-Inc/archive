import { useId } from 'react';
import { cx } from '../cx';
import './Spinner.css';

export type SpinnerSize = 'sm' | 'md' | 'lg';

/* The house loader: a short stretch of the progress bar's wave, rolling
   in place, drawn in currentColor so it takes the tone of whatever it
   sits in. One language for work happening, the same as the bar. */
export function Spinner({
	size = 'md',
	stretch = false,
	title = 'Loading',
	className
}: {
	size?: SpinnerSize;
	/** Runs the width it is given, the wave at its own scale, cut at the end. */ stretch?: boolean;
	title?: string;
	className?: string;
}) {
	const id = useId();
	// Two periods across the box; the path holds a third one to roll in.
	// Stretched, the box is many periods wide and is cut where it ends.
	const period = 12;
	const amp = 2.4;
	const mid = 6;
	const width = period * (stretch ? 200 : 2);
	let d = `M ${-period} ${mid}`;
	for (let x = -period; x < width; x += period) {
		d += ` Q ${x + period / 4} ${mid - amp * 2} ${x + period / 2} ${mid} Q ${x + (period * 3) / 4} ${mid + amp * 2} ${x + period} ${mid}`;
	}
	return (
		<svg
			className={cx('spinner', `spinner-${size}`, stretch && 'spinner-stretch', className)}
			viewBox={`0 0 ${width} 12`}
			preserveAspectRatio={stretch ? 'xMinYMid slice' : 'xMidYMid meet'}
			role="img"
			aria-label={title}
		>
			<title>{title}</title>
			<defs>
				<clipPath id={id}>
					<rect x="0" y="0" width={width} height="12" />
				</clipPath>
			</defs>
			<g clipPath={`url(#${id})`}>
				<path
					className="spinner-wave"
					d={d}
					style={{ '--period': period } as React.CSSProperties}
				/>
			</g>
		</svg>
	);
}
