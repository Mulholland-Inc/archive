import type { ReactNode } from 'react';
import { Bot, Brain, ChevronDown, Eraser, FolderOpen, GitFork, Zap } from 'lucide-react';
import {
	Action,
	Actions,
	Ask,
	Elicit,
	Plan,
	Thinking,
	type ActionKind,
	type ElicitField
} from './Agent';
import { Button, IconButton } from './Button';
import { Composer, type Command } from './Composer';
import { Empty } from './Empty';
import { Icon } from './Icon';
import { Markdown } from './Markdown';
import { Menu, type MenuEntry } from './Menu';
import { Message, Thread } from './Message';
import { Meter } from './Meter';
import { Tooltip } from './Tooltip';
import './Column.css';

/* A coding session as a column: its head (the repo, the branch, the
   context, the ways to open a folder, fork, and start over), its turns as the agent
   streams them, and the composer at the foot with one picker for the
   agent, the model and the effort. What the agent does is a folded run
   of steps; what it asks takes the composer's place. */

/* One piece of an agent's turn, in the order it came. */
export type Block =
	| { kind: 'text'; text: string }
	| { kind: 'thought'; text: string; seconds?: number }
	| {
			kind: 'tool';
			id: string;
			title: string;
			action: ActionKind;
			state: 'running' | 'done' | 'failed';
			detail?: string;
	  }
	| { kind: 'plan'; entries: Array<{ id: string; text: string; state: 'todo' | 'doing' | 'done' }> }
	| {
			kind: 'ask';
			id: string;
			title: string;
			detail?: string;
			options: Array<{ optionId: string; name: string; kind: string }>;
			answered?: 'allowed' | 'always' | 'denied';
	  };

export interface Turn {
	id: number;
	role: 'user' | 'agent';
	text?: string;
	pending?: boolean;
	/** Still arriving: its last words come in at an even pace. */
	streaming?: boolean;
	meta?: string;
	blocks?: Block[];
	/** A rehearsed turn carries its verbs as steps. */
	steps?: Array<{ kind: ActionKind; title: string; detail?: string }>;
}

export interface Choice {
	value: string;
	label: string;
}

export interface Attached {
	id: string;
	name: string;
	path: string;
}

/* What the agent asks, waiting in the composer's place. */
export interface Question {
	message: string;
	fields: ElicitField[];
	open?: { label: string; onOpen: () => void };
	answer: (values: Record<string, unknown>) => void;
	decline: () => void;
}

/* Everything the column shows and does, whichever session drives it. */
export interface SessionView {
	id: string;
	repo: string;
	cwd: string | null;
	branch: string;
	sessionId: string | null;
	agent: string | null;
	setAgent: (v: string | null) => void;
	model: string | null;
	models: Choice[];
	setModel: (v: string | null) => void;
	effort: string | null;
	efforts: Choice[];
	setEffort: (v: string | null) => void;
	context: number;
	draft: string;
	setDraft: (v: string) => void;
	turns: Turn[];
	working: boolean;
	queue: Array<{ id: string; text: string }>;
	commands: Command[];
	attachments: Attached[];
	attach: (paths: string[]) => void;
	unattach: (id: string) => void;
	question?: Question;
	canFork: boolean;
	fork: () => Promise<string | null>;
	send: () => void;
	stop: () => void;
	clear: () => void;
	unqueue: (id: string) => void;
	answer: (askId: string, optionId: string) => void;
	error?: string;
}

export const agentKinds: Choice[] = [
	{ value: 'claude', label: 'Claude Code' },
	{ value: 'codex', label: 'Codex' },
	{ value: 'gemini', label: 'Gemini' }
];

/* The agent's turn, as the protocol streamed it: its words, what it
   thought, what it did (a run of steps folded), what it asks, and its
   plan, in order. */
