import { Renderer, listScenes, loadSources } from './render.js';

const $ = id => document.getElementById(id);
const ui = { scene: $('scene'), bg: $('bg'), line: $('line'), ss: $('ss'), scale: $('scale'), ao: $('ao'), shade: $('shade'), watch: $('watch'), reload: $('reload') };
const out = $('out'), status = $('status'), err = $('err'), stage = $('stage');
const renderer = new Renderer();
const params = new URLSearchParams(location.search);
let last = '', busy = false, again = false;

ui.scale.value = String(Math.min(2, Math.round(devicePixelRatio || 1)));
for (const name of await listScenes()) ui.scene.add(new Option(name, name));
if (params.get('scene')) ui.scene.value = params.get('scene');

async function render() {
  if (busy) { again = true; return; }
  busy = true;
  const t0 = performance.now();
  try {
    const src = await loadSources(ui.scene.value);
    last = src.lib + src.scene;
    const scale = +ui.scale.value;
    status.textContent = 'rendering';
    const ctx = out.getContext('2d');
    const img = await renderer.render(src, {
      scale, ss: +ui.ss.value, line: +ui.line.value, bg: ui.bg.value || null, ao: ui.ao.checked ? 0.35 : 0, shade: ui.shade.value === '0' ? undefined : +ui.shade.value,
      onTile: async img => {
        if (out.width !== img.width || out.height !== img.height) {
          out.width = img.width;
          out.height = img.height;
          out.style.width = `${img.width / scale}px`;
        }
        ctx.putImageData(img, 0, 0);
        await new Promise(r => requestAnimationFrame(r));
      },
    });
    ctx.putImageData(img, 0, 0);
    err.hidden = true;
    status.textContent = `${img.width}x${img.height} in ${Math.round(performance.now() - t0)} ms`;
  } catch (e) {
    err.textContent = e.message;
    err.hidden = false;
    status.textContent = 'error';
  }
  stage.classList.toggle('checker', !ui.bg.value);
  busy = false;
  if (again) { again = false; render(); }
}

async function poll() {
  if (ui.watch.checked && !busy) {
    try {
      const src = await loadSources(ui.scene.value);
      if (src.lib + src.scene !== last) render();
    } catch {}
  }
  setTimeout(poll, 700);
}

for (const k of ['scene', 'bg', 'line', 'ss', 'scale', 'ao', 'shade']) ui[k].addEventListener('change', () => {
  if (k === 'scene') history.replaceState(null, '', `?scene=${ui.scene.value}`);
  render();
});
ui.reload.addEventListener('click', render);
await render();
poll();
