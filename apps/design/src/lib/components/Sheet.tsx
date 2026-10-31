import {
	createContext,
	useContext,
	useEffect,
	useMemo,
	useRef,
	type ClipboardEvent,
	type KeyboardEvent as ReactKeyboardEvent,
	type MouseEvent as ReactMouseEvent,
	type PointerEvent as ReactPointerEvent
} from 'react';
import {
	cellSelectionFeature,
	columnPinningFeature,
	columnSizingFeature,
	createColumnHelper,
	tableFeatures,
	useTable
} from '@tanstack/react-table';
import { useVirtualizer } from '@tanstack/react-virtual';
import { Plus } from 'lucide-react';
import { cx } from '../cx';
import { Icon } from './Icon';
import './Sheet.css';

/* The sheet, as Lulu's build desk has it: TanStack owns the column model
   (sizes, pinning, the row model) and the cell selection; the markup, the
   keyboard and the clipboard are ours. Both axes are virtualised, so a
   sheet of thousands of rows puts nothing in the document that is not on
   the screen. Every cell is an input: click and you are typing, drag and
   you are selecting, and a block pasted from Excel lands where you stand. */

export interface SheetColumn {
	id: string;
	label: string;
	kind?: 'text' | 'number';
	width?: number;
	readonly?: boolean;
	placeholder?: string;
}

export interface SheetRow {
	id: string;
	values: Record<string, string>;
}

export interface SheetChange {
	rowId: string;
	columnId: string;
	value: string;
}

interface Props {
	columns: SheetColumn[];
	rows: SheetRow[];
	pinned?: string[];
	onChange: (rowId: string, columnId: string, value: string) => void;
	onFill?: (changes: SheetChange[]) => void;
	onAddRow?: () => void;
	className?: string;
}

const features = tableFeatures({ columnSizingFeature, columnPinningFeature, cellSelectionFeature });
const helper = createColumnHelper<typeof features, SheetRow>();

/* Column definitions must keep their identity across a keystroke, or the
   input being typed in unmounts. So the cell renderer is one module-level
   component reading the live props through context. */
type Live = { current: Props };
const LiveContext = createContext<Live>({ current: {} as Props });

function readVar(name: string, fallback: number) {
	if (typeof document === 'undefined') return fallback;
	const v = parseFloat(getComputedStyle(document.documentElement).getPropertyValue(name));
	return Number.isNaN(v) ? fallback : v;
}

