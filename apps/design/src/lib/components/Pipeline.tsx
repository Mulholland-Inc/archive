import { cx } from '../cx';
import { Avatar } from './Avatar';
import type { Hue } from '../hue';
import './Pipeline.css';

export interface Stage {
	id: string;
	label: string;
}

export interface Engagement {
	id: string;
	/** The client. */
	name: string;
	/** The client's own emoji, the one its page wears. */
	emoji?: string;
	/** The banner its page wears. */
	cover?: string;
	town?: string;
	/** The industry the client is in: dental, accounting, ecommerce. */
	vertical?: string;
	/** The stage the work stands in. */
	stage: string;
	/** Which pass through that stage this is, where it is one we go
	 *  around more than once: a second demo, a third round. */
	iteration?: number;
	/** How far through the stage it is, 0 to 1; half when unsaid. */
	progress?: number;
	/** The day the engagement began, as an ISO date. */
	started?: string;
	/** Who is working on it, the one who owns it first. */
	crew: string[];
	hue?: Hue;
}

/* Every client on one page and how far each has come. The stages run
   across the top, a client is a row, and its track fills to the stage
   it stands in: solid behind it, filled as far as it has come where it
   is now, empty ahead. Where we have been round a stage more than
   once, its rectangle is cut into one slice a pass, the passes done
   solid and the one under way filled as far as it has come. Then how long we have been on the client,
   and everyone working on it. Press a stage on a row to move the
   client there. */
export function Pipeline({
	clients,
	stages,
	today,
	onOpen,
	onMove,
	className
}: {
	clients: Engagement[];
	stages: Stage[];
	today?: Date;
	onOpen?: (client: Engagement) => void;
	onMove?: (client: Engagement, stage: string) => void;
	className?: string;
}) {
	const now = today ?? new Date();
	return (
		<div className={cx('pipeline', className)}>
			<div className="pipeline-axis chrome">
				<span className="pipeline-gutter">Client</span>
				<span className="pipeline-track">
					{stages.map((s) => (
						<span key={s.id} className="pipeline-stage">
							{s.label}
						</span>
					))}
				</span>
				<span className="pipeline-span">Time on it</span>
				<span className="pipeline-crew">Working on it</span>
			</div>
			{clients.map((c) => {
				const at = stages.findIndex((s) => s.id === c.stage);
				return (
					<div key={c.id} className="pipeline-row">
						<button
							type="button"
							className="pipeline-gutter pipeline-name chrome"
							onClick={() => onOpen?.(c)}
						>
							<span className="pipeline-client">{c.name}</span>
							{c.town && <span className="pipeline-town">{c.town}</span>}
							<span className="pipeline-where">{stages[at]?.label}</span>
						</button>
						<span className="pipeline-track">
							{stages.map((s, i) => (
								<Step
									key={s.id}
									client={c}
									stage={s}
									standing={i < at ? 'behind' : i === at ? 'now' : 'ahead'}
									onMove={onMove}
								/>
							))}
						</span>
						<span className="pipeline-span">{c.started ? span(c.started, now) : ''}</span>
						<span className="pipeline-crew">
							<span className="pipeline-people">
								{c.crew.map((p, i) => (
									<Avatar
										key={p}
										name={p}
										size="sm"
										hue={i === 0 ? c.hue : undefined}
										className={cx(i > 0 && 'pipeline-person-behind')}
									/>
								))}
							</span>
							<span className="pipeline-names">{names(c.crew)}</span>
						</span>
					</div>
				);
			})}
		</div>
	);
}

/* Everyone on it under their marks, the last joined by an ampersand:
   Marijn & Lucie. */
function names(crew: string[]) {
	if (crew.length < 2) return crew[0] ?? '';
	return `${crew.slice(0, -1).join(', ')} & ${crew[crew.length - 1]}`;
}

const ordinals = [
	'first',
	'second',
	'third',
	'fourth',
	'fifth',
	'sixth',
	'seventh',
	'eighth',
	'ninth',
	'tenth'
];

/* How long we have been on the client, in the largest unit that still
   says something true: days for the first fortnight, then weeks, then
   months, then years. */
function span(started: string, today: Date) {
	const days = Math.max(0, Math.round((today.getTime() - new Date(started).getTime()) / 86400000));
	if (days < 1) return 'Today';
	if (days === 1) return 'A day';
	if (days < 14) return `${days} days`;
	if (days < 8 * 7) return `${Math.round(days / 7)} weeks`;
	if (days < 365) return `${Math.round(days / 30)} months`;
	const years = Math.round(days / 365);
	return years === 1 ? 'A year' : `${years} years`;
}

/* One stage on one client's track. It is a button only where a move is
   possible, so a read-only pipeline holds nothing to tab through. The
   stage it stands in is cut into a slice a pass where we have been
   round it more than once, so a third round reads as three. */
function Step({
	client,
	stage,
	standing,
	onMove
}: {
	client: Engagement;
	stage: Stage;
	standing: 'behind' | 'now' | 'ahead';
	onMove?: (client: Engagement, stage: string) => void;
}) {
	const passes =
		standing === 'now' && client.iteration && client.iteration > 1 ? client.iteration : 0;
	const className = cx(
		'pipeline-step',
		`pipeline-step-${standing}`,
		passes > 0 && 'pipeline-step-cut'
	);
	const where = passes
		? `${client.name} is in ${stage.label}, on the ${ordinals[passes - 1] ?? `${passes}th`} pass`
		: `${client.name} is in ${stage.label}`;
	const said = standing === 'now' ? where : `Move ${client.name} to ${stage.label}`;
	const done = Math.max(0, Math.min(1, client.progress ?? 0.5));
	const fill = <span className="pipeline-fill" style={{ width: `${done * 100}%` }} />;
	const body = passes
		? Array.from({ length: passes }, (_, i) => (
				<span key={i} className="pipeline-slice">
					{i === passes - 1 && fill}
				</span>
			))
		: standing === 'now'
			? fill
			: null;
	if (!onMove)
		return (
			<span className={className} title={standing === 'now' ? where : undefined}>
				{body}
			</span>
		);
	return (
		<button
			type="button"
			className={cx(className, 'chrome')}
			aria-label={said}
			aria-current={standing === 'now' ? 'step' : undefined}
			title={said}
			onClick={() => onMove(client, stage.id)}
		>
			{body}
		</button>
	);
}
