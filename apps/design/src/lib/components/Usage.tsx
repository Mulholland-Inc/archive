import { useCallback, useMemo, useRef, useState } from 'react';
import { Gauge } from 'lucide-react';
import { Anchor, Layer, useDismiss, useFlip } from '../floating';
import { IconButton } from './Button';
import { Meter } from './Meter';
import './Usage.css';

/* How much of the plan is used, under one glyph at the sidebar's foot, on hover:
   a row per window, the session's and the week's, with the meter, the
   number and when it resets. */
export interface UsageRow {
	id: string;
	label: string;
	/** Of the window, 0 to 1. */
	value: number;
	/** When the window resets, as words. */
	resets?: string;
}

/** When a window resets, as words: in 2h, in 3d, tomorrow. */
export function resetsIn(iso: string | null | undefined, now = new Date()): string | undefined {
	if (!iso) return undefined;
	const ms = new Date(iso).getTime() - now.getTime();
	if (!Number.isFinite(ms) || ms <= 0) return 'now';
	const h = Math.round(ms / 3600000);
	if (h < 1) return `in ${Math.max(1, Math.round(ms / 60000))} min`;
	if (h < 24) return `in ${h}h`;
	return `in ${Math.round(h / 24)}d`;
}

export function Usage({ rows, error }: { rows: UsageRow[]; error?: string }) {
	const [open, setOpen] = useState(false);
	const anchor = useRef<HTMLButtonElement | null>(null);
	const layer = useRef<HTMLDivElement>(null);
	const timer = useRef<number | undefined>(undefined);
	const close = useCallback(() => setOpen(false), []);
	const inside = useMemo(() => [anchor, layer], []);
	useDismiss(open, close, inside);
	const side = useFlip(open, anchor, 'right-end');
	const show = () => {
		window.clearTimeout(timer.current);
		setOpen(true);
	};
	const hide = () => {
		window.clearTimeout(timer.current);
		timer.current = window.setTimeout(() => setOpen(false), 160);
	};
	const top = rows.reduce((m, r) => Math.max(m, r.value), 0);
	return (
		<Anchor>
			<IconButton
				ref={anchor}
				icon={Gauge}
				label="Usage"
				onMouseEnter={show}
				onMouseLeave={hide}
				onFocus={show}
				onBlur={hide}
				onClick={() => setOpen((o) => !o)}
			/>
			<Layer open={open} layerRef={layer} placement={side} className="usage chrome" role="dialog">
				<div className="usage-body" onMouseEnter={show} onMouseLeave={hide}>
					{error && <span className="usage-note">{error}</span>}
					{!error && rows.length === 0 && <span className="usage-note">Nothing yet.</span>}
					{rows.map((r) => (
						<div key={r.id} className="usage-row">
							<span className="usage-label">{r.label}</span>
							<span className="usage-meter">
								<Meter
									value={r.value}
									tone={r.value > 0.85 ? 'failed' : 'idle'}
									segments={10}
									label={`${r.label} used`}
								/>
								<span className="usage-number">{Math.round(r.value * 100)}%</span>
							</span>
							{r.resets && <span className="usage-resets">{r.resets}</span>}
						</div>
					))}
					{top > 0.85 && <span className="usage-note">Close to the limit.</span>}
				</div>
			</Layer>
		</Anchor>
	);
}