export function Sheet({
	columns,
	rows,
	pinned = [],
	onChange,
	onFill,
	onAddRow,
	className
}: Props) {
	const live = useRef<Props>({} as Props);
	live.current = { columns, rows, pinned, onChange, onFill, onAddRow, className };

	const rowHeight = useMemo(() => readVar('--sheet-row-height', 40), []);
	const defaultWidth = useMemo(() => readVar('--sheet-column-width', 160), []);

	const defs = useMemo(
		() =>
			helper.columns(
				columns.map((c) =>
					helper.display({
						id: c.id,
						size: c.width ?? defaultWidth,
						header: c.label,
						meta: { column: c },
						cell: Cell
					})
				)
			),
		[columns, defaultWidth]
	);

	const table = useTable({
		features,
		columns: defs,
		data: rows,
		getRowId: (r: SheetRow) => r.id,
		autoResetCellSelection: false,
		initialState: { columnPinning: { start: pinned, end: [] } }
	});

	const scrollRef = useRef<HTMLDivElement>(null);
	const modelRows = table.getRowModel().rows;
	const start = table.getStartLeafColumns();
	const centre = table.getCenterLeafColumns();

	const rowV = useVirtualizer({
		count: modelRows.length,
		getScrollElement: () => scrollRef.current,
		estimateSize: () => rowHeight,
		getItemKey: (i) => modelRows[i].id,
		overscan: 8
	});
	const colV = useVirtualizer({
		horizontal: true,
		count: centre.length,
		getScrollElement: () => scrollRef.current,
		estimateSize: (i) => centre[i].getSize(),
		getItemKey: (i) => centre[i].id,
		overscan: 3
	});

	const cols = colV.getVirtualItems();
	const pinnedWidth = start.reduce((n, c) => n + c.getSize(), 0);
	const before = cols.length ? cols[0].start : 0;
	const after = cols.length ? colV.getTotalSize() - cols[cols.length - 1].end : 0;
	const order = useMemo(() => [...start, ...centre], [start, centre]);

	useEffect(() => {
		rowV.measure();
	}, [rowHeight, rowV]);

	const show = (rowId?: string, columnId?: string) => {
		if (!rowId) return;
		const r = rows.findIndex((x) => x.id === rowId);
		if (r >= 0) rowV.scrollToIndex(r, { align: 'auto' });
		const c = centre.findIndex((x) => x.id === columnId);
		if (c >= 0) colV.scrollToIndex(c, { align: 'auto' });
	};
	const showFocused = () => {
		const f = table.getFocusedCell();
		show(f?.row.id, f?.column.id);
	};

	const inputOf = (rowId: string, columnId: string) =>
		scrollRef.current?.querySelector<HTMLInputElement>(
			`[data-row="${CSS.escape(rowId)}"][data-col="${CSS.escape(columnId)}"] input`
		);

	/* Typing a character while the sheet has the focus puts it in the cell
	   that stands selected; the arrows and shift are the sheet's, and inside
	   an input the arrows belong to the caret. */
	const keys = (e: ReactKeyboardEvent<HTMLDivElement>) => {
		const dir = { ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right' }[
			e.key
		] as 'up' | 'down' | 'left' | 'right' | undefined;
		const mod = e.metaKey || e.ctrlKey;
		const inInput = e.target instanceof HTMLInputElement;

		if (dir) {
			if (inInput && !e.shiftKey && !mod) return;
			e.preventDefault();
			if (e.shiftKey) table.extendCellSelection(dir);
			else table.moveCellSelection(dir);
			if (inInput) scrollRef.current?.focus({ preventScroll: true });
			return showFocused();
		}
		if (e.key === 'Enter') {
			e.preventDefault();
			const f = table.getFocusedCell();
			if (!f) return;
			if (inInput) {
				table.moveCellSelection('down');
				scrollRef.current?.focus({ preventScroll: true });
				return showFocused();
			}
			const input = inputOf(f.row.id, f.column.id);
			input?.focus();
			input?.select();
			return;
		}
		if (mod && e.key.toLowerCase() === 'a') {
			e.preventDefault();
			return table.selectAllCells();
		}
		if (mod && e.key.toLowerCase() === 'c' && !inInput) {
			e.preventDefault();
			return copy();
		}
		if (e.key === 'Escape') {
			if (inInput) return scrollRef.current?.focus({ preventScroll: true });
			table.resetCellSelection(true);
			return;
		}
		if ((e.key === 'Backspace' || e.key === 'Delete') && !inInput) {
			e.preventDefault();
			return clear();
		}
		if (!inInput && e.key.length === 1 && !mod && !e.altKey) {
			const f = table.getFocusedCell();
			if (!f) return;
			const input = inputOf(f.row.id, f.column.id);
			if (!input || input.readOnly) return;
			e.preventDefault();
			onChange(f.row.id, f.column.id, e.key);
			requestAnimationFrame(() => {
				input.focus();
				input.setSelectionRange(input.value.length, input.value.length);
			});
		}
	};

	const textOf = (r: SheetRow, columnId: string) => r.values[columnId] ?? '';

	/* A spreadsheet's clipboard is tab separated, quoted where a value holds
	   a tab, a newline or a quote. */
	const copy = () => {
		const text = table
			.getCellSelectionBounds()
			.map((b) =>
				rows
					.slice(b.minRowIndex, b.maxRowIndex + 1)
					.map((r) =>
						order
							.slice(b.minColumnIndex, b.maxColumnIndex + 1)
							.map((c) => {
								const t = textOf(r, c.id);
								return /[\t\n"]/.test(t) ? `"${t.replace(/"/g, '""')}"` : t;
							})
							.join('\t')
					)
					.join('\n')
			)
			.join('\n\n');
		if (text) navigator.clipboard?.writeText(text).catch(() => {});
	};

	const clear = () => {
		const changes: SheetChange[] = [];
		for (const b of table.getCellSelectionBounds())
			for (const r of rows.slice(b.minRowIndex, b.maxRowIndex + 1))
				for (const c of order.slice(b.minColumnIndex, b.maxColumnIndex + 1)) {
					const col = columns.find((x) => x.id === c.id);
					if (col && !col.readonly && textOf(r, c.id) !== '')
						changes.push({ rowId: r.id, columnId: c.id, value: '' });
				}
		if (!changes.length) return;
		if (onFill) onFill(changes);
		else for (const ch of changes) onChange(ch.rowId, ch.columnId, ch.value);
	};

	const paste = (e: ClipboardEvent<HTMLElement>, ri: number, columnId: string) => {
		const text = e.clipboardData.getData('text/plain');
		if (!text.includes('\n') && !text.includes('\t')) return;
		e.preventDefault();
		const ci = order.findIndex((c) => c.id === columnId);
		if (ci < 0) return;
		const lines = text
			.replace(/\r/g, '')
			.split('\n')
			.filter((l, i, a) => l.length || i < a.length - 1);
		const changes: SheetChange[] = [];
		lines.forEach((line, dy) => {
			const row = rows[ri + dy];
			if (!row) return;
			line.split('\t').forEach((cell, dx) => {
				const c = order[ci + dx];
				const col = c && columns.find((x) => x.id === c.id);
				if (!col || col.readonly) return;
				changes.push({ rowId: row.id, columnId: col.id, value: cell.trim() });
			});
		});
		if (onFill) onFill(changes);
		else for (const ch of changes) onChange(ch.rowId, ch.columnId, ch.value);
	};

	const pasteHere = (e: ClipboardEvent<HTMLDivElement>) => {
		if (e.defaultPrevented) return;
		const at = table.getFocusedCell();
		if (!at) return;
		const ri = rows.findIndex((r) => r.id === at.row.id);
		if (ri >= 0) paste(e, ri, at.column.id);
	};

	/* A press starts a selection wherever it lands. On an input the caret is
	   left alone; anywhere else the sheet takes the focus so the keys arrive.
	   The drag follows the pointer, since a press in an input captures it. */
	const from = useRef<{ row: string; col: string } | null>(null);
	const cellUnder = (x: number, y: number) => {
		const el = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-row][data-col]');
		return el ? { row: el.dataset.row!, col: el.dataset.col! } : null;
	};
	const startSelection = (cell: Selectable) => (e: ReactMouseEvent<HTMLDivElement>) => {
		cell.getSelectionStartHandler()(e);
		from.current = { row: cell.row.id, col: cell.column.id };
		const stop = () => {
			from.current = null;
			window.removeEventListener('pointerup', stop);
		};
		window.addEventListener('pointerup', stop);
		if (e.target instanceof HTMLInputElement) return;
		e.preventDefault();
		scrollRef.current?.focus({ preventScroll: true });
	};
	const dragTo = (e: ReactPointerEvent<HTMLDivElement>) => {
		const anchor = from.current;
		if (!anchor) return;
		const to = cellUnder(e.clientX, e.clientY);
		if (!to || (to.row === anchor.row && to.col === anchor.col)) return;
		table.selectCellRange({
			anchorRowId: anchor.row,
			anchorColumnId: anchor.col,
			focusRowId: to.row,
			focusColumnId: to.col
		});
	};

	const pinnedStyle = (i: number) => ({
		width: start[i].getSize(),
		insetInlineStart: start[i].getStart('start')
	});
	const kindOf = (id: string) => columns.find((c) => c.id === id)?.kind;

	return (
		<LiveContext.Provider value={live}>
			<div className={cx('sheet', className)}>
				<div
					ref={scrollRef}
					className="sheet-scroll"
					tabIndex={0}
					onKeyDown={keys}
					onPointerMove={dragTo}
					onPaste={pasteHere}
				>
					<div className="sheet-grid" style={{ width: pinnedWidth + colV.getTotalSize() }}>
						<div className="sheet-head chrome">
							<div className="sheet-row">
								{start.map((c, i) => (
									<div
										key={c.id}
										className={cx(
											'sheet-cell sheet-cell-head sheet-cell-pin',
											kindOf(c.id) === 'number' && 'sheet-end'
										)}
										style={pinnedStyle(i)}
									>
										{c.columnDef.header as string}
									</div>
								))}
								{before ? (
									<div className="sheet-cell sheet-cell-spacer" style={{ width: before }} />
								) : null}
								{cols.map((v) => {
									const c = centre[v.index];
									return (
										<div
											key={c.id}
											className={cx(
												'sheet-cell sheet-cell-head',
												kindOf(c.id) === 'number' && 'sheet-end'
											)}
											style={{ width: v.size }}
										>
											{c.columnDef.header as string}
										</div>
									);
								})}
								{after ? (
									<div className="sheet-cell sheet-cell-spacer" style={{ width: after }} />
								) : null}
							</div>
						</div>
						<div className="sheet-body" style={{ height: rowV.getTotalSize() }}>
							{rowV.getVirtualItems().map((v) => {
								const row = modelRows[v.index];
								const cells = row.getAllCellsByColumnId();
								return (
									<div
										key={row.id}
										className="sheet-row sheet-row-body"
										style={{ transform: `translateY(${v.start}px)` }}
									>
										{start.map((c, i) => (
											<div
												key={c.id}
												className={cx(
													'sheet-cell sheet-cell-pin',
													kindOf(c.id) === 'number' && 'sheet-end',
													picked(cells[c.id])
												)}
												style={pinnedStyle(i)}
												data-row={row.id}
												data-col={c.id}
												onMouseDown={startSelection(cells[c.id])}
												onMouseEnter={cells[c.id].getSelectionExtendHandler()}
												onPasteCapture={(e) => paste(e, v.index, c.id)}
											>
												<table.FlexRender cell={cells[c.id]} />
											</div>
										))}
										{before ? (
											<div className="sheet-cell sheet-cell-spacer" style={{ width: before }} />
										) : null}
										{cols.map((cv) => {
											const c = centre[cv.index];
											const cell = cells[c.id];
											return (
												<div
													key={c.id}
													className={cx(
														'sheet-cell',
														kindOf(c.id) === 'number' && 'sheet-end',
														picked(cell)
													)}
													style={{ width: cv.size }}
													data-row={row.id}
													data-col={c.id}
													onMouseDown={startSelection(cell)}
													onMouseEnter={cell.getSelectionExtendHandler()}
													onPasteCapture={(e) => paste(e, v.index, c.id)}
												>
													<table.FlexRender cell={cell} />
												</div>
											);
										})}
										{after ? (
											<div className="sheet-cell sheet-cell-spacer" style={{ width: after }} />
										) : null}
									</div>
								);
							})}
						</div>
					</div>
				</div>
				{onAddRow && (
					<button type="button" className="sheet-add chrome" onClick={onAddRow}>
						<Icon icon={Plus} size="sm" />
						<span>Add a row</span>
					</button>
				)}
			</div>
		</LiveContext.Provider>
	);
}

/* The outline is drawn from the edges the feature reports, so a range comes
   out as one continuous border rather than a box per cell. */
type Selectable = {
	row: { id: string };
	column: { id: string };
	getIsSelected: () => boolean;
	getIsFocused: () => boolean;
	getSelectionEdges: () => { top: boolean; right: boolean; bottom: boolean; left: boolean };
	getSelectionStartHandler: () => (e: unknown) => void;
	getSelectionExtendHandler: () => (e: unknown) => void;
};

function picked(cell: Selectable): string {
	if (!cell?.getIsSelected?.()) return cell?.getIsFocused?.() ? 'sheet-cell-caret' : '';
	const e = cell.getSelectionEdges();
	return cx(
		'sheet-cell-picked',
		e.top && 'sheet-t',
		e.right && 'sheet-r',
		e.bottom && 'sheet-b',
		e.left && 'sheet-l',
		cell.getIsFocused() && 'sheet-cell-caret'
	);
}

type CellCtx = {
	row: { id: string; original: SheetRow };
	column: { columnDef: { meta?: unknown } };
};

/* Every cell is one input inside a grid box whose hidden twin is the same
   text, so the box is as wide as the words: when the cell has the focus it
   floats out over its neighbours and shows all of them. */
function Cell(ctx: CellCtx) {
	const live = useContext(LiveContext);
	const col = (ctx.column.columnDef.meta as { column: SheetColumn }).column;
	const value = ctx.row.original.values[col.id] ?? '';
	const { onChange } = live.current;
	return (
		<span className="sheet-grow" data-value={value || col.placeholder || ''}>
			<input
				className="sheet-input"
				value={value}
				readOnly={col.readonly}
				placeholder={col.placeholder}
				inputMode={col.kind === 'number' ? 'decimal' : undefined}
				onChange={(e) => onChange?.(ctx.row.id, col.id, e.target.value)}
			/>
		</span>
	);
}
