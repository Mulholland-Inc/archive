import { useEffect, useMemo, useRef, useState } from 'react';
import { Calendar as CalendarIcon, ChevronLeft, ChevronRight, Plus } from 'lucide-react';
import { cx } from '../cx';
import { Button, IconButton } from './Button';
import { Emoji } from './Emoji';
import { Icon } from './Icon';
import { Segmented } from './Tabs';
import type { Hue } from '../hue';
import './Calendar.css';

export interface CalendarEvent {
	id: string;
	/** The day, as YYYY-MM-DD. */
	date: string;
	title: string;
	time?: string;
	/** The meeting's own emoji, as a page has one. */
	emoji?: string;
	hue?: Hue;
}

export type CalendarView = 'month' | 'week';

const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const monthNames = [
	'January',
	'February',
	'March',
	'April',
	'May',
	'June',
	'July',
	'August',
	'September',
	'October',
	'November',
	'December'
];

function key(d: Date) {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
function shift(d: Date, days: number) {
	const n = new Date(d);
	n.setDate(n.getDate() + days);
	return n;
}
function mondayOf(d: Date) {
	const n = new Date(d);
	n.setDate(n.getDate() - ((n.getDay() + 6) % 7));
	return n;
}

/* The month as a grid of weeks, or one week as seven columns, with the
   meetings on their days as Notion's calendar has its pages: a mark
   and a title on a tint. A day's number sits at its top left, today's
   in the act colour, the first of a month with its month. A plus shows
   on the day under the pointer; press it, or the day itself, and a
   new meeting opens on that day to be named in place. Press a meeting
   to open it. */
export function Calendar({
	cursor,
	onCursorChange,
	events,
	view = 'month',
	onViewChange,
	onCreate,
	onOpen,
	today = new Date(),
	inset = false,
	className
}: {
	cursor: Date;
	onCursorChange: (d: Date) => void;
	events: CalendarEvent[];
	view?: CalendarView;
	onViewChange?: (v: CalendarView) => void;
	/** A new meeting, named, on a day. */ onCreate?: (date: string, title: string) => void;
	onOpen?: (e: CalendarEvent) => void;
	today?: Date;
	/** The head carries the page's inset itself, so the grid can run edge to edge. */ inset?: boolean;
	className?: string;
}) {
	const [naming, setNaming] = useState<string | null>(null);
	const byDay = useMemo(() => {
		const m = new Map<string, CalendarEvent[]>();
		for (const e of events) m.set(e.date, [...(m.get(e.date) ?? []), e]);
		return m;
	}, [events]);

	const days = useMemo(() => {
		if (view === 'week') {
			const start = mondayOf(cursor);
			return Array.from({ length: 7 }, (_, i) => shift(start, i));
		}
		const first = new Date(cursor.getFullYear(), cursor.getMonth(), 1);
		const start = mondayOf(first);
		const out: Date[] = [];
		for (let d = start; out.length < 42; d = shift(d, 1)) out.push(d);
		while (out.length > 35 && out[35].getMonth() !== cursor.getMonth()) out.splice(35);
		return out;
	}, [cursor, view]);

	const step = (n: number) => {
		if (view === 'week') return onCursorChange(shift(cursor, 7 * n));
		onCursorChange(new Date(cursor.getFullYear(), cursor.getMonth() + n, 1));
	};

	const title =
		view === 'week'
			? `${monthNames[days[0].getMonth()]} ${days[0].getDate()} to ${days[6].getDate() < days[0].getDate() ? `${monthNames[days[6].getMonth()]} ` : ''}${days[6].getDate()}`
			: `${monthNames[cursor.getMonth()]} ${cursor.getFullYear()}`;
	const todayKey = key(today);

	return (
		<div className={cx('calendar', `calendar-${view}`, inset && 'calendar-inset', className)}>
			<div className="calendar-head chrome">
				<span className="calendar-title">{title}</span>
				<span className="calendar-tools">
					{onViewChange && (
						<Segmented
							value={view}
							onChange={onViewChange}
							tabs={[
								{ id: 'month', label: 'Month' },
								{ id: 'week', label: 'Week' }
							]}
						/>
					)}
					<IconButton icon={ChevronLeft} label="Earlier" size="sm" onClick={() => step(-1)} />
					<Button size="sm" variant="ghost" onClick={() => onCursorChange(new Date(today))}>
						Today
					</Button>
					<IconButton icon={ChevronRight} label="Later" size="sm" onClick={() => step(1)} />
					{onCreate && (
						<Button
							size="sm"
							variant="primary"
							onClick={() =>
								setNaming(
									view === 'week'
										? key(cursor)
										: key(
												cursor.getMonth() === today.getMonth() &&
													cursor.getFullYear() === today.getFullYear()
													? today
													: new Date(cursor.getFullYear(), cursor.getMonth(), 1)
											)
								)
							}
						>
							New
						</Button>
					)}
				</span>
			</div>
			<div className="calendar-days chrome">
				{dayNames.map((d, i) => (
					<span key={d} className="calendar-dayname">
						<span className="calendar-dayname-long">{d}</span>
						<span className="calendar-dayname-short">{d[0]}</span>
						{view === 'week' && (
							<span
								className={cx(
									'calendar-daynum',
									key(days[i]) === todayKey && 'calendar-daynum-today'
								)}
							>
								{days[i].getDate()}
							</span>
						)}
					</span>
				))}
			</div>
			<div className="calendar-grid">
				{days.map((d) => {
					const k = key(d);
					const here = k === todayKey;
					const off = view === 'month' && d.getMonth() !== cursor.getMonth();
					const list = byDay.get(k) ?? [];
					const first = d.getDate() === 1;
					return (
						<div
							key={k}
							className={cx(
								'calendar-cell',
								off && 'calendar-cell-off',
								here && 'calendar-cell-today'
							)}
							onClick={() => onCreate && setNaming(k)}
						>
							<div className="calendar-cell-head chrome">
								{view === 'month' && (
									<span className="calendar-num">
										{first ? `${monthNames[d.getMonth()].slice(0, 3)} ${d.getDate()}` : d.getDate()}
									</span>
								)}
								{onCreate && (
									<IconButton
										icon={Plus}
										label={`New meeting on ${k}`}
										size="sm"
										className="calendar-add"
										onClick={(ev) => {
											ev.stopPropagation();
											setNaming(k);
										}}
									/>
								)}
							</div>
							<div className="calendar-events">
								{list.map((e) => (
									<button
										key={e.id}
										type="button"
										className="calendar-event chrome"
										style={
											{
												'--event-hue': `var(--hue-${e.hue ?? 'gray'}-text)`,
												'--event-soft': `var(--hue-${e.hue ?? 'gray'}-soft)`
											} as React.CSSProperties
										}
										onClick={(ev) => {
											ev.stopPropagation();
											onOpen?.(e);
										}}
									>
										{e.emoji ? (
											<Emoji size="sm">{e.emoji}</Emoji>
										) : (
											<Icon icon={CalendarIcon} size="sm" />
										)}
										<span className="calendar-event-title">{e.title}</span>
										{e.time && <span className="calendar-event-time">{e.time}</span>}
									</button>
								))}
								{naming === k && onCreate && (
									<Naming
										onDone={(t) => {
											setNaming(null);
											if (t.trim()) onCreate(k, t.trim());
										}}
									/>
								)}
							</div>
						</div>
					);
				})}
			</div>
		</div>
	);
}

/* A new meeting being named where it will sit. Enter keeps it, Escape
   or leaving with nothing typed lets it go. */
function Naming({ onDone }: { onDone: (title: string) => void }) {
	const field = useRef<HTMLInputElement>(null);
	const [text, setText] = useState('');
	useEffect(() => field.current?.focus(), []);
	return (
		<span className="calendar-event calendar-naming" onClick={(e) => e.stopPropagation()}>
			<Icon icon={CalendarIcon} size="sm" />
			<input
				ref={field}
				className="calendar-naming-field"
				value={text}
				placeholder="Name it"
				onChange={(e) => setText(e.target.value)}
				onBlur={() => onDone(text)}
				onKeyDown={(e) => {
					if (e.key === 'Enter') onDone(text);
					if (e.key === 'Escape') onDone('');
				}}
			/>
		</span>
	);
}
