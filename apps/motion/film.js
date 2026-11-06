import { shot, W, H, C, APPS, SANS, art, curve, fill, font, hash, logo, mix, project, round, shifted, tile, wordmark, zoomed } from './kit.js';

function wall(g, spin, flip) {
  const pitch = 270, size = 250;
  for (let c = -1; c <= 8; c++) {
    const off = (c % 2 ? 1 : -1) * spin;
    const y0 = ((off % pitch) + pitch) % pitch;
    const base = Math.floor(off / pitch);
    for (let r = -2; r <= 5; r++) {
      const name = APPS[(((c + 1) * 7 + (r - base) * 3 + 330) % APPS.length + APPS.length) % APPS.length];
      const cx = 60 + c * pitch + 150, cy = r * pitch + y0 + 60;
      tile(g, name, cx, cy, size, flip ? flip(cx, cy) : 1);
    }
  }
}

shot('apps', 1.5, s => {
  const spin = s.v(0, 1.6, 'quart'), rush = s.v(1.15, 0.35, 'in');
  const bar = s.bar(0, 430, W, 220, C.ink, 0, 0.16);
  const head = s.type(['Your company runs on apps.'], { x: W / 2, y: 590, size: 140, at: 0.04, gap: 0.03, center: true, color: C.white });
  return (g, t) => {
    fill(g, C.bg);
    zoomed(g, 1 + 0.18 * t + 2.4 * rush.v, W / 2, H / 2, () => wall(g, 520 * spin.v + 120 * t));
    bar(g);
    head(g);
  };
});

shot('behind', 2.2, s => {
  const cols = [['Notion', C.sky, 0.2], ['Salesforce', C.paper, 0.65], ['QuickBooks', C.lilac, 1.1]].map(([name, color, at]) => ({ name, color, at, lift: s.v(at, 0.28, 'soft'), logo: s.v(0, 0.3, 'pop') }));
  const head = s.type(['Your data sits behind each one.'], { x: W / 2, y: 1010, size: 112, at: 0.1, center: true });
  return (g, t) => {
    cols.forEach((c, i) => {
      const x = i * 640;
      fill(g, c.color, x, 0, 640, H);
      if (c.lift.v > 0) art(g, 'drum', 0.6 * Math.min(1, Math.max(0, (t - c.at) / 0.7)), { x, y: 0, w: 640, h: H, center: [0, 0.75, 0], zoom: 125 + 15 * t });
      const y = -H * c.lift.v;
      fill(g, '#ffffff', x, y, 640, H);
      logo(g, c.name, x + 320, y + H / 2 + 40 * (1 - c.logo.v), 320);
      fill(g, C.ink, x + 638, 0, 3, H);
    });
    head(g);
  };
});

shot('hides', 1.5, 'left', s => {
  const head = s.type(['Every app', 'hides a'], { x: 96, y: 360, size: 280, lead: 270, gap: 0.08 });
  const word = s.type(['database.'], { x: 96, y: 900, size: 280, at: 0.38, color: C.lime });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'drum', 0.1 + 0.35 * t, { x: 1160, y: 0, w: 760, h: H, center: [0, 1.1, 0], zoom: 240 + 40 * t });
    head(g);
    word(g);
  };
});

shot('tables', 2, s => {
  const z = s.v(0.1, 1, 'quart');
  const head = s.type(['A database is just tables.'], { x: 96, y: 1000, size: 130, at: 0.65 });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'drum', 0.45 + t, shifted({ center: [0, 0.3, 0], zoom: mix(620, 150, z.v) + 34 * t }, 0, mix(0, -80, z.v)));
    head(g);
  };
});

