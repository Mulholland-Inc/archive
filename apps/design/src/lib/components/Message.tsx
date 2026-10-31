import { useEffect, useRef, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Check, Copy, RotateCcw } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import { IconButton } from './Button';
import { Spinner } from './Spinner';
import { Tooltip } from './Tooltip';
import './Message.css';

export type MessageRole = 'user' | 'agent';

/* One turn. The person's words sit in a tint on the right; the agent's are
   plain text on the page, with its verbs above them. */
export function Message({
	role,
	author,
	time,
	children,
	pending = false,
	meta,
	onCopy,
	onRetry,
	className
}: {
	role: MessageRole;
	author?: string;
	time?: string;
	children?: ReactNode;
	pending?: boolean;
	/** A quiet line under the turn: how long, how much. */ meta?: string;
	onCopy?: () => void;
	onRetry?: () => void;
	className?: string;
}) {
	return (
		<div className={cx('message', `message-${role}`, className)}>
			{(author || time) && (
				<div className="message-meta chrome">
					{author && <span className="message-author">{author}</span>}
					{time && <span className="message-time">{time}</span>}
				</div>
			)}
			<div className="message-body">
				{pending ? (
					<span className="message-pending">
						<Spinner size="sm" />
						<span>Thinking</span>
					</span>
				) : (
					children
				)}
			</div>
			{meta && !pending && <span className="message-meta-line chrome">{meta}</span>}
			{role === 'agent' && !pending && (onCopy || onRetry) && (
				<div className="message-actions chrome">
					{onCopy && (
						<Tooltip label="Copy">
							<IconButton icon={Copy} label="Copy" size="sm" onClick={onCopy} />
						</Tooltip>
					)}
					{onRetry && (
						<Tooltip label="Try again">
							<IconButton icon={RotateCcw} label="Try again" size="sm" onClick={onRetry} />
						</Tooltip>
					)}
				</div>
			)}
		</div>
	);
}

export type StepState = 'running' | 'done' | 'failed';

/* A verb the agent ran, as one quiet line: what it did and whether it is
   still doing it. */
export function Step({
	icon,
	label,
	detail,
	state = 'done',
	className
}: {
	icon: LucideIcon;
	label: string;
	detail?: string;
	state?: StepState;
	className?: string;
}) {
	return (
		<div className={cx('step chrome', `step-${state}`, className)}>
			<span className="step-glyph">
				{state === 'running' ? <Spinner size="sm" /> : <Icon icon={icon} size="sm" />}
			</span>
			<span className="step-label">{label}</span>
			{detail && <span className="step-detail">{detail}</span>}
			{state === 'done' && <Icon icon={Check} size="sm" className="icon icon-sm step-check" />}
		</div>
	);
}

/** The box that scrolls: the thread itself, or the nearest ancestor that
   does, never the page. */
const scroller = (el: HTMLElement): HTMLElement | null => {
	let node: HTMLElement | null = el;
	while (node && node !== document.body) {
		const o = getComputedStyle(node).overflowY;
		if (o === 'auto' || o === 'scroll') return node;
		node = node.parentElement;
	}
	return null;
};

/* The turns, in order. When a turn is added the scroll box moves to the
   newest; while the newest grows (`tail` changes) it keeps up only if it
   was already near the foot, so a reader who scrolled up is left alone.
   It never scrolls the page. */
export function Thread({
	children,
	className,
	follow = true,
	count,
	tail
}: {
	children: ReactNode;
	className?: string;
	follow?: boolean;
	count?: number;
	/** Anything that grows with the newest turn, a length. */ tail?: number;
}) {
	const box = useRef<HTMLDivElement | null>(null);
	useEffect(() => {
		const el = box.current && scroller(box.current);
		if (follow && el) el.scrollTop = el.scrollHeight;
	}, [follow, count]);
	useEffect(() => {
		const el = box.current && scroller(box.current);
		if (!follow || !el || tail === undefined) return;
		const near = el.scrollHeight - el.scrollTop - el.clientHeight < 96;
		if (near) el.scrollTop = el.scrollHeight;
	}, [follow, tail]);
	return (
		<div ref={box} className={cx('thread', className)}>
			{children}
		</div>
	);
}
