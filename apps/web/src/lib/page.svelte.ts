export type Crumb = { label: string; href?: string };

export const page = $state<{ crumbs: Crumb[] }>({ crumbs: [] });

export function setCrumbs(crumbs: Crumb[]) {
  page.crumbs = crumbs;
  const last = crumbs[crumbs.length - 1]?.label;
  document.title = last ? `${last} · Archive` : 'Archive';
}
