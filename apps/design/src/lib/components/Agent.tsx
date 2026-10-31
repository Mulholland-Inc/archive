import { useState, type ReactNode } from 'react';
import { AnimatePresence, motion } from 'motion/react';
import type { LucideIcon } from 'lucide-react';
import {
	Check,
	ChevronDown,
	ExternalLink,
	FilePen,
	FileSearch,
	Globe,
	SquareTerminal,
	Wrench,
	X
} from 'lucide-react';
import { cx } from '../cx';
import { Button } from './Button';
import { Icon } from './Icon';
import { Input } from './Input';
import { Spinner } from './Spinner';
import { Tooltip } from './Tooltip';
import './Agent.css';

/* The parts of a coding session that are not words: what the agent did,
   what it asks, what it plans, and how the session stands. Nothing here
   opens a file; an action is a sentence about what changed, with the
   file and its counts as the detail. */

export type ActionKind = 'edit' | 'run' | 'search' | 'fetch' | 'tool';
export type ActionState = 'running' | 'done' | 'failed';

const glyph: Record<ActionKind, LucideIcon> = {
	edit: FilePen,
	run: SquareTerminal,
	search: FileSearch,
	fetch: Globe,
	tool: Wrench
};

export function Action({
	kind,
	state = 'done',
	title,
	detail,
	className
}: {
	kind: ActionKind;
	state?: ActionState;
	title: string;
	detail?: string;
	className?: string;
}) {
	return (
		<div className={cx('action', `action-${state}`, className)}>
			<span className="action-mark">
				{state === 'running' ? (
					<Spinner size="sm" />
				) : state === 'failed' ? (
					<Icon icon={X} size="sm" />
				) : (
					<Icon icon={glyph[kind]} size="sm" />
				)}
			</span>
			<span className="action-title">{title}</span>
			{detail && <span className="action-detail">{detail}</span>}
		</div>
	);
}

/* Several actions, folded to one line until they are wanted. While one
   still runs, the wave runs from the summary to the chevron. */
export function Actions({
	summary,
	busy = false,
	children,
	defaultOpen = false
}: {
	summary: string;
	busy?: boolean;
	children: ReactNode;
	defaultOpen?: boolean;
}) {
	const [open, setOpen] = useState(defaultOpen);
	return (
		<div className="actions">
			<button
				type="button"
				className="actions-head chrome"
				onClick={() => setOpen((o) => !o)}
				aria-expanded={open}
			>
				<span className="actions-summary">{summary}</span>
				{busy && <Spinner size="sm" stretch title="Working" className="actions-wave" />}
				<motion.span
					className="action-fold"
					animate={{ rotate: open ? 180 : 0 }}
					transition={{ duration: 0.2 }}
				>
					<Icon icon={ChevronDown} size="sm" />
				</motion.span>
			</button>
			<AnimatePresence initial={false}>
				{open && (
					<motion.div
						className="action-body-fold"
						initial={{ height: 0, opacity: 0 }}
						animate={{ height: 'auto', opacity: 1 }}
						exit={{ height: 0, opacity: 0 }}
						transition={{ duration: 0.22, ease: [0.2, 0, 0, 1] }}
					>
						<div className="actions-list">{children}</div>
					</motion.div>
				)}
			</AnimatePresence>
		</div>
	);
}

/* What the agent thought, folded to how long it took. */
export function Thinking({ seconds, children }: { seconds?: number; children?: ReactNode }) {
	const [open, setOpen] = useState(false);
	const label = seconds === undefined ? 'Thinking' : `Thought for ${seconds}s`;
	return (
		<div className="thinking">
			<button
				type="button"
				className="thinking-head chrome"
				onClick={() => children && setOpen((o) => !o)}
			>
				{seconds === undefined && <Spinner size="sm" />}
				<span>{label}</span>
			</button>
			<AnimatePresence initial={false}>
				{open && children && (
					<motion.div
						className="action-body-fold"
						initial={{ height: 0, opacity: 0 }}
						animate={{ height: 'auto', opacity: 1 }}
						exit={{ height: 0, opacity: 0 }}
						transition={{ duration: 0.22, ease: [0.2, 0, 0, 1] }}
					>
						<div className="thinking-body">{children}</div>
					</motion.div>
				)}
			</AnimatePresence>
		</div>
	);
}

/* The agent asks before it does something: allow once, always for this
   session, or not. */
export function Ask({
	title,
	detail,
	onAllow,
	onAlways,
	onDeny,
	answered
}: {
	title: string;
	detail?: string;
	onAllow: () => void;
	onAlways?: () => void;
	onDeny: () => void;
	answered?: 'allowed' | 'always' | 'denied';
}) {
	return (
		<div className={cx('ask', answered && 'ask-answered')}>
			<div className="ask-text">
				<span className="ask-title">{title}</span>
				{detail && <span className="ask-detail">{detail}</span>}
			</div>
			{answered ? (
				<span className="ask-outcome">
					{answered === 'denied'
						? 'Not allowed'
						: answered === 'always'
							? 'Allowed for the session'
							: 'Allowed'}
				</span>
			) : (
				<div className="ask-actions">
					<Button variant="primary" size="sm" onClick={onAllow}>
						Allow
					</Button>
					{onAlways && (
						<Button size="sm" onClick={onAlways}>
							Always
						</Button>
					)}
					<Button variant="ghost" size="sm" onClick={onDeny}>
						Deny
					</Button>
				</div>
			)}
		</div>
	);
}

