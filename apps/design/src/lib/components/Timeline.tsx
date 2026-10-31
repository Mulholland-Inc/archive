import type { ReactNode } from 'react';
import { cx } from '../cx';
import type { Hue } from '../hue';
import './Timeline.css';

export interface Entry {
	id: string;
	/** The branch, or the trunk. */
	name: string;
	/** What it did, in a sentence. */
	summary: string;
	meta?: string;
	/** Off the trunk, on its own lane. */
	branch?: boolean;
	/** Back on the trunk already. */
	merged?: boolean;
	hue?: Hue;
	trailing?: ReactNode;
}

const ROW = 72;
const LANE = 20;
const PAD = 10;

/* History as a story, not a list: one block per branch, newest first,
   with its name, a sentence of what it did and when. The graph beside
   it leaves the trunk where the branch started and comes back where it
   merged; an open branch ends at its node. */
export function Timeline({
	entries,
	onOpen,
	className
}: {
	entries: Entry[];
	onOpen?: (e: Entry) => void;
	className?: string;
}) {
	const height = entries.length * ROW;
	const x = (lane: number) => PAD + lane * LANE;
	const y = (i: number) => i * ROW + ROW / 2;
	const hueOf = (e: Entry) => e.hue ?? (e.branch ? 'teal' : 'navy');
	return (
		<div
			className={cx('timeline', className)}
			style={{ '--timeline-row': `${ROW}px` } as React.CSSProperties}
		>
			<svg className="timeline-graph" width={PAD * 2 + LANE} height={height} aria-hidden="true">
				<line x1={x(0)} y1={0} x2={x(0)} y2={height} className="timeline-trunk" />
				{entries.map((e, i) => {
					if (!e.branch) return null;
					const cx0 = x(0);
					const cx1 = x(1);
					const cy = y(i);
					const fork = `M ${cx0} ${cy + ROW / 2} C ${cx0} ${cy + ROW / 6} ${cx1} ${cy + ROW / 3} ${cx1} ${cy}`;
					const merge = `M ${cx1} ${cy} C ${cx1} ${cy - ROW / 3} ${cx0} ${cy - ROW / 6} ${cx0} ${cy - ROW / 2}`;
					return (
						<g key={e.id} style={{ stroke: `var(--hue-${hueOf(e)})` }}>
							<path d={fork} className="timeline-edge" />
							{e.merged && <path d={merge} className="timeline-edge" />}
						</g>
					);
				})}
				{entries.map((e, i) => (
					<rect
						key={e.id}
						x={x(e.branch ? 1 : 0) - 4}
						y={y(i) - 4}
						width="8"
						height="8"
						rx="2"
						className="timeline-node"
						style={{ fill: `var(--hue-${hueOf(e)})` }}
					/>
				))}
			</svg>
			<div className="timeline-rows">
				{entries.map((e) => {
					const Tag = onOpen ? 'button' : 'div';
					return (
						<Tag
							key={e.id}
							type={onOpen ? 'button' : undefined}
							className={cx('timeline-row', onOpen && 'timeline-row-clickable chrome')}
							onClick={onOpen ? () => onOpen(e) : undefined}
						>
							<span className="timeline-head">
								<span
									className="timeline-ref"
									style={
										{
											'--ref-hue': `var(--hue-${hueOf(e)})`,
											'--ref-soft': `var(--hue-${hueOf(e)}-soft)`
										} as React.CSSProperties
									}
								>
									{e.name}
								</span>
								{e.meta && <span className="timeline-meta">{e.meta}</span>}
								{e.trailing}
							</span>
							<span className="timeline-summary">{e.summary}</span>
						</Tag>
					);
				})}
			</div>
		</div>
	);
}