shot('linked', 2.6, 'iris', s => {
  const inL = s.v(0, 0.4), inR = s.v(0.08, 0.4);
  const pairs = [[1, 3, 0.55], [3, 1, 1.05], [4, 3, 1.5]].map(([a, b, at]) => ({ a, b, at, p: s.v(at, 0.34) }));
  const left = s.type(['Orders'], { x: 170, y: 230, size: 92, at: 0.12, color: C.white });
  const right = s.type(['Customers'], { x: 1080, y: 230, size: 92, at: 0.12, color: C.white });
  const head = s.type(['Orders are linked to customers.'], { x: W / 2, y: 1020, size: 112, at: 0.85, center: true, color: C.white });
  const rowY = i => 290 + i * 112;
  return (g, t) => {
    fill(g, C.ink);
    zoomed(g, 1 + 0.05 * t, W / 2, H / 2, () => {
      const L = -700 * (1 - inL.v) + 170, R = 1080 + 700 * (1 - inR.v);
      const table = (x, side) => {
        for (let i = 0; i < 5; i++) {
          const on = pairs.some(p => t > p.at + 0.34 && (side === 0 ? p.a : p.b) === i);
          round(g, x, rowY(i), 670, 96, 16);
          g.fillStyle = on ? C.lilac : C.graphite;
          g.fill();
          g.fillStyle = on ? C.ink : C.steel;
          round(g, x + 34, rowY(i) + 34, 150 + 170 * hash(i * 3 + side), 28, 14);
          g.fill();
          round(g, x + 430, rowY(i) + 34, 150, 28, 14);
          g.fill();
        }
      };
      table(L, 0);
      table(R, 1);
      left(g);
      right(g);
      g.lineWidth = 9;
      g.lineCap = 'round';
      g.strokeStyle = C.lilac;
      for (const { a, b, p } of pairs) {
        const x0 = L + 670, y0 = rowY(a) + 48, x1 = R, y1 = rowY(b) + 48;
        curve(g, Array.from({ length: 41 }, (_, i) => {
          const u = i / 40, v = 1 - u;
          return [v ** 3 * x0 + 3 * v * v * u * (x0 + 240) + 3 * v * u * u * (x1 - 240) + u ** 3 * x1, v ** 3 * y0 + 3 * v * v * u * y0 + 3 * v * u * u * y1 + u ** 3 * y1];
        }), p.v);
      }
      g.lineCap = 'butt';
    });
    head(g);
  };
});

shot('reach', 2.5, s => {
  const cam = s.cam({ center: [-3.3, 1.1, 0], zoom: 560 }, { center: [1.2, 2.6, 0], zoom: 142 }, 0.2, 1.5);
  const head = s.type(['Your AI can’t reach', 'your data directly.'], { x: 96, y: 125, size: 120, lead: 125, at: 1.1, gap: 0.03 });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'straw', t, cam());
    head(g);
  };
});

shot('sips', 2.4, s => {
  const head = s.type(['It asks one small question', 'at a time through an'], { x: 96, y: 130, size: 110, lead: 118, gap: 0.03 });
  const swap = s.swap([['API.', 0.5], ['MCP.', 1.45]], { x: 90, y: 560, size: 300 });
  const pan = s.cam({ center: [-0.5, 1.32, 0] }, { center: [-0.9, 1.32, 0] }, 0, 2.4, 'linear');
  return (g, t) => {
    fill(g, C.sky);
    art(g, 'straw', 1 + t, pan({ yaw: 0, pitch: 7, zoom: 1050, line: 3.4 }));
    head(g);
    swap(g);
  };
});

shot('decides', 2.4, 'up', s => {
  const panels = [[C.sky, -16, 1, 215], [C.bg, 0, 1.15, 200], [C.paper, 16, 0.5, 138]].map(([color, x, cy, zoom], i) => ({ color, x, cy, zoom, a: s.v(i * 0.07, 0.3) }));
  const head = s.type(['The app decides what you can do.'], { x: 96, y: 1010, size: 130, at: 0.35, color: C.white });
  return (g, t) => {
    fill(g, C.ink);
    panels.forEach((p, i) => {
      const y = 860 * (1 - p.a.v), x = i * 640;
      fill(g, p.color, x, y, 640, 840);
      art(g, 'limits', t, { x, y, w: 640, h: 840, center: [p.x, p.cy, 0], zoom: p.zoom + 8 * t });
    });
    head(g);
  };
});