export type PlanState = 'todo' | 'doing' | 'done';

/* The agent's plan for the turn, each line with where it stands. */
export function Plan({ items }: { items: Array<{ id: string; text: string; state: PlanState }> }) {
	return (
		<div className="plan">
			{items.map((it) => (
				<div key={it.id} className={cx('plan-item', `plan-${it.state}`)}>
					<span className="plan-mark">
						{it.state === 'doing' ? (
							<Spinner size="sm" />
						) : it.state === 'done' ? (
							<Icon icon={Check} size="sm" />
						) : (
							<span className="plan-dash" />
						)}
					</span>
					<span className="plan-text">{it.text}</span>
				</div>
			))}
		</div>
	);
}

export type ElicitKind = 'options' | 'multi' | 'text' | 'number' | 'boolean';

/* One thing the agent asks for. Options are buttons; a lone options
   field answers on the press. */
export interface ElicitField {
	key: string;
	label: string;
	/** The question itself, where the label is only its heading. */
	description?: string;
	kind: ElicitKind;
	options?: Array<{ value: string; label: string; hint?: string }>;
	required?: boolean;
}

/* The agent's question, where the composer was: its words, then what it
   asks for as buttons or fields, and a way to decline. A link to open
   sits first when the answer happens elsewhere (a sign-in). */
export function Elicit({
	message,
	fields,
	onAnswer,
	onDecline,
	open
}: {
	message: string;
	fields: ElicitField[];
	onAnswer: (values: Record<string, unknown>) => void;
	onDecline: () => void;
	open?: { label: string; onOpen: () => void };
}) {
	const [values, setValues] = useState<Record<string, unknown>>({});
	const lone = fields.length === 1 && fields[0].kind === 'options';
	const set = (key: string, v: unknown) => setValues((x) => ({ ...x, [key]: v }));
	const ready = fields.every((f) => !f.required || values[f.key] !== undefined);
	return (
		<div className="elicit">
			<span className="elicit-message">{message}</span>
			{open && (
				<Button size="sm" icon={ExternalLink} onClick={open.onOpen}>
					{open.label}
				</Button>
			)}
			{fields.map((f) => (
				<div key={f.key} className="elicit-field">
					{!lone && <span className="elicit-label chrome">{f.label}</span>}
					{f.description && <span className="elicit-question">{f.description}</span>}
					{f.kind === 'options' && (
						<div className="elicit-options">
							{f.options?.map((o) => {
								const b = (
									<Button
										key={o.value}
										size="sm"
										variant={values[f.key] === o.value ? 'primary' : 'secondary'}
										onClick={() => (lone ? onAnswer({ [f.key]: o.value }) : set(f.key, o.value))}
									>
										{o.label}
									</Button>
								);
								return o.hint ? (
									<Tooltip key={o.value} label={o.hint}>
										{b}
									</Tooltip>
								) : (
									b
								);
							})}
						</div>
					)}
					{f.kind === 'multi' && (
						<div className="elicit-options">
							{f.options?.map((o) => {
								const on = ((values[f.key] as string[] | undefined) ?? []).includes(o.value);
								return (
									<Button
										key={o.value}
										size="sm"
										variant={on ? 'primary' : 'secondary'}
										onClick={() =>
											set(
												f.key,
												on
													? ((values[f.key] as string[]) ?? []).filter((v) => v !== o.value)
													: [...((values[f.key] as string[] | undefined) ?? []), o.value]
											)
										}
									>
										{o.label}
									</Button>
								);
							})}
						</div>
					)}
					{f.kind === 'boolean' && (
						<div className="elicit-options">
							<Button
								size="sm"
								variant={values[f.key] === true ? 'primary' : 'secondary'}
								onClick={() => set(f.key, true)}
							>
								Yes
							</Button>
							<Button
								size="sm"
								variant={values[f.key] === false ? 'primary' : 'secondary'}
								onClick={() => set(f.key, false)}
							>
								No
							</Button>
						</div>
					)}
					{(f.kind === 'text' || f.kind === 'number') && (
						<Input
							size="sm"
							type={f.kind === 'number' ? 'number' : 'text'}
							value={(values[f.key] as string | undefined) ?? ''}
							onChange={(e) =>
								set(f.key, f.kind === 'number' ? Number(e.target.value) : e.target.value)
							}
						/>
					)}
				</div>
			))}
			<div className="elicit-actions">
				{!lone && (
					<Button variant="primary" size="sm" disabled={!ready} onClick={() => onAnswer(values)}>
						{fields.length ? 'Done' : 'Continue'}
					</Button>
				)}
				<Button variant="ghost" size="sm" onClick={onDecline}>
					Decline
				</Button>
			</div>
		</div>
	);
}
