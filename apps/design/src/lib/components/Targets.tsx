import type { ReactNode } from 'react';
import { cx } from '../cx';
import { Avatar } from './Avatar';
import { Badge } from './Badge';
import { Meter } from './Meter';
import { Popover } from './Popover';
import './Targets.css';

export interface Target {
	id: string;
	name: string;
	stage: string;
	score: number;
	/** Why the score, in a sentence. */
	why?: string;
	fit?: number;
	timing?: number;
	connection?: number;
	connections: string[];
	/** The industry it is in: dental, accounting, ecommerce. */
	vertical: string;
}

/* A score out of a hundred as a small meter and its number. Press it
   and the reasoning opens: the sentence, and the three parts it is made
   of. */
export function Score({
	value,
	why,
	parts
}: {
	value: number;
	why?: string;
	parts?: Array<{ label: string; value: number }>;
}) {
	const tone = value >= 80 ? 'done' : value >= 60 ? 'running' : 'idle';
	const word = value >= 80 ? 'Strong' : value >= 60 ? 'Warm' : 'Early';
	const face = (
		<button type="button" className="score chrome" aria-label={`Score ${value}`}>
			<Meter value={value / 100} tone={tone} segments={5} />
			<span className="score-number">{value}</span>
		</button>
	);
	if (!why && !parts) return face;
	return (
		<Popover trigger={face}>
			<div className="score-detail">
				<div className="score-head">
					<span className="score-big">{value}</span>
					<Badge hue={value >= 80 ? 'green' : value >= 60 ? 'amber' : 'gray'}>{word}</Badge>
				</div>
				{why && <p className="score-why">{why}</p>}
				{parts && (
					<div className="score-parts">
						{parts.map((p) => (
							<div key={p.label} className="score-part">
								<span className="score-part-label">{p.label}</span>
								<span className="score-part-value">
									<Meter
										value={p.value / 100}
										tone={p.value >= 80 ? 'done' : p.value >= 60 ? 'running' : 'idle'}
										segments={5}
									/>
									<span>{p.value}</span>
								</span>
							</div>
						))}
					</div>
				)}
			</div>
		</Popover>
	);
}

/* The accounts we are going after, grouped by where they stand: the
   name, the score, who we know there, and the vertical it is in. */
export function Targets({
	targets,
	stages,
	onOpen,
	className
}: {
	targets: Target[];
	stages: string[];
	onOpen?: (t: Target) => void;
	className?: string;
}) {
	return (
		<div className={cx('targets', className)}>
			<div className="targets-head chrome">
				<span>Account</span>
				<span>Score</span>
				<span>Connections</span>
				<span>Vertical</span>
			</div>
			{stages.map((stage) => {
				const rows = targets.filter((t) => t.stage === stage);
				if (!rows.length) return null;
				return (
					<div key={stage} className="targets-group">
						<div className="targets-stage chrome">{stage}</div>
						{rows.map((t) => (
							<div key={t.id} className="target">
								<button type="button" className="target-name chrome" onClick={() => onOpen?.(t)}>
									<Avatar name={t.name} size="sm" />
									<span>{t.name}</span>
								</button>
								<span className="target-score">
									<Score
										value={t.score}
										why={t.why}
										parts={
											t.fit !== undefined
												? [
														{ label: 'Fit', value: t.fit },
														{ label: 'Timing', value: t.timing ?? 0 },
														{ label: 'Connection', value: t.connection ?? 0 }
													]
												: undefined
										}
									/>
								</span>
								<span className="target-people">
									{t.connections.slice(0, 3).map((p, i) => (
										<Avatar
											key={p}
											name={p}
											size="sm"
											className={cx(i > 0 && 'target-person-behind')}
										/>
									))}
								</span>
								<span className="target-vertical">
									<Badge hue="gray">{t.vertical}</Badge>
								</span>
							</div>
						))}
					</div>
				);
			})}
		</div>
	);
}

export function TargetsBar({ children }: { children: ReactNode }) {
	return <div className="targets-bar">{children}</div>;
}