shot('ceiling', 1.7, s => {
  const drop = s.v(0.42, 0.2, 'pop'), draw = s.v(0, 1.2, 'linear');
  const limit = s.type(['Their limit'], { x: 96, y: 0, size: 80, at: 0.5, color: C.white, family: SANS });
  const need = s.type(['What you need'], { x: 1270, y: 150, size: 72, at: 0.85, color: C.white, family: SANS });
  const head = s.type(['You hit a limit', 'someone else set.'], { x: 900, y: 800, size: 130, lead: 130, at: 0.3, color: C.white });
  const y = x => 930 - 860 * ((x - 120) / 1680) ** 0.72;
  const pts = Array.from({ length: 81 }, (_, i) => { const x = 120 + 1680 * i / 80; return [x, y(x)]; });
  return (g, t) => {
    fill(g, C.blue);
    const shake = t > 0.62 ? 10 * Math.sin(t * 70) * Math.exp(-(t - 0.62) * 7) : 0;
    const ceil = mix(-30, 400, drop.v) + shake;
    g.lineCap = 'round';
    g.setLineDash([2, 26]);
    g.lineWidth = 11;
    g.strokeStyle = C.lilac;
    curve(g, pts, draw.v);
    g.setLineDash([]);
    g.lineWidth = 14;
    g.strokeStyle = C.white;
    curve(g, pts.map(([x, v]) => [x, Math.max(v, ceil + 8)]), draw.v);
    g.lineCap = 'butt';
    fill(g, C.ink, 0, ceil - 120, W, 120);
    g.save();
    g.translate(0, ceil - 40);
    limit(g);
    g.restore();
    need(g);
    head(g);
  };
});

shot('knows', 2, s => {
  const head = s.type(['This is what', 'your company', 'knows.'], { x: 90, y: 400, size: 165, lead: 170, at: 0.05, gap: 0.05 });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'pour', t + 0.6, shifted({ center: [0, 2.6, 0], zoom: 130 + 12 * t }, 440, 250 - 30 * t));
    head(g);
  };
});

shot('renting', 2.7, s => {
  const cam = s.cam({ yaw: 28, pitch: 22, center: [0, 0.75, 0], zoom: 560 }, { center: [2.5, 3.2, 0], zoom: 86 }, 0.5, 1.5);
  const doors = [[-3, 0], [-2, 0], [-1, 0], [1, 0], [2, 0], [3, 0], [4, 0], [5, 0], [6, 0], [-3, 1], [-2, 1], [-1, 1], [0, 1], [1, 1], [2, 1], [3, 1], [4, 1], [5, 1], [6, 1], [-2, 2], [-1, 2], [0, 2], [1, 2], [2, 2], [3, 2], [4, 2], [5, 2], [6, 2], [-1, 3], [0, 3], [1, 3], [2, 3], [3, 3], [4, 3], [5, 3], [6, 3]]
    .map(([cx, cy], i) => ({ cx, cy, i, a: s.v(1.4 + hash(i) * 0.5, 0.22, 'spring') }));
  const bar = s.bar(0, 770, 1240, 310, C.ink, 0.75);
  const head = s.type(['You’re only renting', 'a place to keep it.'], { x: 80, y: 900, size: 130, lead: 130, at: 0.8, color: C.white, hot: ['renting'], hotColor: C.lilac });
  return (g, t) => {
    fill(g, C.bg);
    const c = cam();
    art(g, 'storage', t, c);
    for (const d of doors) {
      const [x, y] = project([d.cx * 2, 0.8 + d.cy * 1.7, 0.1], c);
      tile(g, APPS[(d.i * 5) % APPS.length], x, y, 92 * d.a.v);
    }
    bar(g);
    head(g);
  };
});

