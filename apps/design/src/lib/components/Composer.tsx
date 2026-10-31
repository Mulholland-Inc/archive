import {
	useCallback,
	useEffect,
	useMemo,
	useRef,
	useState,
	type KeyboardEvent,
	type ReactNode
} from 'react';
import { ArrowUp, Paperclip, Square, X } from 'lucide-react';
import { cx } from '../cx';
import { Anchor, Layer, useDismiss } from '../floating';
import { Icon } from './Icon';
import { IconButton } from './Button';
import './Composer.css';

export interface Attachment {
	id: string;
	name: string;
}

export interface Command {
	id: string;
	label: string;
	hint?: string;
}

export interface Queued {
	id: string;
	text: string;
}

/* Where you talk to the agent. Grows with what you type (a hidden twin
   of the text sets the height, so nothing is measured), Enter sends,
   Shift+Enter breaks the line, attachments sit as words above the text.
   While the agent works the send is a stop, and Escape stops too; a
   message sent then waits above the field until its turn. A slash at
   the start opens the commands. */
export function Composer({
	value,
	onChange,
	onSend,
	onStop,
	working = false,
	commands,
	onCommand,
	queued = [],
	onUnqueue,
	onAttach,
	attachments = [],
	onRemoveAttachment,
	placeholder = 'Write a message',
	sending = false,
	disabled = false,
	leading,
	className
}: {
	value: string;
	onChange: (value: string) => void;
	onSend: () => void;
	onStop?: () => void;
	working?: boolean;
	commands?: Command[];
	onCommand?: (id: string) => void;
	/** Messages sent while the agent worked; they go next, in order. */
	queued?: Queued[];
	onUnqueue?: (id: string) => void;
	onAttach?: () => void;
	attachments?: Attachment[];
	onRemoveAttachment?: (id: string) => void;
	placeholder?: string;
	sending?: boolean;
	disabled?: boolean;
	leading?: ReactNode;
	className?: string;
}) {
	const [focused, setFocused] = useState(false);
	const [active, setActive] = useState(0);
	const box = useRef<HTMLSpanElement | null>(null);
	const field = useRef<HTMLTextAreaElement | null>(null);
	const layer = useRef<HTMLDivElement>(null);
	const canSend = value.trim().length > 0 && !sending && !disabled;

	const slash = useMemo(() => {
		const m = /^\/(\S*)$/.exec(value);
		if (!m || !commands?.length) return null;
		const q = m[1].toLowerCase();
		const hits = commands.filter(
			(c) => c.id.toLowerCase().startsWith(q) || c.label.toLowerCase().includes(q)
		);
		return hits.length ? hits : null;
	}, [value, commands]);
	const menuOpen = !!slash && focused;
	/* The command the words begin with, once it is whole: what it does,
	   above the field, so it is clear what sending will run. */
	const chosen = useMemo(() => {
		const m = /^\/(\S+)(?:\s|$)/.exec(value);
		if (!m || !commands?.length) return null;
		return commands.find((c) => c.id.toLowerCase() === m[1].toLowerCase()) ?? null;
	}, [value, commands]);
	useEffect(() => setActive(0), [value]);
	// The one the arrows are on stays in view while the list scrolls.
	useEffect(() => {
		layer.current?.querySelector('.menu-item-active')?.scrollIntoView({ block: 'nearest' });
	}, [active]);
	const close = useCallback(() => setFocused(false), []);
	const inside = useMemo(() => [box, layer], []);
	useDismiss(menuOpen, close, inside);

	const pick = (c: Command) => {
		if (onCommand) onCommand(c.id);
		else onChange(`/${c.id} `);
	};

	const key = (e: KeyboardEvent<HTMLTextAreaElement>) => {
		if (menuOpen && slash) {
			if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
				e.preventDefault();
				setActive((a) =>
					e.key === 'ArrowDown' ? (a + 1) % slash.length : (a - 1 + slash.length) % slash.length
				);
				return;
			}
			if (e.key === 'Enter' || e.key === 'Tab') {
				e.preventDefault();
				pick(slash[active]);
				return;
			}
		}
		if (e.key === 'Escape' && working && onStop) {
			e.preventDefault();
			onStop();
			return;
		}
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			if (canSend) onSend();
		}
	};

	return (
		<Anchor block anchorRef={box}>
			<div
				className={cx('composer', focused && 'composer-focused', className)}
				onMouseDown={(e) => {
					// A press on the box itself, not on a control in it, is for the field.
					if (
						e.target === e.currentTarget ||
						(e.target as HTMLElement).classList.contains('composer-grow') ||
						(e.target as HTMLElement).classList.contains('composer-bar')
					) {
						e.preventDefault();
						field.current?.focus();
					}
				}}
			>
				{queued.length > 0 && (
					<div className="composer-queue">
						{queued.map((q) => (
							<span key={q.id} className="composer-queued">
								<span className="composer-queued-text">{q.text}</span>
								{onUnqueue && (
									<button
										type="button"
										className="composer-attachment-remove"
										aria-label="Remove from the queue"
										onClick={() => onUnqueue(q.id)}
									>
										<Icon icon={X} size="sm" />
									</button>
								)}
							</span>
						))}
					</div>
				)}
				{chosen?.hint && !menuOpen && <div className="composer-command chrome">{chosen.hint}</div>}
				{attachments.length > 0 && (
					<div className="composer-attachments">
						{attachments.map((a) => (
							<span key={a.id} className="composer-attachment chrome">
								<Icon icon={Paperclip} size="sm" />
								<span className="composer-attachment-name">{a.name}</span>
								{onRemoveAttachment && (
									<button
										type="button"
										className="composer-attachment-remove"
										aria-label={`Remove ${a.name}`}
										onClick={() => onRemoveAttachment(a.id)}
									>
										<Icon icon={X} size="sm" />
									</button>
								)}
							</span>
						))}
					</div>
				)}
				<span className="composer-grow" data-value={value}>
					<textarea
						ref={field}
						className="composer-field"
						rows={1}
						value={value}
						placeholder={working ? 'Type ahead, it goes next' : placeholder}
						disabled={disabled}
						onChange={(e) => onChange(e.target.value)}
						onKeyDown={key}
						onFocus={() => setFocused(true)}
						onBlur={() => setFocused(false)}
					/>
				</span>
				<div className="composer-bar">
					<div className="composer-tools">
						{onAttach && (
							<IconButton
								icon={Paperclip}
								label="Attach"
								size="sm"
								onClick={onAttach}
								disabled={disabled}
							/>
						)}
						{leading}
					</div>
					{working && onStop ? (
						<button
							type="button"
							className="composer-send composer-stop chrome"
							aria-label="Stop"
							onClick={onStop}
						>
							<Icon icon={Square} size="sm" fill="currentColor" />
						</button>
					) : (
						<button
							type="button"
							className={cx('composer-send chrome', canSend && 'composer-send-ready')}
							aria-label={working ? 'Queue' : 'Send'}
							disabled={!canSend}
							onClick={onSend}
						>
							<Icon icon={ArrowUp} />
						</button>
					)}
				</div>
			</div>
			<Layer
				open={menuOpen}
				layerRef={layer}
				placement="top-start"
				className="menu chrome"
				role="listbox"
			>
				<div className="menu-list menu-list-scroll">
					{slash?.map((c, i) => (
						<button
							key={c.id}
							type="button"
							role="option"
							aria-selected={i === active}
							className={cx('menu-item', i === active && 'menu-item-active')}
							onMouseEnter={() => setActive(i)}
							onMouseDown={(e) => e.preventDefault()}
							onClick={() => pick(c)}
						>
							<span className="menu-item-label">/{c.id}</span>
							{c.hint && <span className="menu-item-hint">{c.hint}</span>}
						</button>
					))}
				</div>
			</Layer>
		</Anchor>
	);
}
