/**
 * The product the dashboard is dressed as: the engine's own (Archive) or a module's brand, such
 * as a family archive's. Its name and logo come from GET /api/brand; its colours and type come
 * from /brand.css, which index.html loads after the dashboard's own stylesheet.
 */
export const brand = $state<{ name: string; logo: string }>({ name: 'Archive', logo: '' });

export async function loadBrand(): Promise<void> {
  try {
    const r = await fetch('/api/brand', { credentials: 'same-origin' });
    if (!r.ok) return;
    const b = (await r.json()) as { name?: string; logo?: string };
    brand.name = b.name || 'Archive';
    brand.logo = b.logo || '';
    document.title = brand.name;
  } catch {
    // offline or the engine is older: the engine's own name and mark stay
  }
}