shot('leave', 2.6, s => {
  const p = s.v(0.1, 2, 'linear');
  const up = Array.from({ length: 61 }, (_, i) => { const u = i / 60; return [1090 + 740 * u, 950 - 530 * u ** 1.6]; });
  const dn = Array.from({ length: 61 }, (_, i) => { const u = i / 60; return [1090 + 740 * u, 430 + 520 * u ** 0.6]; });
  const data = s.type(['Your data'], { x: 1420, y: 400, size: 72, at: 1.2, family: SANS, color: C.lime });
  const out = s.type(['Way out'], { x: 1090, y: 395, size: 72, at: 0.3, family: SANS });
  const head = s.type(['Can you take your', 'data and leave?'], { x: 1050, y: 130, size: 112, lead: 120, at: 0.1 });
  return (g, t) => {
    fill(g, C.sky, 0, 0, 1000, H);
    art(g, 'door', t, { x: 0, y: 0, w: 1000, h: H, zoom: 170 + 8 * t });
    fill(g, C.bg, 1000, 0, 920, H);
    fill(g, C.ink, 998, 0, 3, H);
    g.lineCap = 'round';
    g.lineWidth = 14;
    g.strokeStyle = C.lime;
    curve(g, up, p.v);
    g.strokeStyle = C.ink;
    curve(g, dn, p.v);
    g.lineCap = 'butt';
    data(g);
    out(g);
    head(g);
  };
});

shot('another', 1.4, 'iris', s => {
  const head = s.type(['There is another way.'], { x: W / 2, y: 1000, size: 130, at: 0.25, gap: 0.05, center: true, color: C.white });
  const rings = [0.19, 0.57, 0.95].map(at => ({ at, a: s.v(at, 0.45, 'linear'), e: s.v(at, 0.45) }));
  return (g, t) => {
    fill(g, C.blue);
    for (const r of rings) {
      if (r.a.v <= 0 || r.a.v >= 1) continue;
      g.strokeStyle = C.lilac;
      g.lineWidth = 6 * (1 - r.a.v);
      g.beginPath();
      g.ellipse(W / 2, H / 2 + 60, 200 + 640 * r.e.v, 110 + 360 * r.e.v, 0, 0, Math.PI * 2);
      g.stroke();
    }
    art(g, 'key', t, { zoom: 440 });
    head(g);
  };
});

shot('ontology', 2.4, s => {
  const e = s.v(0.4, 1.6, 'soft');
  const head = s.type(['This is your', 'ontology, the map', 'of your business.'], { x: 80, y: 130, size: 110, lead: 112, at: 0.2, hot: ['ontology,'] });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'map', t * 1.05, shifted({ center: [mix(0, 0.3, e.v), 0.25, mix(0, 0.2, e.v)], zoom: mix(235, 275, e.v) }, 310, 160));
    head(g);
  };
});

shot('kinds', 2.2, s => {
  const names = ['type-person', 'type-invoice', 'type-place', 'type-coin', 'type-document', 'type-event', '', '', '', 'type-gavel', 'type-briefcase', 'type-letter', 'type-box', 'type-note', 'type-book'];
  const tones = [C.sky, C.bg, C.paper, C.lilac, C.bg];
  const cells = names.map((name, i) => {
    const cx = i % 5, cy = Math.floor(i / 5);
    return { name, i, x: cx * 384, y: cy * 360, tone: name ? tones[(i * 2 + cy) % 5] : C.ink, a: s.v((cx + cy) * 0.035, 0.22, 'pop') };
  });
  const head = s.type(['It names every kind of', 'thing you work with.'], { x: 420, y: 510, size: 104, lead: 112, at: 0.1, color: C.white });
  return (g, t) => {
    for (const c of cells) {
      g.save();
      g.beginPath();
      g.rect(c.x, c.y, 384, 360);
      g.clip();
      fill(g, c.tone, c.x, c.y, 384, 360);
      if (c.name && c.a.v > 0.3) art(g, c.name, 0, { x: c.x, y: c.y + 360 * (1 - c.a.v), w: 384, h: 360, zoom: 142, yaw: 45 + t * 170 + c.i * 37 });
      g.restore();
    }
    head(g);
  };
});

