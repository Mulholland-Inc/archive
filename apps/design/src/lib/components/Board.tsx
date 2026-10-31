import { useMemo, useRef, useState, type ReactNode } from 'react';
import {
	DndContext,
	DragOverlay,
	KeyboardSensor,
	PointerSensor,
	closestCorners,
	useDroppable,
	useSensor,
	useSensors,
	type DragEndEvent,
	type DragOverEvent,
	type DragStartEvent
} from '@dnd-kit/core';
import {
	SortableContext,
	arrayMove,
	sortableKeyboardCoordinates,
	useSortable,
	verticalListSortingStrategy
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Plus } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Board.css';

export interface Lane {
	id: string;
	label: string;
}

export interface BoardCard {
	id: string;
	lane: string;
}

/* Lanes across, cards down them. A card lifts on a short drag, follows
   the pointer, and drops where it is let go, in any lane. The order is
   the array's order; a move hands back the whole array. */
export function Board<T extends BoardCard>({
	lanes,
	cards,
	onChange,
	renderCard,
	onOpen,
	onAdd,
	className
}: {
	lanes: Lane[];
	cards: T[];
	onChange: (cards: T[]) => void;
	renderCard: (card: T) => ReactNode;
	onOpen?: (card: T) => void;
	onAdd?: (lane: string) => void;
	className?: string;
}) {
	const [active, setActive] = useState<T | null>(null);
	/* The click that follows a drop is not an open. */
	const dropped = useRef(false);
	const open = (card: T) => {
		if (dropped.current) return;
		onOpen?.(card);
	};
	const sensors = useSensors(
		useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
		useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
	);
	const byLane = useMemo(() => {
		const m = new Map<string, T[]>(lanes.map((l) => [l.id, []]));
		for (const c of cards) m.get(c.lane)?.push(c);
		return m;
	}, [lanes, cards]);

	const laneOf = (id: string) =>
		lanes.some((l) => l.id === id) ? id : cards.find((c) => c.id === id)?.lane;

	const start = (e: DragStartEvent) => setActive(cards.find((c) => c.id === e.active.id) ?? null);

	/* Crossing into another lane moves the card there at once, so the
	   lane opens a slot under the pointer while it is still held. */
	const over = (e: DragOverEvent) => {
		const from = laneOf(String(e.active.id));
		const to = e.over ? laneOf(String(e.over.id)) : undefined;
		if (!from || !to || from === to) return;
		const moving = cards.find((c) => c.id === e.active.id);
		if (!moving) return;
		const rest = cards.filter((c) => c.id !== moving.id);
		const overIndex = rest.findIndex((c) => c.id === e.over?.id);
		const at = overIndex >= 0 ? overIndex : rest.length;
		rest.splice(at, 0, { ...moving, lane: to });
		onChange(rest);
	};

	const end = (e: DragEndEvent) => {
		setActive(null);
		dropped.current = true;
		window.setTimeout(() => (dropped.current = false), 0);
		if (!e.over || e.active.id === e.over.id) return;
		const a = cards.findIndex((c) => c.id === e.active.id);
		const b = cards.findIndex((c) => c.id === e.over?.id);
		if (a < 0 || b < 0) return;
		onChange(arrayMove(cards, a, b));
	};

	return (
		<DndContext
			sensors={sensors}
			collisionDetection={closestCorners}
			onDragStart={start}
			onDragOver={over}
			onDragEnd={end}
			onDragCancel={() => setActive(null)}
		>
			<div className={cx('board', className)}>
				{lanes.map((l) => (
					<LaneColumn
						key={l.id}
						lane={l}
						cards={byLane.get(l.id) ?? []}
						renderCard={renderCard}
						onOpen={open}
						onAdd={onAdd}
					/>
				))}
			</div>
			<DragOverlay dropAnimation={{ duration: 200, easing: 'cubic-bezier(0.2, 0, 0, 1)' }}>
				{active ? <div className="card card-lifted">{renderCard(active)}</div> : null}
			</DragOverlay>
		</DndContext>
	);
}

function LaneColumn<T extends BoardCard>({
	lane,
	cards,
	renderCard,
	onOpen,
	onAdd
}: {
	lane: Lane;
	cards: T[];
	renderCard: (card: T) => ReactNode;
	onOpen?: (card: T) => void;
	onAdd?: (lane: string) => void;
}) {
	const { setNodeRef, isOver } = useDroppable({ id: lane.id });
	return (
		<section className={cx('lane', isOver && 'lane-over')}>
			<header className="lane-head chrome">
				<span className="lane-label">{lane.label}</span>
				{onAdd && (
					<button
						type="button"
						className="lane-add"
						aria-label={`Add to ${lane.label}`}
						onClick={() => onAdd(lane.id)}
					>
						<Icon icon={Plus} size="sm" />
					</button>
				)}
			</header>
			<SortableContext items={cards.map((c) => c.id)} strategy={verticalListSortingStrategy}>
				<div ref={setNodeRef} className="lane-cards">
					{cards.map((c) => (
						<SortableCard key={c.id} card={c} onOpen={onOpen}>
							{renderCard(c)}
						</SortableCard>
					))}
				</div>
			</SortableContext>
		</section>
	);
}

function SortableCard<T extends BoardCard>({
	card,
	children,
	onOpen
}: {
	card: T;
	children: ReactNode;
	onOpen?: (card: T) => void;
}) {
	const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
		id: card.id
	});
	return (
		<div
			ref={setNodeRef}
			style={{ transform: CSS.Transform.toString(transform), transition }}
			className={cx('card', isDragging && 'card-ghost')}
			{...attributes}
			{...listeners}
			onClick={() => onOpen?.(card)}
		>
			{children}
		</div>
	);
}

/* What a card usually holds: a title, a line under it, and a foot. */
export function CardBody({
	title,
	subtitle,
	foot
}: {
	title: string;
	subtitle?: string;
	foot?: ReactNode;
}) {
	return (
		<>
			<span className="card-title">{title}</span>
			{subtitle && <span className="card-subtitle">{subtitle}</span>}
			{foot && <span className="card-foot">{foot}</span>}
		</>
	);
}
