import { api, send } from '../api.js';

// A family archive opens on one of its own pictures, the same one all day.
export async function keepsake() {
  const pictures = (await api('/api/files?limit=500')).filter((f) => f.mime?.startsWith('image/'));
  if (!pictures.length) return null;
  const f = pictures[Math.floor(Date.now() / 864e5) % pictures.length];
  const blob = await (await send(`/api/files/${f.id}/content`)).blob();
  return { url: URL.createObjectURL(blob), title: f.name.replace(/\.[^.]+$/, '') };
}
