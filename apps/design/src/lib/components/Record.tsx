import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import type { Hue } from '../hue';
import { ArrowUpRight, X } from 'lucide-react';
import { cx } from '../cx';
import { Cover } from './Cover';
import { Icon } from './Icon';
import { IconButton } from './Button';
import './Record.css';

/* One record as a page: the same top a document has — a banner, the mark
   half over its edge, the name below — and under it a property per row with
   its glyph and a quiet inline editor, the notes as prose, then the rows
   that point back at it.

   The top is Cover, shared with Document, so the two cannot drift apart. An
   icon still stands beside the properties, because that is the interface
   saying what kind of thing this is rather than the thing wearing its own
   mark. */
export function Record({
	icon,
	emoji,
	onEmoji,
	cover,
	hue,
	title,
	onTitleChange,
	placeholder = 'Untitled',
	onClose,
	children,
	className
}: {
	icon?: LucideIcon;
	emoji?: string;
	onEmoji?: (emoji: string) => void;
	/** A picture across the top; without one the banner is the record's tint. */ cover?: string;
	hue?: Hue;
	title: string;
	onTitleChange?: (title: string) => void;
	placeholder?: string;
	/** The way back to where the record was opened from. */ onClose?: () => void;
	children: ReactNode;
	className?: string;
}) {
	return (
		<Cover
			cover={cover}
			hue={hue}
			emoji={emoji}
			onEmoji={onEmoji}
			title={title}
			onTitle={onTitleChange}
			placeholder={placeholder}
			tools={
				onClose ? <IconButton icon={X} label="Close" size="sm" onClick={onClose} /> : undefined
			}
			className={cx('record', className)}
		>
			{icon && <Icon icon={icon} size="lg" className="record-icon" />}
			{children}
		</Cover>
	);
}

/* A property: its glyph and name on the left in one fixed column, the
   editor on the right. */
export function RecordProperty({
	icon,
	label,
	children
}: {
	icon?: LucideIcon;
	label: string;
	children: ReactNode;
}) {
	return (
		<div className="record-prop">
			<span className="record-prop-label chrome">
				{icon && <Icon icon={icon} size="sm" />}
				<span>{label}</span>
			</span>
			<span className="record-prop-control">{children}</span>
		</div>
	);
}

export function RecordProperties({ children }: { children: ReactNode }) {
	return <div className="record-props">{children}</div>;
}

/* The written part of the record, as prose you can type into. */
export function RecordBody({
	value,
	onChange,
	onBlur,
	placeholder = 'Write something'
}: {
	value: string;
	onChange?: (value: string) => void;
	/** Left alone: where a write belongs, rather than on every keystroke. */ onBlur?: (
		value: string
	) => void;
	placeholder?: string;
}) {
	return (
		<div className="record-body">
			<span className="record-body-grow" data-value={value}>
				<textarea
					className="record-body-field"
					value={value}
					placeholder={placeholder}
					readOnly={!onChange}
					onChange={(e) => onChange?.(e.target.value)}
					onBlur={(e) => onBlur?.(e.target.value)}
					rows={1}
				/>
			</span>
		</div>
	);
}

/* The rows that link back to this record, one section per type. */
export function RecordLinks({ title, children }: { title: string; children: ReactNode }) {
	return (
		<div className="record-links">
			<span className="record-links-title chrome">{title}</span>
			<div className="record-links-list">{children}</div>
		</div>
	);
}

export function RecordLink({
	icon,
	label,
	detail,
	onClick
}: {
	icon: LucideIcon;
	label: string;
	detail?: string;
	onClick?: () => void;
}) {
	return (
		<button type="button" className="record-link chrome" onClick={onClick}>
			<Icon icon={icon} />
			<span className="record-link-label">{label}</span>
			{detail && <span className="record-link-detail">{detail}</span>}
			<Icon icon={ArrowUpRight} size="sm" className="icon icon-sm record-link-arrow" />
		</button>
	);
}

export function RecordDivider() {
	return <hr className="record-divider" />;
}