export function AgentTurn({
	turn: t,
	onAnswer
}: {
	turn: Turn;
	onAnswer: (askId: string, optionId: string) => void;
}) {
	if (!t.blocks) {
		return (
			<>
				{t.steps && (
					<Actions summary={`${t.steps.length} steps`}>
						{t.steps.map((st, i) => (
							<Action key={i} kind={st.kind} title={st.title} detail={st.detail} />
						))}
					</Actions>
				)}
				{t.text && <Markdown>{t.text}</Markdown>}
			</>
		);
	}
	const out: ReactNode[] = [];
	let run: Array<Extract<Block, { kind: 'tool' }>> = [];
	const flush = () => {
		if (run.length === 0) return;
		const tools = run;
		run = [];
		const busy = tools.some((x) => x.state === 'running');
		out.push(
			<Actions key={`run-${tools[0].id}`} summary={`${tools.length} steps`} busy={busy}>
				{tools.map((x) => (
					<Action key={x.id} kind={x.action} state={x.state} title={x.title} detail={x.detail} />
				))}
			</Actions>
		);
	};
	const last = t.blocks.length - 1;
	t.blocks.forEach((b, i) => {
		if (b.kind === 'tool') return run.push(b);
		flush();
		if (b.kind === 'text')
			out.push(
				<Markdown key={i} stream={Boolean(t.streaming) && i === last}>
					{b.text}
				</Markdown>
			);
		else if (b.kind === 'thought')
			out.push(
				<Thinking key={i} seconds={b.seconds}>
					{b.text}
				</Thinking>
			);
		else if (b.kind === 'plan') out.push(<Plan key={i} items={b.entries} />);
		else if (b.kind === 'ask') {
			const of = (kind: string) => b.options.find((o) => o.kind === kind);
			const allow = of('allow_once') ?? b.options[0];
			const always = of('allow_always');
			const deny = of('reject_once') ?? of('reject_always');
			out.push(
				<Ask
					key={b.id}
					title={b.title}
					detail={b.detail}
					answered={b.answered}
					onAllow={() => onAnswer(b.id, allow.optionId)}
					onAlways={always ? () => onAnswer(b.id, always.optionId) : undefined}
					onDeny={() => deny && onAnswer(b.id, deny.optionId)}
				/>
			);
		}
	});
	flush();
	return <>{out}</>;
}

