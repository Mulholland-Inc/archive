import type { ReactNode } from 'react';
import { cx } from '../cx';
import './Table.css';

export interface Column<T> {
	id: string;
	label: string;
	render: (row: T) => ReactNode;
	align?: 'start' | 'end';
	width?: string;
}

/* Rows under a quiet head. Numbers sit on the right; everything else on
   the left. */
export function Table<T>({
	columns,
	rows,
	rowKey,
	onRowClick,
	className
}: {
	columns: Column<T>[];
	rows: T[];
	rowKey: (row: T) => string;
	onRowClick?: (row: T) => void;
	className?: string;
}) {
	return (
		<table className={cx('table', className)}>
			<thead className="chrome">
				<tr>
					{columns.map((c) => (
						<th
							key={c.id}
							className={cx('table-head', c.align === 'end' && 'table-end')}
							style={{ width: c.width }}
						>
							{c.label}
						</th>
					))}
				</tr>
			</thead>
			<tbody>
				{rows.map((r) => (
					<tr
						key={rowKey(r)}
						className={cx('table-row', onRowClick && 'table-row-clickable')}
						onClick={onRowClick ? () => onRowClick(r) : undefined}
					>
						{columns.map((c) => (
							<td key={c.id} className={cx('table-cell', c.align === 'end' && 'table-end')}>
								{c.render(r)}
							</td>
						))}
					</tr>
				))}
			</tbody>
		</table>
	);
}