shot('connect', 2.6, s => {
  const nodes = [
    [560, 300, 'Client', C.lilac, 0], [1090, 180, 'Property', C.sky, 0.22], [1020, 540, 'Invoice', C.lime, 0.42], [1560, 380, 'Contract', C.sky, 0.62, true], [1520, 800, 'File', C.lilac, 0.82],
    [220, 150, '', C.paper, 1], [780, 90, '', C.paper, 1.04], [1420, 120, '', C.lilac, 1.08], [1820, 200, '', C.paper, 1.12], [1820, 620, '', C.sky, 1.16], [1250, 900, '', C.paper, 1.2], [1800, 960, '', C.paper, 1.24], [160, 480, '', C.sky, 1.28], [760, 760, '', C.paper, 1.32],
  ].map(([x, y, label, color, at, above], i) => ({ x, y, label, color, at, above, i, a: s.v(at, 0.22, 'pop'), l: s.v(at + 0.05, 0.28) }));
  const edges = [[0, 1], [0, 2], [1, 3], [2, 3], [3, 4], [2, 4], [5, 0], [6, 1], [7, 1], [8, 3], [9, 3], [9, 4], [10, 4], [11, 4], [12, 0], [13, 2]]
    .map(([a, b]) => ({ a: nodes[a], b: nodes[b], p: s.v(Math.max(nodes[a].at, nodes[b].at) + 0.05, 0.25) }));
  const pull = s.v(0.1, 1), morph = s.v(0, 0.3);
  const bar = s.bar(40, 800, 800, 240, C.bg, 1.1);
  const head = s.type(['It shows how', 'they all connect.'], { x: 70, y: 910, size: 116, lead: 112, at: 1.1, gap: 0.05 });
  return (g, t) => {
    fill(g, C.bg);
    zoomed(g, mix(1.5, 1, pull.v), 560, 300, () => {
      g.strokeStyle = C.ink;
      g.lineWidth = 5;
      for (const { a, b, p } of edges) {
        if (p.v <= 0) continue;
        g.beginPath();
        g.moveTo(a.x, a.y);
        g.lineTo(mix(a.x, b.x, p.v), mix(a.y, b.y, p.v));
        g.stroke();
      }
      for (const n of nodes) {
        if (n.a.v <= 0) continue;
        const r = (n.label ? 74 : 38) * n.a.v + (n.label ? 4 * Math.sin(t * 5 + n.i) : 0);
        const m = n.i === 0 ? morph.v : 1;
        const w = mix(300, r * 2, m), h = mix(90, r * 2, m);
        round(g, n.x - w / 2, n.y - h / 2, w, h, mix(14, r, m));
        g.fillStyle = n.color;
        g.fill();
        g.lineWidth = 4;
        g.strokeStyle = C.ink;
        g.stroke();
        g.lineWidth = 5;
        if (!n.label || n.l.v <= 0) continue;
        font(g, 64);
        const tw = g.measureText(n.label).width;
        const ly = n.above ? n.y - r - 20 : n.y + r + 66;
        g.lineWidth = 26;
        g.strokeStyle = C.bg;
        g.lineJoin = 'round';
        g.strokeText(n.label, n.x - tw / 2, ly);
        g.save();
        g.beginPath();
        g.rect(n.x - tw / 2 - 6, ly - 59, tw + 12, 79);
        g.clip();
        g.fillStyle = C.ink;
        g.fillText(n.label, n.x - tw / 2, ly + 77 * (1 - n.l.v));
        g.restore();
        g.lineWidth = 5;
        g.strokeStyle = C.ink;
      }
    });
    bar(g);
    head(g);
  };
});

