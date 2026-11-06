import { Table2, Kanban, CalendarDays, NotebookText, MapPinned, LayoutGrid, ChartColumn, ChartGantt, ListTree, Building, ChartScatter, Grid3x3 } from '@lucide/svelte';

export const layouts = {
  table: { icon: Table2 },
  board: { icon: Kanban, component: () => import('./Board.svelte'), by: true },
  calendar: { icon: CalendarDays, component: () => import('./Calendar.svelte'), by: true },
  list: { icon: NotebookText, component: () => import('./List.svelte') },
  map: { icon: MapPinned, component: () => import('./Map.svelte'), by: true },
  gallery: { icon: LayoutGrid, component: () => import('./Gallery.svelte') },
  chart: { icon: ChartColumn, component: () => import('./Chart.svelte'), by: true },
  timeline: { icon: ChartGantt, component: () => import('./Timeline.svelte'), by: true },
  tree: { icon: ListTree, component: () => import('./Tree.svelte'), by: true },
  stack: { icon: Building, component: () => import('./Stack.svelte'), by: true },
  scatter: { icon: ChartScatter, component: () => import('./Scatter.svelte'), by: true },
  year: { icon: Grid3x3, component: () => import('./Year.svelte'), by: true }
};

export const layoutOf = (view) => layouts[view?.layout] ?? layouts.table;
