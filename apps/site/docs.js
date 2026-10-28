(() => {
  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const INK = "#141413", WHITE = "#ffffff", T1 = "#f3f3f3", T2 = "#e4e4e4", LINE = "#c9c9c9", MID = "#8c8c8c";
  const G = 12, P = 8, STEP = 140;
  const font = w => `${w} 17px Geist, ui-sans-serif, system-ui, sans-serif`;
  const seeded = seed => () => ((seed = (seed * 48271) % 2147483647) / 2147483647);

  function toc() {
    const nav = document.querySelector(".toc");
    if (!nav) return;
    const links = [...nav.querySelectorAll("a.tg")];
    const picker = document.querySelector(".picker select");
    const targets = links.flatMap((l, i) => l.dataset.ids.split(" ").map(id => ({ el: document.getElementById(id), i })));
    let current = -1;
    const mark = () => {
      const line = Math.min(160, innerHeight * 0.3);
      let at = targets[0];
      targets.forEach(t => { if (t.el.getBoundingClientRect().top <= line) at = t; });
      if (innerHeight + scrollY >= document.documentElement.scrollHeight - 4) at = targets[targets.length - 1];
      if (at.i === current) return;
      current = at.i;
      links.forEach((l, i) => l.classList.toggle("on", i === current));
      if (picker) picker.value = links[current].hash.slice(1);
    };
    addEventListener("scroll", mark, { passive: true });
    addEventListener("resize", mark);
    if (picker) picker.addEventListener("change", () => {
      const s = document.getElementById(picker.value);
      if (s) { s.scrollIntoView({ behavior: reduced ? "auto" : "smooth" }); history.replaceState(null, "", "#" + picker.value); }
    });
    mark();
    const target = location.hash && document.getElementById(location.hash.slice(1));
    if (target && document.fonts) document.fonts.ready.then(() => { target.scrollIntoView({ behavior: "instant" }); mark(); });
  }

  function walk(points) {
    const out = [points[0]];
    for (let i = 1; i < points.length; i++) {
      let [x, y] = out[out.length - 1];
      const [tx, ty] = points[i];
      while (x !== tx || y !== ty) {
        if (x !== tx) x += Math.sign(tx - x); else y += Math.sign(ty - y);
        out.push([x, y]);
      }
    }
    return out;
  }

  function board(el) {
    const c = el.querySelector("canvas");
    const b = { c, ctx: c.getContext("2d"), cols: 0, rows: 0, ox: 0, oy: 0 };
    b.size = rows => {
      const w = c.clientWidth;
      const d = Math.min(2, devicePixelRatio || 1);
      b.cols = Math.floor(w / G);
      b.rows = rows;
      b.ox = Math.floor((w - b.cols * G) / 2);
      b.oy = 0;
      c.style.height = rows * G + "px";
      c.width = Math.round(w * d);
      c.height = Math.round(rows * G * d);
      b.ctx.setTransform(d, 0, 0, d, 0, 0);
    };
    b.fit = items => {
      let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
      items.forEach(o => {
        const [x, y, w, h] = Array.isArray(o) ? [o[0], o[1], 1, 1] : [o.x, o.y, o.w, o.h];
        x0 = Math.min(x0, x); y0 = Math.min(y0, y); x1 = Math.max(x1, x + w); y1 = Math.max(y1, y + h);
      });
      const w = c.clientWidth, hpx = (y1 - y0) * G;
      const d = Math.min(2, devicePixelRatio || 1);
      c.style.height = hpx + "px";
      c.width = Math.round(w * d);
      c.height = Math.round(hpx * d);
      b.ctx.setTransform(d, 0, 0, d, 0, 0);
      b.ox = Math.round((w - (x1 - x0) * G) / 2 - x0 * G);
      b.oy = -y0 * G;
    };
    b.clear = () => b.ctx.clearRect(0, 0, c.width, c.height);
    b.px = (gx, gy) => [b.ox + gx * G + G / 2, b.oy + gy * G + G / 2];
    b.pixel = (gx, gy, color = INK) => {
      const [x, y] = b.px(gx, gy);
      b.ctx.fillStyle = color;
      b.ctx.beginPath();
      b.ctx.roundRect(x - P / 2, y - P / 2, P, P, 2.5);
      b.ctx.fill();
    };
    b.tile = (t, fill = WHITE, text = INK, stroke = null) => {
      const ctx = b.ctx, x = b.ox + t.x * G, y = b.oy + t.y * G, w = t.w * G, h = t.h * G;
      ctx.fillStyle = fill;
      ctx.beginPath();
      ctx.roundRect(x + 1, y + 1, w - 2, h - 2, 14);
      ctx.fill();
      if (stroke) { ctx.strokeStyle = stroke; ctx.lineWidth = 2; ctx.stroke(); }
      if (t.label) {
        ctx.fillStyle = text;
        ctx.font = font(t.bold ? 600 : 500);
        ctx.textAlign = t.align || "center";
        ctx.textBaseline = "middle";
        const tx = t.align === "left" ? x + (t.pad || 16) : x + w / 2 + (t.shift || 0);
        ctx.fillText(t.label, tx, y + (t.ly != null ? t.ly * G + G / 2 : h / 2) + 1);
      }
    };
    b.text = (label, gx, gy, align = "left", w = 500, color = INK) => {
      const ctx = b.ctx;
      ctx.fillStyle = color;
      ctx.font = font(w);
      ctx.textAlign = align;
      ctx.textBaseline = "middle";
      ctx.fillText(label, b.ox + gx * G, b.oy + gy * G + G / 2 + 1);
    };
    b.line = cells => {
      const ctx = b.ctx;
      ctx.strokeStyle = LINE;
      ctx.lineWidth = 1.5;
      ctx.lineJoin = "round";
      ctx.beginPath();
      cells.forEach(([gx, gy], i) => { const [x, y] = b.px(gx, gy); i ? ctx.lineTo(x, y) : ctx.moveTo(x, y); });
      ctx.stroke();
    };
    b.measure = (label, w = 500) => { b.ctx.font = font(w); return Math.ceil((b.ctx.measureText(label).width + 32) / G); };
    return b;
  }

  function lock(b, gx, gy, open, color) {
    const ctx = b.ctx, [cx, cy] = b.px(gx, gy);
    ctx.fillStyle = color;
    ctx.strokeStyle = color;
    ctx.lineWidth = 2.5;
    ctx.beginPath();
    ctx.roundRect(cx - 8, cy - 2, 16, 12, 3);
    ctx.fill();
    const lift = open ? 5 : 0;
    ctx.beginPath();
    ctx.moveTo(cx - 4.5, cy - 2 - (open ? lift : 0));
    ctx.lineTo(cx - 4.5, cy - 6 - lift);
    ctx.arc(cx, cy - 6 - lift, 4.5, Math.PI, 0);
    ctx.lineTo(cx + 4.5, cy - 2 - lift);
    ctx.stroke();
  }

  const ICONS = {
    web: new Path2D("M3.5 6.5A2.5 2.5 0 0 1 6 4h12a2.5 2.5 0 0 1 2.5 2.5v11A2.5 2.5 0 0 1 18 20H6a2.5 2.5 0 0 1-2.5-2.5zM3.5 9h17"),
    code: new Path2D("M8.5 7l-5 5 5 5M15.5 7l5 5-5 5"),
    ai: new Path2D("M12 3.5l2 6.5 6.5 2-6.5 2-2 6.5-2-6.5-6.5-2 6.5-2z")
  };

  function icon(b, name, x, y, color) {
    const ctx = b.ctx;
    ctx.save();
    ctx.translate(x, y);
    ctx.scale(20 / 24, 20 / 24);
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    ctx.stroke(ICONS[name]);
    ctx.restore();
  }

  function arch(el) {
    const b = board(el);
    const R = seeded(7);
    let L;
    const layout = () => {
      b.size(1);
      const C = b.cols, wide = C >= 40;
      const th = wide ? 5 : 7, gap = wide ? 2 : 1;
      const tw = Math.floor((C - gap * 2) / 3);
      const x0 = Math.floor((C - (tw * 3 + gap * 2)) / 2);
      const names = [["web", "Web app"], ["code", "Your code"], ["ai", "Your AI"]];
      const tiles = names.map(([ic, label], i) => ({ x: x0 + i * (tw + gap), y: 0, w: tw, h: th, ic, label }));
      const exits = tiles.map(t => t.x + Math.floor(t.w / 2));
      const py = th + 3, perm = { x: x0, y: py, w: tw * 3 + gap * 2, h: 5 };
      const gateRow = py + 3, route = py + 7, by = py + 9;
      const block = { x: x0, y: by, w: perm.w, h: 11 };
      const sw = Math.floor((block.w - 3) / 2);
      const stores = [
        { x: block.x + 1, y: by + 1, w: sw, h: 9, label: "Your tables", ly: 1, kind: "table" },
        { x: block.x + block.w - 1 - sw, y: by + 1, w: sw, h: 9, label: "Your files", ly: 1, kind: "files" }
      ];
      stores.forEach(st => {
        st.cx = st.x + Math.floor(st.w / 2);
        if (st.kind === "table") {
          const cw = Math.min(5, st.w - 2);
          st.cells = [];
          for (let r = 0; r < 4; r++) for (let c = 0; c < cw; c++) st.cells.push([st.cx - Math.floor(cw / 2) + c, st.y + 3 + r, r]);
          st.parts = 3;
        } else {
          const pw = wide ? 3 : 2, total = pw * 3 + 2, left = st.cx - Math.floor(total / 2);
          st.cells = [];
          for (let p = 0; p < 3; p++) for (let r = 0; r < 4; r++) for (let c = 0; c < pw; c++) st.cells.push([left + p * (pw + 1) + c, st.y + 3 + r, p]);
          st.parts = 3;
        }
      });
      const paths = tiles.map((t, i) => stores.map(st => walk([[exits[i], t.y + t.h], [exits[i], route], [st.cx, route], [st.cx, by]])));
      const lines = [
        ...exits.map(x => walk([[x, th], [x, route]])),
        walk([[Math.min(...exits, ...stores.map(s => s.cx)), route], [Math.max(...exits, ...stores.map(s => s.cx)), route]]),
        ...stores.map(st => walk([[st.cx, route], [st.cx, by]]))
      ];
      L = { tiles, perm, block, stores, paths, lines, gateRow, exits, wide };
      b.fit([...tiles, perm, block]);
      const every = 9, count = 12;
      L.reqs = [];
      for (let j = 0; j < count; j++) {
        const tile = Math.floor(R() * 3), store = Math.floor(R() * 2);
        const path = paths[tile][store];
        const g = path.findIndex(([, y]) => y === gateRow);
        L.reqs.push({ start: j * every, tile, store, part: Math.floor(R() * 3), path, g, refused: j % 5 === 3 });
      }
      L.period = count * every;
    };
    const draw = tick => {
      b.clear();
      const k = reduced ? 30 : tick % L.period;
      const open = new Set(), deny = new Set(), lit = [new Set(), new Set()], home = new Set(), dots = [];
      L.reqs.forEach(q => {
        let s = k - q.start;
        if (s < 0) s += L.period;
        const n = q.path.length, gx = q.path[q.g][0];
        if (q.refused) {
          const dur = (q.g - 1) * 2 + 4;
          if (s >= dur) return;
          if (s < q.g) dots.push([q.path[s], INK]);
          else if (s < q.g + 3) { dots.push([q.path[q.g - 1], INK]); if ((s - q.g) % 2 === 0) deny.add(gx); }
          else dots.push([q.path[Math.max(0, q.g - 1 - (s - q.g - 3))], MID]);
          return;
        }
        const dur = n * 2 + 7;
        if (s >= dur) return;
        if (s < n) { dots.push([q.path[s], INK]); if (Math.abs(s - q.g) <= 1) open.add(gx); }
        else if (s < n + 4) lit[q.store].add(q.part);
        else if (s < n * 2 + 4) { const i = n - 1 - (s - n - 4); dots.push([q.path[i], MID]); if (Math.abs(i - q.g) <= 1) open.add(gx); }
        else home.add(q.tile);
      });
      L.lines.forEach(l => b.line(l));
      L.tiles.forEach((t, i) => {
        const on = home.has(i), fg = on ? WHITE : INK;
        b.tile({ x: t.x, y: t.y, w: t.w, h: t.h }, on ? INK : WHITE);
        const cx = b.ox + (t.x + t.w / 2) * G, top = b.oy + t.y * G;
        b.ctx.font = font(500);
        b.ctx.fillStyle = fg;
        b.ctx.textBaseline = "middle";
        if (L.wide) {
          const tw = b.ctx.measureText(t.label).width, gx = cx - (20 + 8 + tw) / 2;
          icon(b, t.ic, gx, top + t.h * G / 2 - 10, fg);
          b.ctx.textAlign = "left";
          b.ctx.fillText(t.label, gx + 28, top + t.h * G / 2 + 1);
        } else {
          icon(b, t.ic, cx - 10, top + 14, fg);
          b.ctx.textAlign = "center";
          b.ctx.fillText(t.label, cx, top + 58);
        }
      });
      b.tile({ ...L.perm, label: "Permissions", align: "left", ly: 1 });
      for (let x = L.perm.x + 1; x < L.perm.x + L.perm.w - 1; x++) {
        if (open.has(x)) continue;
        b.pixel(x, L.gateRow, deny.has(x) ? MID : INK);
      }
      b.tile(L.block);
      L.stores.forEach((st, i) => {
        b.tile(st, T1);
        st.cells.forEach(([x, y, part]) => {
          const on = st.kind === "table" ? part > 0 && lit[i].has(part - 1) : lit[i].has(part);
          b.pixel(x, y, on ? INK : st.kind === "table" && part === 0 ? MID : T2);
        });
      });
      dots.forEach(([c, color]) => b.pixel(c[0], c[1], color));
    };
    return { layout, draw };
  }

  function key(el) {
    const b = board(el);
    let L;
    const layout = () => {
      b.size(1);
      const wide = b.cols >= 46;
      const req = wide ? "Your request" : "Request";
      const rooms = wide ? ["finance space", "sales space"] : ["finance", "sales"];
      if (wide) {
        const rw = b.measure(req), mw = Math.max(...rooms.map(n => b.measure(n))) + 3;
        const r = { x: 5, y: 6, w: rw, h: 5, label: req };
        const sp = [{ x: b.cols - 1 - mw, y: 1, w: mw, h: 5, label: rooms[0], shift: 14 }, { x: b.cols - 1 - mw, y: 11, w: mw, h: 5, label: rooms[1], shift: 14 }];
        const exit = [r.x + rw, 8], mx = Math.floor((r.x + rw + sp[0].x) / 2);
        L = {
          rows: 17, r, sp, locks: sp.map(s => [s.x + 2, s.y + 2]),
          into: walk([[0, 8], [r.x + 2, 8]]),
          to: sp.map(s => walk([exit, [mx, 8], [mx, s.y + 2], [s.x - 1, s.y + 2]])),
          lines: sp.map(s => walk([exit, [mx, 8], [mx, s.y + 2], [s.x - 1, s.y + 2]]))
        };
      } else {
        const rw = b.measure(req), mw = Math.max(...rooms.map(n => b.measure(n))) + 3;
        const gapX = Math.max(2, b.cols - mw * 2);
        const sp = [{ x: 0, y: 12, w: mw, h: 5, label: rooms[0], shift: 14 }, { x: mw + gapX, y: 12, w: mw, h: 5, label: rooms[1], shift: 14 }];
        const rwo = rw % 2 ? rw : rw + 1;
        const r = { x: Math.floor((mw * 2 + gapX) / 2) - Math.floor(rwo / 2), y: 1, w: rwo, h: 5, label: req };
        const cx = r.x + Math.floor(r.w / 2);
        const doors = sp.map(s => s.x + Math.floor(s.w / 2));
        L = {
          rows: 18, r, sp, locks: sp.map(s => [s.x + 2, s.y + 2]),
          into: walk([[r.x - 4, 3], [r.x + 2, 3]]),
          to: sp.map((s, i) => walk([[cx, r.y + r.h], [cx, 8], [doors[i], 8], [doors[i], s.y - 1]])),
          lines: sp.map((s, i) => walk([[cx, r.y + r.h], [cx, 8], [doors[i], 8], [doors[i], s.y - 1]]))
        };
      }
      b.fit([L.r, ...L.sp, ...L.into, ...L.to.flat()]);
      const a = L.into.length, f = L.to[0].length, s = L.to[1].length;
      L.t = { hold1: a, go1: a + 3, open: a + 3 + f, hold2: a + 3 + f + 4, go2: a + 3 + f + 7, stop: a + 3 + f + 7 + s - 1 };
      L.period = L.t.stop + 16;
    };
    const draw = tick => {
      b.clear();
      const k = reduced ? L.t.open + 2 : tick % L.period, T = L.t;
      L.lines.forEach(l => b.line(l));
      const holding = (k >= T.hold1 && k < T.go1) || (k >= T.hold2 && k < T.go2);
      b.tile(L.r, holding ? INK : WHITE, holding ? WHITE : INK);
      const open = k >= T.open;
      b.tile(L.sp[0], open ? WHITE : T2, INK, open ? INK : null);
      b.tile(L.sp[1], T2, INK);
      lock(b, ...L.locks[0], open, INK);
      const blink = k >= T.stop && k < T.stop + 6 && (k - T.stop) % 2 === 0;
      lock(b, ...L.locks[1], false, blink ? MID : INK);
      let at = null;
      if (k < T.hold1) at = L.into[k];
      else if (k >= T.go1 && k < T.open) at = L.to[0][k - T.go1];
      else if (k >= T.open && k < T.hold2) at = L.to[0][L.to[0].length - 1];
      else if (k >= T.go2 && k <= T.stop) at = L.to[1][Math.min(k - T.go2, L.to[1].length - 2)];
      else if (k > T.stop && k < T.stop + 8) at = L.to[1][Math.max(0, L.to[1].length - 2 - Math.floor((k - T.stop) / 2))];
      if (at) b.pixel(...at);
    };
    return { layout, draw };
  }

  function code(el) {
    const b = board(el);
    let L;
    const layout = () => {
      b.size(1);
      const aw = 5, rw = 7;
      const span = Math.min(b.cols, aw + 18 + rw), x0 = Math.floor((b.cols - span) / 2);
      const lane = y => {
        const ai = { x: x0, y, w: aw, h: 5 };
        const ar = { x: x0 + span - rw, y, w: rw, h: 5 };
        const row = y + 2;
        return { ai, ar, row, path: walk([[ai.x + aw, row], [ar.x - 1, row]]), steps: [0, 1, 2].map(i => [ar.x + 2 + i, row]) };
      };
      L = { a: lane(0), b: lane(7) };
      b.fit([L.a.ai, L.a.ar, L.b.ai, L.b.ar]);
      const n = L.a.path.length;
      L.trip = n * 2 + 3;
      L.calls = 4;
      L.period = L.trip * L.calls + 14;
      const s = L.b;
      s.write = 8;
      s.go = s.write + 2;
      s.cross = Math.max(0, n - 3);
      s.run = s.go + s.cross;
      s.back = s.run + 4;
      s.done = s.back + n;
    };
    const lanePic = (ln, aiOn, lit) => {
      b.line(ln.path);
      b.tile(ln.ai, aiOn ? INK : WHITE);
      b.tile(ln.ar);
      ln.steps.forEach(([x, y], i) => b.pixel(x, y, lit[i] ? INK : T2));
    };
    const draw = tick => {
      b.clear();
      const k = reduced ? L.period - 1 : tick % L.period;
      const A = L.a, n = A.path.length;
      const call = Math.min(L.calls - 1, Math.floor(k / L.trip)), s = k - call * L.trip, busy = k < L.trip * L.calls;
      const litA = [false, false, false];
      let dotA = null, back = false, aiA = false;
      if (busy) {
        if (s < n) dotA = A.path[s];
        else if (s < n + 2) litA[call % 3] = true;
        else if (s < n * 2 + 2) { dotA = A.path[n - 1 - (s - n - 2)]; back = true; }
        else aiA = true;
      }
      lanePic(A, aiA, litA);
      if (dotA) b.pixel(...dotA, back ? MID : INK);
      const B = L.b;
      const running = k >= B.run && k < B.back;
      const done = k >= B.done;
      lanePic(B, done && k < B.done + 4, [running, running, running]);
      if (k < B.run) {
        const bx = B.ai.x + B.ai.w + (k < B.go ? 0 : Math.min(B.cross, k - B.go)), by = B.row - 1;
        const cells = [[0, 0], [1, 0], [2, 0], [0, 1], [1, 1], [0, 2], [1, 2], [2, 2]];
        cells.slice(0, Math.min(k + 1, cells.length)).forEach(([i, j]) => {
          const [x, y] = b.px(bx + i, by + j);
          b.ctx.fillStyle = INK;
          b.ctx.beginPath();
          b.ctx.roundRect(x - 3, y - 3, 6, 6, 2);
          b.ctx.fill();
        });
      }
      if (k >= B.back && k < B.done) b.pixel(...B.path[B.path.length - 1 - (k - B.back)], MID);
    };
    return { layout, draw };
  }

  function crud(el) {
    const req = el.querySelector(".crud-req");
    const res = el.querySelector(".crud-res");
    const rows = el.querySelector(".crud-rows");
    const steps = [
      { req: "GET /api/invoice", res: "2 rows", rows: 2 },
      { req: "POST /api/invoice\n{\"customer\": \"Hart\", \"amount\": \"€ 980\"}", res: "201, the new row", rows: 3, mark: "new" },
      { req: "GET /api/invoice/{id}", res: "the Hart row", rows: 3, hi: 2 },
      { req: "PATCH /api/invoice/{id}\n{\"status\": \"paid\"}", res: "the changed row", rows: 3, paid: true, cell: true },
      { req: "DELETE /api/invoice/{id}", res: "204, gone", rows: 3, hi: 2, gone: true },
      { req: "GET /api/search?q=north", res: "1 hit", rows: 2, hi: 0 }
    ];
    const data = [["Northwind", "€4,200", "unpaid"], ["Acme", "€1,350", "paid"], ["Hart", "€980", "unpaid"]];
    const per = 14;
    const draw = tick => {
      const i = reduced ? 3 : Math.floor(tick / per) % steps.length, s = steps[i], sub = tick % per;
      req.textContent = s.req;
      res.textContent = s.res;
      let shown = s.rows;
      if (s.gone && sub >= per / 2) shown = 2;
      rows.innerHTML = "";
      data.slice(0, shown).forEach((r, j) => {
        const row = document.createElement("div");
        row.className = "crud-row";
        if (s.hi === j) row.classList.add("hi");
        if (s.mark === "new" && j === 2) row.classList.add("new");
        r.forEach((v, c) => {
          const cell = document.createElement("span");
          cell.textContent = j === 2 && c === 2 && s.paid ? "paid" : v;
          if (s.cell && j === 2 && c === 2) cell.className = "hi";
          row.appendChild(cell);
        });
        rows.appendChild(row);
      });
    };
    return { draw };
  }

  function schema(el) {
    const say = el.querySelector(".say-text");
    const tbl = el.querySelector(".sch-table");
    const due = [...el.querySelectorAll(".due")];
    const plan = el.querySelector(".sch-plan");
    const ask = say.dataset.text;
    const typed = Math.ceil(ask.length / 2);
    const period = typed + 4 + due.length * 2 + 26;
    const draw = tick => {
      const k = reduced ? period - 1 : tick % period;
      say.textContent = ask.slice(0, Math.min(ask.length, k * 2));
      const shown = Math.max(0, Math.floor((k - typed - 4) / 2) + 1);
      tbl.classList.toggle("grow", shown > 0);
      due.forEach((d, i) => d.classList.toggle("in", i < shown));
      plan.classList.toggle("in", shown > due.length);
    };
    return { draw };
  }

  const kinds = { arch, key, code, crud, schema };
  const live = [];
  document.querySelectorAll("[data-viz]").forEach(el => {
    const v = kinds[el.dataset.viz](el);
    v.el = el;
    v.tick = 0;
    v.on = false;
    const paint = () => { v.layout && v.layout(); v.draw(v.tick); };
    new ResizeObserver(paint).observe(el);
    new IntersectionObserver(([e]) => { v.on = e.isIntersecting; if (v.on) start(); }).observe(el);
    if (document.fonts) document.fonts.ready.then(paint);
    live.push(v);
  });

  let raf = 0, last = 0, acc = 0;
  function frame(t) {
    raf = 0;
    const on = live.filter(v => v.on);
    if (!on.length || reduced) return;
    acc += Math.min(500, t - (last || t));
    last = t;
    while (acc >= STEP) {
      acc -= STEP;
      on.forEach(v => { v.tick++; v.draw(v.tick); });
    }
    raf = requestAnimationFrame(frame);
  }
  function start() { if (!raf && !reduced) { last = 0; raf = requestAnimationFrame(frame); } }
  document.addEventListener("visibilitychange", () => { if (!document.hidden) start(); });

  toc();
})();