shot('own', 1.6, s => {
  const a = s.v(0.3, 0.25);
  const head = s.type(['gives you your own database.'], { x: W / 2, y: 720, size: 130, at: 0.4, center: true });
  const flip = s.v(0, 1.6, 'linear');
  return (g, t) => {
    fill(g, C.bg);
    wall(g, 90 * t, (cx, cy) => { const x = (flip.v * 1.6 - 0.1 - Math.hypot(cx - W / 2, cy - H / 2) / 2400) / 0.22; const u = Math.min(1, Math.max(0, x)); return (1 - u) ** 4; });
    g.save();
    g.beginPath();
    g.rect(0, 230, W, 300);
    g.clip();
    if (a.v > 0) wordmark(g, W / 2 - 520, 260 + 300 * (1 - a.v), 1040, C.ink);
    g.restore();
    head(g);
  };
});

shot('postgres', 2.4, s => {
  const turn = s.v(0, 2.6, 'soft');
  const head = s.type(['It’s real Postgres,', 'and it belongs', 'to you.'], { x: 90, y: 300, size: 130, lead: 135, at: 0.1, gap: 0.05, hot: ['Postgres,'] });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'plot', t, shifted({ center: [0, 0.9, 0], zoom: 152 + 8 * t, yaw: 45 + 14 * turn.v }, 400, 20));
    head(g);
  };
});

shot('wherever', 2.1, s => {
  const hop = s.keys(0, [[1, 0.6, 'soft', 0.28], [2, 0.6, 'soft', 0.32]]);
  const head = s.type(['It runs wherever you choose.'], { x: W / 2, y: 135, size: 140, at: 0.1, gap: 0.05, center: true });
  return (g, t) => {
    fill(g, C.sky);
    art(g, 'places', t, { yaw: 25, pitch: 24, center: [hop.v * 7 + 0.6 * Math.sin(t * 2), 4.45, 0], zoom: 118 });
    head(g);
  };
});

shot('code', 2.4, s => {
  const code = ['const invoices =', '  await archive.list("invoice");', 'for (const i of invoices)', '  await archive.update("invoice",', '    i.id, { paid: true });', '', 'await archive.migrate(sql);'];
  const total = code.reduce((n, l) => n + l.length + 2, 0);
  const typed = s.v(0, total / 95, 'linear', 0, total);
  const head = s.type(['Your AI writes', 'code right on', 'your data.'], { x: 1240, y: 790, size: 100, lead: 100, at: 0.1 });
  return (g, t) => {
    fill(g, C.ink);
    fill(g, C.sky, 1200, 0, 720, H);
    art(g, 'bot', t, { x: 1200, y: -190, w: 720, h: H, zoom: 195 + 10 * t });
    font(g, 68, SANS);
    g.letterSpacing = '0px';
    let left = Math.floor(typed.v);
    code.forEach((line, i) => {
      const y = 190 + i * 112;
      const shown = line.slice(0, Math.max(0, left));
      left -= line.length + 2;
      g.fillStyle = line.includes('await') ? C.lilac : C.white;
      g.fillText(shown, 74, y);
      if (left <= 0 && left + line.length + 2 > 0) fill(g, C.white, 78 + g.measureText(shown).width, y - 48, 26, 62);
    });
    head(g);
  };
});

