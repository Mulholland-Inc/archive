// The layouts a view can be drawn in, by the name its comment's layout: line gives: the icon the sidebar shows
// it by, and what draws its rows under the view's header. A table has no component; the view page draws it,
// and one with by needs the view's by: line. Each component takes { view, rows, shown, text, pick }: the view,
// the rows its search and filter left, the columns it shows, a cell as text, and what opens a row.
import { Table2, Kanban, CalendarDays, NotebookText, MapPinned } from '@lucide/svelte';
import Board from './Board.svelte';
import Calendar from './Calendar.svelte';
import List from './List.svelte';
import MapView from './Map.svelte';

export const layouts = {
  table: { icon: Table2 },
  board: { icon: Kanban, component: Board, by: true },
  calendar: { icon: CalendarDays, component: Calendar, by: true },
  list: { icon: NotebookText, component: List },
  map: { icon: MapPinned, component: MapView, by: true }
};

export const layoutOf = (view) => layouts[view?.layout] ?? layouts.table;