export function AgentColumn({
	session: s,
	sessions,
	onSwitch,
	local = false,
	onOpen,
	onAgent,
	onFork,
	onAttach,
	agents = agentKinds,
	className
}: {
	session: SessionView;
	/** With more than one, the branch is a menu of them. */ sessions?: SessionView[];
	onSwitch?: (id: string) => void;
	/** On a machine with folders: the ways to open, fork and close show. */ local?: boolean;
	onOpen?: () => void;
	onAgent?: (agent: string) => void;
	onFork?: () => void;
	onAttach?: () => void;
	agents?: Choice[];
	className?: string;
}) {
	const branch =
		sessions && sessions.length > 1 && onSwitch ? (
			<Menu
				trigger={
					<button type="button" className="agent-branch agent-branch-pick chrome">
						<span>{s.branch || s.repo}</span>
						<Icon icon={ChevronDown} size="sm" />
					</button>
				}
				items={sessions.map((x) => ({
					id: x.id,
					label: x.branch || x.repo,
					hint: x.working ? 'working' : undefined
				}))}
				onSelect={onSwitch}
			/>
		) : (
			<span className="agent-branch">{s.branch}</span>
		);
	const pickAgent = (v: string | null) => {
		if (!v) return;
		s.setAgent(v);
		onAgent?.(v);
	};
	const label = (list: Choice[], v: string | null) => list.find((c) => c.value === v)?.label;
	const picks = [label(agents, s.agent), label(s.models, s.model), label(s.efforts, s.effort)]
		.filter(Boolean)
		.join(', ');
	const pickItems: MenuEntry[] = [
		{
			id: 'agent',
			label: 'Agent',
			icon: Bot,
			hint: label(agents, s.agent),
			items: agents.map((c) => ({
				id: `agent:${c.value}`,
				label: c.label,
				checked: c.value === s.agent
			}))
		},
		{
			id: 'model',
			label: 'Model',
			icon: Brain,
			hint: label(s.models, s.model),
			items: s.models.map((c) => ({
				id: `model:${c.value}`,
				label: c.label,
				checked: c.value === s.model
			}))
		},
		{
			id: 'effort',
			label: 'Effort',
			icon: Zap,
			hint: label(s.efforts, s.effort),
			items: s.efforts.map((c) => ({
				id: `effort:${c.value}`,
				label: c.label,
				checked: c.value === s.effort
			}))
		}
	];
	const pick = (id: string) => {
		const [what, value] = id.split(':');
		if (what === 'agent') pickAgent(value);
		else if (what === 'model') s.setModel(value);
		else if (what === 'effort') s.setEffort(value);
	};
	const newest = s.turns[s.turns.length - 1];
	const tail =
		newest?.blocks?.reduce((n, b) => n + ('text' in b ? b.text.length : 1), 0) ??
		newest?.text?.length ??
		0;
	return (
		<div className={['agent', className].filter(Boolean).join(' ')}>
			<div className="agent-head chrome" data-tauri-drag-region>
				<div className="agent-where" data-tauri-drag-region>
					{local && onOpen && (
						<Tooltip label="Open a repository">
							<IconButton icon={FolderOpen} label="Open a repository" size="sm" onClick={onOpen} />
						</Tooltip>
					)}
					<span className="agent-repo">{s.repo}</span>
					{branch}
					{local && s.canFork && s.sessionId && onFork && (
						<Tooltip label="Fork into another column">
							<IconButton icon={GitFork} label="Fork the session" size="sm" onClick={onFork} />
						</Tooltip>
					)}
					<span className="agent-context">
						<Meter
							value={s.context}
							tone={s.context > 0.85 ? 'failed' : 'idle'}
							segments={10}
							label="Context used"
						/>
						<span>{Math.ceil(s.context * 100)}%</span>
					</span>
					<Tooltip label="Start over: stop, forget the context, stay in the folder">
						<IconButton icon={Eraser} label="Start over" size="sm" onClick={s.clear} />
					</Tooltip>
				</div>
			</div>
			<div className="agent-thread">
				{local && !s.cwd ? (
					<Empty
						title="No repository open"
						action={onOpen && <Button onClick={onOpen}>Open a repository</Button>}
					>
						Pick the folder the agent should work in.
					</Empty>
				) : (
					<Thread count={s.turns.length} tail={tail}>
						{s.turns.map((t) =>
							t.role === 'user' ? (
								<Message key={t.id} role="user">
									{t.text}
								</Message>
							) : (
								<Message key={t.id} role="agent" pending={t.pending} meta={t.meta}>
									<AgentTurn turn={t} onAnswer={s.answer} />
								</Message>
							)
						)}
						{s.error && <span className="agent-error">{s.error}</span>}
					</Thread>
				)}
			</div>
			<div className="agent-foot">
				{s.question ? (
					<Elicit
						message={s.question.message}
						fields={s.question.fields}
						open={s.question.open}
						onAnswer={s.question.answer}
						onDecline={s.question.decline}
					/>
				) : (
					<Composer
						value={s.draft}
						onChange={s.setDraft}
						onSend={s.send}
						onStop={s.stop}
						working={s.working}
						queued={s.queue}
						onUnqueue={s.unqueue}
						disabled={local && !s.cwd}
						commands={s.commands}
						onAttach={onAttach}
						attachments={s.attachments}
						onRemoveAttachment={s.unattach}
						placeholder="Tell it what to change, or type a slash"
						leading={
							<Menu
								trigger={
									<button
										type="button"
										className="agent-picks chrome"
										aria-label={picks}
										title={picks}
									>
										<Icon icon={Bot} size="sm" />
									</button>
								}
								items={pickItems}
								onSelect={pick}
								placement="top-start"
								stay
							/>
						}
					/>
				)}
			</div>
		</div>
	);
}