shot('records', 2.2, s => {
  const z = s.v(0, 1.5, 'soft'), front = s.v(0.15, 1.85, 'linear');
  const bar = s.bar(130, 390, 1660, 330, C.ink, 0.4, 0.16);
  const head = s.type(['It changes a hundred thousand', 'records in seconds.'], { x: W / 2, y: 530, size: 120, lead: 135, at: 0.45, gap: 0.03, center: true, color: C.white });
  const nx = 400, ny = 250;
  const small = Object.assign(document.createElement('canvas'), { width: nx, height: ny });
  const sg = small.getContext('2d');
  const img = sg.createImageData(nx, ny);
  const cols = [[244, 245, 247], [102, 174, 245], [26, 64, 161]];
  return g => {
    const zoom = mix(14, 1, z.v), f = 4200 * front.v;
    for (let cy = 0; cy < ny; cy++) for (let cx = 0; cx < nx; cx++) {
      const a = f - Math.hypot((cx - 200) * 4.8, (cy - 125) * 4.32) - ((cx * 7 + cy * 13) % 11) * 12;
      const c = a < 0 ? cols[0] : a < 170 ? cols[1] : Math.floor((a - 170) / 560) % 2 ? cols[1] : cols[2];
      const i = (cy * nx + cx) * 4;
      img.data[i] = c[0];
      img.data[i + 1] = c[1];
      img.data[i + 2] = c[2];
      img.data[i + 3] = 255;
    }
    sg.putImageData(img, 0, 0);
    const cw = W / nx * zoom, ch = H / ny * zoom, x0 = W / 2 - nx / 2 * cw, y0 = H / 2 - ny / 2 * ch;
    g.imageSmoothingEnabled = false;
    g.drawImage(small, x0, y0, nx * cw, ny * ch);
    g.imageSmoothingEnabled = true;
    if (zoom > 2) {
      g.fillStyle = C.ink;
      g.globalAlpha = Math.min(1, (zoom - 2) / 3);
      for (let i = Math.ceil(-x0 / cw); x0 + i * cw < W; i++) g.fillRect(x0 + i * cw, 0, 1.5, H);
      for (let i = Math.ceil(-y0 / ch); y0 + i * ch < H; i++) g.fillRect(0, y0 + i * ch, W, 1.5);
      g.globalAlpha = 1;
    }
    bar(g);
    head(g);
  };
});

shot('direct', 2.2, s => {
  const z = s.v(0.1, 1.8, 'soft');
  const bar = s.bar(0, 790, 1040, 290, C.ink, 0.4, 0.16);
  const head = s.type(['Now your AI reaches', 'your data directly.'], { x: 70, y: 910, size: 110, lead: 118, at: 0.45, color: C.white });
  return (g, t) => {
    fill(g, C.bg);
    art(g, 'straw', 19.75 + t, { center: [mix(0.4, 0.7, z.v), mix(1.2, 1.7, z.v), 0], zoom: mix(400, 132, z.v) });
    bar(g);
    head(g);
  };
});

shot('struck', 1.8, s => {
  const head = s.strike([['There’s no ', 'asking,', ''], ['no ', 'waiting,', ''], ['no ', 'limit', ' someone'], ['', '', 'else set.']], { x: 110, y: 240, size: 200, lead: 225, gap: 0.3, color: C.white, line: C.lilac });
  return g => {
    fill(g, C.ink);
    head(g);
  };
});

shot('more', 1.3, 'up', s => {
  const label = s.type(['Through the app'], { x: 90, y: 140, size: 84, family: SANS });
  const head = s.type(['Now far more gets done.'], { x: 660, y: 400, size: 130, at: 0.05 });
  const flow = s.v(0, 1.3, 'linear', 0, 1170);
  const lane = (g, y, h) => { fill(g, C.mint, 0, y, W, h); fill(g, C.ink, 0, y, W, 4); fill(g, C.ink, 0, y + h - 4, W, 4); };
  return g => {
    fill(g, C.bg);
    lane(g, 170, 40);
    lane(g, 450, 480);
    label(g);
    wordmark(g, 90, 300, 480, C.lime);
    head(g);
    round(g, (flow.v % 1100) - 40, 176, 56, 28, 14);
    g.fillStyle = C.lime;
    g.fill();
    for (let r = 0; r < 6; r++) for (let c = 0; c < 18; c++) {
      const x = ((c * 113.3 + r * 37 + flow.v * (1 + 0.04 * r)) % (W + 120)) - 110;
      round(g, x, 470 + r * 74, 92, 60, 10);
      g.fillStyle = [C.lime, C.lilac, C.blue][(r + c) % 3];
      g.fill();
      g.lineWidth = 3;
      g.strokeStyle = C.ink;
      g.stroke();
    }
  };
});

