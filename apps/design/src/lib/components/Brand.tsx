import { cx } from '../cx';
import './Brand.css';

/* The marks of the two doors, in their own colours. Lucide carries no
   brands by design, so these are the marks themselves on the 24 grid. */
export function Brand({ name, className }: { name: 'google' | 'microsoft'; className?: string }) {
	if (name === 'microsoft')
		return (
			<svg className={cx('icon brand', className)} viewBox="0 0 24 24" aria-hidden="true">
				<rect x="2" y="2" width="9.5" height="9.5" fill="var(--brand-microsoft-red)" />
				<rect x="12.5" y="2" width="9.5" height="9.5" fill="var(--brand-microsoft-green)" />
				<rect x="2" y="12.5" width="9.5" height="9.5" fill="var(--brand-microsoft-blue)" />
				<rect x="12.5" y="12.5" width="9.5" height="9.5" fill="var(--brand-microsoft-yellow)" />
			</svg>
		);
	return (
		<svg className={cx('icon brand', className)} viewBox="0 0 24 24" aria-hidden="true">
			<path
				fill="var(--brand-google-blue)"
				d="M21.6 12.23c0-.68-.06-1.36-.19-2.02H12v3.83h5.4a4.62 4.62 0 0 1-2 3.03v2.5h3.22c1.89-1.74 2.98-4.3 2.98-7.34z"
			/>
			<path
				fill="var(--brand-google-green)"
				d="M12 22c2.7 0 4.97-.9 6.62-2.43l-3.22-2.5c-.9.6-2.04.96-3.4.96-2.6 0-4.81-1.76-5.6-4.13H3.07v2.58A10 10 0 0 0 12 22z"
			/>
			<path
				fill="var(--brand-google-yellow)"
				d="M6.4 13.9a6 6 0 0 1 0-3.8V7.52H3.07a10 10 0 0 0 0 8.96L6.4 13.9z"
			/>
			<path
				fill="var(--brand-google-red)"
				d="M12 5.97c1.47 0 2.79.5 3.83 1.5l2.86-2.86C16.96 3 14.7 2 12 2a10 10 0 0 0-8.93 5.52L6.4 10.1c.79-2.37 3-4.13 5.6-4.13z"
			/>
		</svg>
	);
}