shot('faster', 1.5, s => {
  const head = s.type(['That’s why it’s'], { x: W / 2, y: 170, size: 130, center: true, color: C.white });
  const ten = s.v(0.2, 0.25);
  const word = s.type(['faster.'], { x: W / 2, y: 970, size: 260, at: 0.4, center: true, color: C.white });
  return (g, t) => {
    fill(g, C.blue);
    head(g);
    if (t > 0.2) zoomed(g, mix(1.25, 1, ten.v) + 0.03 * t, W / 2, 500, () => {
      font(g, 520);
      g.fillStyle = C.white;
      g.fillText('10x', (W - g.measureText('10x').width) / 2, 660);
    });
    word(g);
  };
});

shot('sync', 2.2, s => {
  const names = ['Salesforce', 'Notion', 'QuickBooks', 'Shopify', 'Slack', 'Stripe', 'HubSpot', 'Gmail'].map((name, i) => ({ name, i, a: s.v(0.1 + i * 0.04, 0.26, 'spring') }));
  const head = s.type(['Your apps sync', 'into your own', 'database.'], { x: 50, y: 110, size: 96, lead: 98, at: 0.1 });
  return (g, t) => {
    fill(g, C.bg);
    const cam = shifted({ center: [0, 0.7, 0], zoom: 130 + 10 * t, yaw: 45 + 10 * t }, 170, 0);
    art(g, 'dock', t, cam);
    for (const n of names) {
      const a = n.i * Math.PI / 4;
      const [x, y] = project([Math.cos(a) * 4.7, 0.3, Math.sin(a) * 4.7], cam);
      tile(g, n.name, x, y, 150 * n.a.v);
    }
    head(g);
  };
});

shot('door', 2.6, s => {
  const z = s.v(1.75, 0.85, 'in');
  const head = s.type(['You can take it', 'and leave', 'whenever you like.'], { x: 1150, y: 420, size: 104, lead: 118, at: 0.05, gap: 0.03, leave: 1.8, color: C.ink });
  return (g, t) => {
    fill(g, C.blue);
    art(g, 'lock', t * 0.75, { yaw: 32, pitch: 14, center: [mix(1, 0.45, z.v), 1.8, mix(0, -0.1, z.v)], zoom: mix(285, 2600, z.v) });
    head(g);
  };
});

shot('end', 2.8, s => {
  const a = s.type(['Own your'], { x: 96, y: 290, size: 280, at: 0.05, color: C.white });
  const b = s.type(['data'], { x: 96, y: 565, size: 280, at: 0.3, color: C.white });
  const c = s.type(['infrastructure.'], { x: 96, y: 840, size: 280, at: 0.55, color: C.white });
  const name = s.v(1.1, 0.3);
  return (g, t) => {
    fill(g, C.blue);
    zoomed(g, 1 + 0.012 * t, W / 2, H / 2, () => { a(g); b(g); c(g); });
    if (t > 0.5) art(g, 'key', 1 + (t - 0.5) * 2.2, { x: 720, y: 290, w: 760, h: 340, zoom: 240, center: [0, 0, 0] });
    g.save();
    g.beginPath();
    g.rect(1320, 890, 520, 150);
    g.clip();
    if (name.v > 0) wordmark(g, 1340, 925 + 130 * (1 - name.v), 480, C.white);
    g.restore();
  };
});
