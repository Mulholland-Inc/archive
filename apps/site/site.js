(() => {
  const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const INK = "#141413";
  const T3 = "#dadada";
  const clamp = (v, a = 0, b = 1) => Math.min(b, Math.max(a, v));
  const lerp = (a, b, t) => a + (b - a) * t;
  const ease = t => 1 - Math.pow(1 - t, 3);
  const seeded = seed => () => ((seed = (seed * 48271) % 2147483647) / 2147483647);
  const range = (p, a, b) => clamp((p - a) / (b - a));

  function progress(el, a, b) {
    const r = el.getBoundingClientRect(), vh = innerHeight;
    return clamp((vh - r.top) / (vh * a + r.height * b));
  }

  function fitCanvas(c) {
    const r = c.getBoundingClientRect();
    const d = Math.min(2, devicePixelRatio || 1);
    c.width = Math.round(r.width * d);
    c.height = Math.round(r.height * d);
    const ctx = c.getContext("2d");
    ctx.setTransform(d, 0, 0, d, 0, 0);
    return { ctx, w: r.width, h: r.height };
  }

  const scenes = [];
  function scene(el, s) {
    s.el = el;
    s.on = false;
    s.p = reduced ? 1 : 0;
    scenes.push(s);
    new IntersectionObserver(([e]) => { s.on = e.isIntersecting; if (s.on) kick(); }, { rootMargin: "200px" }).observe(el);
    new ResizeObserver(() => { s.layout && s.layout(); s.draw(performance.now(), true); }).observe(el);
  }

  let raf = 0;
  function loop(t) {
    raf = 0;
    let any = false;
    scenes.forEach(s => {
      if (!s.on) return;
      any = true;
      const goal = reduced ? 1 : s.target();
      s.p += (goal - s.p) * (s.snap || 0.16);
      if (Math.abs(goal - s.p) < 0.0005) s.p = goal;
      s.draw(t, false);
    });
    if (any && !reduced && !raf) raf = requestAnimationFrame(loop);
  }
  function kick() { if (!raf && !reduced) raf = requestAnimationFrame(loop); }
  addEventListener("scroll", kick, { passive: true });

  function reveals() {
    document.querySelectorAll(".reveal").forEach(h => {
      let i = 0;
      h.querySelectorAll(".line").forEach(line => {
        const nodes = [...line.childNodes];
        line.textContent = "";
        nodes.forEach(n => {
          const make = () => {
            const w = document.createElement("span");
            w.className = "w";
            w.style.transitionDelay = i++ * 60 + "ms";
            line.append(w);
            return w;
          };
          if (n.nodeType === 3) {
            n.textContent.split(/(\s+)/).forEach(part => {
              if (!part) return;
              if (/^\s+$/.test(part)) line.append(" ");
              else make().textContent = part;
            });
          } else make().append(n);
        });
      });
    });
    const io = new IntersectionObserver(es => es.forEach(e => {
      if (!e.isIntersecting) return;
      e.target.classList.add("in");
      io.unobserve(e.target);
    }), { threshold: 0.2 });
    document.querySelectorAll(".reveal, .rise").forEach(el => {
      if (el.classList.contains("rise")) {
        const sibs = [...el.parentElement.children].filter(x => x.classList.contains("rise"));
        el.style.transitionDelay = 250 + sibs.indexOf(el) * 120 + "ms";
      }
      io.observe(el);
    });
  }

  function ask() {
    const el = document.getElementById("ask");
    const panel = el.querySelector(".chat-panel");
    const askEl = el.querySelector(".ask");
    const answer = el.querySelector(".answer");
    const typed = el.querySelector(".typed");
    const send = el.querySelector(".send");
    const sources = [...el.querySelectorAll(".source")];
    const ai = document.getElementById("ai");
    const q = askEl.textContent;
    const words = answer.textContent.split(" ");
    answer.textContent = "";
    const spans = words.map(w => {
      const s = document.createElement("span");
      s.className = "w";
      s.textContent = w;
      answer.append(s, " ");
      return s;
    });
    const names = ["ChatGPT", "Claude", "Gemini"];
    let n = 0, visible = false;
    new IntersectionObserver(([e]) => { visible = e.isIntersecting; }).observe(el);
    if (!reduced) setInterval(() => {
      if (!visible) return;
      ai.classList.add("out");
      setTimeout(() => { ai.textContent = names[n = (n + 1) % names.length]; ai.classList.remove("out"); }, 350);
    }, 2800);

    scene(el, {
      target: () => progress(panel, 0.5, 0.5),
      draw() {
        const p = this.p;
        const kt = range(p, 0.04, 0.32);
        const sent = p > 0.36;
        typed.textContent = sent ? "" : q.slice(0, Math.round(q.length * kt));
        send.classList.toggle("go", p > 0.32 && p < 0.4);
        const ka = ease(range(p, 0.36, 0.46));
        askEl.style.opacity = ka;
        askEl.style.transform = `translateY(${((1 - ka) * 40).toFixed(1)}px) scale(${lerp(0.94, 1, ka).toFixed(3)})`;
        const shown = Math.round(spans.length * range(p, 0.46, 0.78));
        spans.forEach((s, i) => s.classList.toggle("in", i < shown));
        sources.forEach((s, i) => {
          const k = ease(range(p, 0.78 + i * 0.07, 0.9 + i * 0.07));
          s.style.opacity = k;
          s.style.transform = `translateX(${((1 - k) * 60).toFixed(1)}px)`;
        });
      }
    });
  }

  function ambient(el, frame) {
    let on = false, id = 0;
    const tick = t => { frame(t); if (on) id = requestAnimationFrame(tick); };
    new IntersectionObserver(([e]) => {
      on = e.isIntersecting && !reduced;
      cancelAnimationFrame(id);
      if (on) id = requestAnimationFrame(tick);
    }).observe(el);
  }

  function pix(ctx, x, y, s) {
    ctx.beginPath();
    ctx.roundRect(x - s / 2, y - s / 2, s, s, s * 0.32);
    ctx.fill();
  }

  function tidy() {
    const el = document.getElementById("tidy");
    const inTrack = el.querySelector(".lane-in .track");
    const outTrack = el.querySelector(".lane-out .track");
    const layer = el.querySelector(".layer canvas");
    const ICON = {
      mail: ["........", "########", "##....##", "#.#..#.#", "#..##..#", "#......#", "########", "........"],
      receipt: [".######.", ".#....#.", ".#.##.#.", ".#....#.", ".#.###..", ".#....#.", ".#....#.", ".#.##.#."],
      doc: [".####...", ".#..##..", ".#...##.", ".#.##.#.", ".#....#.", ".#.###..", ".#....#.", ".######."],
      chat: ["........", "########", "#......#", "#.####.#", "#......#", "########", ".##.....", ".#......"],
      cal: ["..#..#..", "########", "#......#", "########", "#.#.#..#", "#......#", "#.#.#..#", "########"],
      sheet: ["########", "#..#...#", "########", "#..#...#", "########", "#..#...#", "########", "........"],
      photo: ["########", "#......#", "#.##...#", "#.##...#", "#....#.#", "#...####", "#..#####", "########"],
      prompt: ["........", "#.......", ".#......", "..#.....", ".#......", "#..####.", "........", "........"],
      star: ["...#....", "#..#..#.", ".#.#.#..", "..###...", "#######.", "..###...", ".#.#.#..", "#..#..#."],
      spark: ["...#....", "...#....", "..###...", "#######.", "..###...", "...#....", "...#....", "........"],
      gem: ["...#....", "...#....", "..#.#...", "##...##.", "..#.#...", "...#....", "...#....", "........"],
      cursor: ["#.......", "##......", "#.#.....", "#..#....", "#...#...", "#.###...", "##......", "#......."],
      team: [".##..##.", ".##..##.", "........", "###.###.", "###.###.", "###.###.", "........", "........"]
    };
    const icon = name => {
      const cv = document.createElement("canvas");
      cv.className = "ico";
      const d = Math.min(2, devicePixelRatio || 1);
      cv.width = 40 * d; cv.height = 40 * d;
      const ctx = cv.getContext("2d");
      ctx.setTransform(d, 0, 0, d, 0, 0);
      ctx.fillStyle = INK;
      ICON[name].forEach((r, y) => [...r].forEach((ch, x) => { if (ch === "#") pix(ctx, x * 5 + 2.5, y * 5 + 2.5, 4); }));
      return cv;
    };
    const MESSY = [["mail", "fwd: invoice pls pay"], ["photo", "IMG_2041.jpg"], ["receipt", "taxi 38,50"], ["sheet", "Q3 final final.xlsx"], ["chat", "call notes acme"], ["cal", "mtg moved to thu??"], ["doc", "contract_v3 (2).pdf"], ["mail", "re: re: quote"]];
    const AGENTS = [["prompt", "Claude Code"], ["prompt", "Codex"], ["spark", "ChatGPT"], ["star", "Claude"], ["gem", "Gemini"], ["cursor", "Cursor"], ["team", "Your team"]];
    const rnd = seeded(83);
    const fillIn = () => {
      const make = ([ic, label]) => {
        const c = document.createElement("div");
        c.className = "mcard";
        c.style.transform = `rotate(${((rnd() - 0.5) * 7).toFixed(1)}deg) translateY(${Math.round((rnd() - 0.5) * 14)}px)`;
        const t = document.createElement("span");
        t.textContent = label;
        c.append(icon(ic), t);
        return c;
      };
      [0, 1].forEach(() => MESSY.forEach(m => inTrack.appendChild(make(m))));
    };
    const fillOut = () => {
      const make = ([ic, name]) => {
        const c = document.createElement("div");
        c.className = "acard";
        const b = document.createElement("b");
        b.textContent = name;
        c.append(icon(ic), b);
        return c;
      };
      [0, 1].forEach(() => AGENTS.forEach(a => outTrack.appendChild(make(a))));
    };
    fillIn();
    fillOut();
    new IntersectionObserver(([e]) => el.classList.toggle("off", !e.isIntersecting)).observe(el);
    const SHAPES = [[[0, 0], [1, 0], [2, 0], [3, 0]], [[0, 0], [1, 0], [0, 1], [1, 1]], [[0, 0], [1, 0], [2, 0], [1, 1]], [[1, 0], [2, 0], [0, 1], [1, 1]], [[0, 0], [1, 0], [1, 1], [2, 1]], [[0, 0], [0, 1], [1, 1], [2, 1]], [[2, 0], [0, 1], [1, 1], [2, 1]]];
    const norm = c => { const mx = Math.min(...c.map(p => p[0])), my = Math.min(...c.map(p => p[1])); return c.map(([x, y]) => [x - mx, y - my]); };
    const ROTS = SHAPES.map(sh => { const r = [norm(sh)]; for (let i = 0; i < 3; i++) r.push(norm(r[i].map(([x, y]) => [y, -x]))); return r; });
    const W = 10;
    let S = null, cell = 12, cols = 0, rows = 0, last = 0, wells = [];
    const fits = (g, cells, x, y) => cells.every(([cx, cy]) => { const X = x + cx, Y = y + cy; return X >= 0 && X < W && Y < rows && (Y < 0 || !g[Y][X]); });
    const score = (g, cells, x) => {
      let y = -4;
      while (fits(g, cells, x, y + 1)) y++;
      if (!fits(g, cells, x, y)) return null;
      const t = g.map(r => r.slice());
      cells.forEach(([cx, cy]) => { if (y + cy >= 0) t[y + cy][x + cx] = 1; });
      let lines = 0, holes = 0, agg = 0, bump = 0, prev = -1;
      t.forEach(r => { if (r.every(Boolean)) lines++; });
      for (let c = 0; c < W; c++) {
        let top = rows;
        for (let r = 0; r < rows; r++) if (t[r][c]) { top = r; break; }
        const hgt = rows - top;
        for (let r = top + 1; r < rows; r++) if (!t[r][c]) holes++;
        agg += hgt;
        if (prev >= 0) bump += Math.abs(hgt - prev);
        prev = hgt;
      }
      return -0.51 * agg + 0.76 * lines - 0.36 * holes - 0.18 * bump + Math.random() * 0.3;
    };
    const spawn = w => {
      const k = Math.floor(Math.random() * ROTS.length);
      let best = null;
      ROTS[k].forEach(cells => { for (let x = 0; x < W; x++) { const v = score(w.g, cells, x); if (v !== null && (!best || v > best.v)) best = { v, cells, x }; } });
      if (!best || !fits(w.g, best.cells, best.x, -2)) { w.g = w.g.map(r => r.map(() => 0)); w.p = null; w.wait = 6; return; }
      w.p = { cells: best.cells, x: best.x, y: -2 };
    };
    const newWell = () => ({ g: Array.from({ length: rows }, () => Array(W).fill(0)), p: null, wait: Math.floor(Math.random() * 30) });
    const layout = () => {
      S = fitCanvas(layer);
      cols = Math.floor(S.w / cell);
      rows = Math.floor(S.h / cell);
      wells = Array.from({ length: Math.ceil(cols / W) }, newWell);
      if (reduced) wells.forEach(w => { for (let i = 0; i < 40; i++) { spawn(w); if (w.p) { while (fits(w.g, w.p.cells, w.p.x, w.p.y + 1)) w.p.y++; lock(w); } } });
    };
    const lock = w => {
      w.p.cells.forEach(([cx, cy]) => { const Y = w.p.y + cy; if (Y >= 0) w.g[Y][w.p.x + cx] = 1; });
      w.p = null;
      w.g = w.g.filter(r => !r.every(Boolean));
      while (w.g.length < rows) w.g.unshift(Array(W).fill(0));
    };
    const step = () => wells.forEach(w => {
      if (w.wait > 0) { w.wait--; return; }
      if (!w.p) { spawn(w); return; }
      if (fits(w.g, w.p.cells, w.p.x, w.p.y + 1)) w.p.y++;
      else lock(w);
    });
    const draw = () => {
      const { ctx, w, h } = S;
      ctx.clearRect(0, 0, w, h);
      const ox = (w - cols * cell) / 2, oy = (h - rows * cell) / 2;
      for (let y = 0; y < rows; y++) for (let x = 0; x < cols; x++) {
        const well = wells[Math.floor(x / W)], lx = x % W;
        const falling = well.p && well.p.cells.some(([cx, cy]) => well.p.x + cx === lx && well.p.y + cy === y);
        ctx.fillStyle = falling ? "#141413" : well.g[y][lx] ? "#bdbdbb" : "#f0f0f0";
        pix(ctx, ox + x * cell + cell / 2, oy + y * cell + cell / 2, cell - 3);
      }
    };
    layout();
    draw();
    new ResizeObserver(() => { layout(); draw(); }).observe(layer);
    ambient(el, t => {
      if (t - last < 90) return;
      last = t;
      step();
      draw();
    });
  }

  function own() {
    const el = document.getElementById("yours");
    const box = el.querySelector(".own-art");
    const c = box.querySelector("canvas");
    const modes = [...el.querySelectorAll(".mode")];
    const HILL = "#d6d6d5", ROAD = "#a9a9a8", SOFT = "#9c9c9b", MID = "#7d7d7c", CLOUD = "#e2e2e1", WHITE = "#ffffff";
    let S = null, cell = 12, C = 0, R = 30, ground = [], A = null, B = null, trees = [], clouds = [], lights = [], tick = 0, last = 0;
    let goal = 0, forced = -1, until = 0, shown = -1, courier = { x: 0, dir: 1 }, smoke = [];
    function layout() {
      const w = box.clientWidth;
      const m = w < 640;
      cell = m ? 10 : Math.max(10, Math.min(14, Math.floor(w / 110)));
      R = m ? 30 : 30;
      const hh = R * cell + "px";
      if (box.style.height !== hh) box.style.height = hh;
      S = fitCanvas(c);
      C = Math.floor(S.w / cell);
      const base = R - 6;
      const ax = Math.round(C * (m ? 0.2 : 0.24)), bx = Math.round(C * (m ? 0.76 : 0.72));
      const aw = m ? 7 : 9, bw = m ? 9 : 11;
      const flat = x => Math.min(clamp((Math.abs(x - ax) - aw) / 6), clamp((Math.abs(x - bx) - bw) / 6));
      ground = [];
      for (let x = 0; x < C; x++) ground.push(Math.round(base + (1.6 * Math.sin(x / 9) + 1.1 * Math.sin(x / 4.3 + 1)) * flat(x)));
      A = { x0: ax - Math.floor(aw / 2), w: aw, h: m ? 10 : 13 };
      A.base = ground[ax];
      B = { x0: bx - Math.floor(bw / 2), w: bw, body: m ? 5 : 7, roof: Math.ceil(bw / 2) };
      B.base = ground[bx];
      const rnd = seeded(71);
      lights = [];
      for (let y = 2; y < A.h - 1; y += 2) for (let x = 1; x < A.w - 1; x += 2) lights.push({ k: "a", x: A.x0 + x, y: A.base - A.h + y, on: rnd() < 0.5 });
      for (let x = 2; x < B.w - 2; x += 3) lights.push({ k: "b", x: B.x0 + x, y: B.base - B.body + 2, on: rnd() < 0.6 });
      trees = [];
      const spots = m ? [0.05, 0.42, 0.55, 0.95] : [0.05, 0.12, 0.4, 0.47, 0.58, 0.88, 0.96];
      spots.forEach(f => { const x = Math.round(C * f); if (x > 1 && x < C - 2) trees.push({ x, big: rnd() < 0.5 }); });
      clouds = [{ x: Math.round(C * 0.1), y: 3, w: 7 }, { x: Math.round(C * 0.55), y: 6, w: 9 }, { x: Math.round(C * 0.85), y: 2, w: 6 }];
      courier = { x: A.x0 + A.w + 1, dir: 1 };
      smoke = [];
    }
    const at = (x, y, col) => { S.ctx.fillStyle = col; pix(S.ctx, x * cell + cell / 2, y * cell + cell / 2, cell - 2); };
    function draw() {
      const { ctx, w, h } = S;
      ctx.clearRect(0, 0, w, h);
      const aOn = goal === 0, bOn = goal === 1;
      clouds.forEach(cl => {
        for (let i = 0; i < cl.w; i++) {
          const x = (cl.x + i) % C;
          at(x, cl.y, CLOUD);
          if (i > 0 && i < cl.w - 1) at(x, cl.y - 1, CLOUD);
          if (i > 1 && i < cl.w - 3) at(x, cl.y - 2, CLOUD);
        }
      });
      const hc = A.x0 + Math.floor(A.w / 2), top = A.base - A.h;
      [[-3, -3], [-2, -3], [-1, -3], [0, -3], [1, -3], [2, -3], [3, -3], [-2, -4], [-1, -4], [0, -4], [1, -4], [-1, -5], [0, -5]].forEach(([dx, dy]) => at(hc + dx, top + dy, aOn ? MID : CLOUD));
      for (let x = 0; x < C; x++) for (let y = ground[x]; y < R; y++) at(x, y, HILL);
      for (let x = A.x0 + A.w; x < B.x0; x++) at(x, ground[x], ROAD);
      trees.forEach(t => {
        const g = ground[t.x];
        at(t.x, g - 1, MID);
        const r = t.big ? 2 : 1;
        for (let dy = -r; dy <= r; dy++) for (let dx = -r; dx <= r; dx++) if (Math.abs(dx) + Math.abs(dy) <= r + (t.big ? 1 : 0)) at(t.x + dx, g - 2 - r + dy, SOFT);
      });
      for (let y = 0; y < A.h; y++) for (let x = 0; x < A.w; x++) at(A.x0 + x, A.base - A.h + y, aOn ? INK : SOFT);
      for (let y = 0; y < B.body; y++) for (let x = 0; x < B.w; x++) at(B.x0 + x, B.base - B.body + y, bOn ? INK : SOFT);
      for (let r = 0; r < B.roof; r++) for (let x = r; x < B.w - r; x++) at(B.x0 + x, B.base - B.body - 1 - r, bOn ? INK : SOFT);
      const chx = B.x0 + B.w - 3, chy = B.base - B.body - 2;
      at(chx, chy, bOn ? INK : SOFT);
      at(chx, chy - 1, bOn ? INK : SOFT);
      const door = B.x0 + Math.floor(B.w / 2);
      at(door, B.base - 1, WHITE);
      at(door, B.base - 2, WHITE);
      lights.forEach(l => { if (l.on) at(l.x, l.y, WHITE); else if ((l.k === "a" && aOn) || (l.k === "b" && bOn)) at(l.x, l.y, MID); });
      smoke.forEach(s => at(chx + s.dx, s.y, CLOUD));
      at(courier.x, ground[courier.x] - 1, INK);
      at(courier.x, ground[courier.x] - 2, INK);
    }
    function stepWorld() {
      tick++;
      if (tick % 6 === 0) clouds.forEach(cl => { cl.x = (cl.x + 1) % C; });
      if (tick % 9 === 0) { const l = lights[Math.floor(Math.random() * lights.length)]; l.on = !l.on; }
      if (tick % 3 === 0) {
        smoke.forEach(s => { s.y -= 1; if (Math.random() < 0.3) s.dx += 1; });
        smoke = smoke.filter(s => s.y > B.base - B.body - B.roof - 8);
        if (tick % 6 === 0) smoke.push({ y: B.base - B.body - 4, dx: 0 });
      }
      if (tick % 2 === 0) {
        courier.x += courier.dir;
        if (courier.x >= B.x0 - 1 || courier.x <= A.x0 + A.w) courier.dir *= -1;
      }
    }
    modes.forEach((m, i) => {
      const pick = () => { forced = i; until = performance.now() + 6000; render(performance.now()); };
      m.addEventListener("pointerenter", pick);
      m.addEventListener("click", pick);
    });
    function render(t) {
      goal = ((t / 1000) % 9) < 4.5 ? 0 : 1;
      if (forced >= 0 && t < until) goal = forced;
      if (goal !== shown) { shown = goal; modes.forEach((m, i) => m.classList.toggle("on", i === goal)); }
      draw();
    }
    layout();
    render(performance.now());
    new ResizeObserver(() => { layout(); render(performance.now()); }).observe(box);
    ambient(el, t => {
      if (t - last < 140) return;
      last = t;
      stepWorld();
      render(t);
    });
  }

  const GRAY = "#b9b9b8", AIC = "#7d7d7c";
  const sprite = rows => { const out = []; rows.forEach((r, y) => [...r].forEach((ch, x) => { if (ch === "#") out.push([x, y]); })); return out; };
  const PERSON = sprite([".#.", "###", "#.#"]);
  const SPARK = sprite([".#.", "###", ".#."]);

  function hero() {
    const el = document.getElementById("hero");
    const c = el.querySelector(".hero-art canvas");
    const parts = [...el.querySelectorAll(".part")];
    const icons = [
      sprite([".#######.", "#########", "#.......#", ".#######.", "#.......#", "#########", "#.......#", ".#######."]),
      sprite(["####.......", "###########", "#.........#", "#.........#", "#.........#", "#.........#", "#.........#", "###########"]),
      sprite(["....#....", "...###...", "..#####..", "#########", "..#####..", "...###...", "....#...."])
    ];
    let S = null, cell = 14, gc = 0, gr = 0, ox = 0, oy = 0, base = [], marks = [], wires = [], t0 = 0;
    function layout() {
      const wd = c.parentElement.clientWidth;
      cell = Math.max(6, Math.min(14, Math.floor(wd / 46)));
      const hh = cell * 13 + "px";
      if (c.parentElement.style.height !== hh) c.parentElement.style.height = hh;
      S = fitCanvas(c);
      gc = Math.floor(S.w / cell);
      gr = Math.floor(S.h / cell);
      ox = (S.w - gc * cell) / 2;
      oy = (S.h - gr * cell) / 2;
      const rnd = seeded(31);
      const left = Math.max(1, Math.round(gc * 0.08)), right = gc - 1 - left;
      base = [];
      for (let y = gr - 3; y < gr; y++) {
        const inset = gr - 1 - y;
        for (let x = left + (2 - inset); x <= right - (2 - inset); x++) base.push({ x, y, d: (x - left) / (right - left) * 0.7 + rnd() * 0.3 });
      }
      const top = gr - 3;
      const centers = [1 / 6, 0.5, 5 / 6].map(f => Math.round(gc * f));
      marks = [];
      const boxes = icons.map((ic, i) => {
        const w = Math.max(...ic.map(p => p[0])) + 1, h = Math.max(...ic.map(p => p[1])) + 1;
        const x0 = centers[i] - Math.floor(w / 2), y0 = top - 1 - h;
        ic.forEach(([x, y]) => marks.push({ x: x0 + x, y: y0 + y, d: i * 0.33 + rnd() * 0.33 }));
        return { x0, y0, w, h };
      });
      wires = [];
      for (let i = 0; i < boxes.length - 1; i++) {
        const a = boxes[i], b = boxes[i + 1];
        const y = Math.round(top - 1 - Math.min(a.h, b.h) / 2);
        const cells = [];
        for (let x = a.x0 + a.w + 1; x < b.x0 - 1; x++) cells.push([x, y]);
        wires.push(cells);
      }
    }
    function frame(t) {
      if (!t0) t0 = t;
      const { ctx, w, h } = S;
      const u = reduced ? 99 : (t - t0) / 1000;
      ctx.clearRect(0, 0, w, h);
      const at = (x, y, col) => { ctx.fillStyle = col; pix(ctx, ox + x * cell + cell / 2, oy + y * cell + cell / 2, cell - 3); };
      parts.forEach((p, i) => p.classList.toggle("in", u > 2.1 + i * 0.5));
      base.forEach(b => { if (u > 0.2 + b.d * 1.4) at(b.x, b.y, GRAY); });
      marks.forEach(m => { if (u > 1.6 + m.d * 1.5) at(m.x, m.y, INK); });
      if (u > 3.3) {
        const step = Math.floor((u - 3.3) / 0.09);
        wires.forEach((cells, i) => cells.forEach(([x, y], j) => {
          if (j > step) return;
          const head = reduced ? -1 : (step + i * 7) % (cells.length + 6);
          at(x, y, j === head ? INK : GRAY);
        }));
      }
    }
    layout();
    new ResizeObserver(() => { layout(); frame(performance.now()); }).observe(c);
    frame(performance.now());
    ambient(el, frame);
  }

  function flow() {
    const el = document.getElementById("flow");
    const rows = [...el.querySelectorAll(".t-row")];
    const c = el.querySelector(".water");
    const list = el.querySelector(".t-list");
    const WATER = "#e4e4e3", SURFACE = "#d2d2d1";
    let timers = [], visible = false, running = false, S = null, cell = 12, cols = 0, gr = 0, level = 0, tick = 0, last = 0, bubbles = [];
    const later = (ms, f) => timers.push(setTimeout(f, ms));
    const set = (r, on) => { r.classList.toggle("on", on); r.querySelector(".t-pill").textContent = on ? "Connected" : "Connect"; };
    const target = () => {
      const n = rows.filter(r => r.classList.contains("on")).length;
      const listTop = Math.floor((list.getBoundingClientRect().top - c.getBoundingClientRect().top) / cell);
      const low = 2, high = gr - listTop + 1;
      return Math.round(low + (high - low) * n / rows.length);
    };
    function layout() {
      S = fitCanvas(c);
      cell = S.w < 640 ? 10 : 12;
      cols = Math.ceil(S.w / cell);
      gr = Math.ceil(S.h / cell);
      level = reduced ? target() : Math.min(level || 3, gr);
    }
    function draw() {
      const { ctx, w, h } = S;
      ctx.clearRect(0, 0, w, h);
      for (let x = 0; x < cols; x++) {
        const wave = reduced ? 0 : Math.round(Math.sin(x * 0.35 - tick * 0.45) * 0.9);
        const top = gr - Math.max(1, level + wave);
        for (let y = Math.max(0, top); y < gr; y++) {
          ctx.fillStyle = y === top ? SURFACE : WATER;
          pix(ctx, x * cell + cell / 2, h - (gr - y) * cell + cell / 2, cell - 2);
        }
      }
      ctx.fillStyle = "#ffffff";
      bubbles.forEach(b => pix(ctx, b.x * cell + cell / 2, h - (gr - b.y) * cell + cell / 2, cell - 2));
    }
    function step() {
      tick++;
      const t = target();
      if (level !== t) level += Math.sign(t - level);
      bubbles.forEach(b => { b.y -= 1; });
      bubbles = bubbles.filter(b => b.y > gr - level + 1);
      if (bubbles.length < 2 && level > 6 && Math.random() < 0.08) bubbles.push({ x: Math.floor(Math.random() * cols), y: gr - 1 });
      draw();
    }
    layout();
    if (reduced) { rows.forEach(r => set(r, true)); layout(); draw(); new ResizeObserver(() => { layout(); draw(); }).observe(c); return; }
    draw();
    new ResizeObserver(() => { layout(); draw(); }).observe(c);
    function play() {
      timers.forEach(clearTimeout);
      timers = [];
      running = true;
      rows.forEach(r => set(r, false));
      rows.forEach((r, i) => later(900 + i * 1200, () => set(r, true)));
      later(900 + rows.length * 1200 + 5000, () => { running = false; if (visible) play(); });
    }
    new IntersectionObserver(([e]) => {
      visible = e.isIntersecting;
      if (visible && !running) play();
    }, { threshold: 0.3 }).observe(el);
    ambient(el, t => {
      if (t - last < 180) return;
      last = t;
      step();
    });
  }

  function edit() {
    const el = document.getElementById("edit");
    const tabs = [...el.querySelectorAll(".tab")];
    const ways = [...el.querySelectorAll(".way")];
    const say = el.querySelector(".say");
    const add = el.querySelector(".add");
    const tbl = el.querySelector(".tbl");
    const tables = el.querySelector(".tables");
    const who = el.querySelector(".th-who");
    const paidHead = el.querySelector(".th-paid");
    const paidCells = [...el.querySelectorAll(".tbl > .pc:not(.th)")];
    let mode = 1, hold = 0, visible = false, started = false, timers = [];
    const later = (ms, f) => timers.push(setTimeout(f, ms));
    const flash = n => { n.classList.add("flash"); later(900, () => n.classList.remove("flash")); };
    const fill = from => paidCells.forEach((c, k) => later(from + k * 300, () => c.classList.remove("empty")));
    function reset() {
      tbl.classList.remove("paid");
      tables.classList.remove("two");
      who.textContent = "Customer";
      paidHead.textContent = "Paid on";
      paidHead.classList.remove("editing", "flash");
      who.classList.remove("flash");
      paidCells.forEach(c => c.classList.add("empty"));
      say.textContent = "";
      add.classList.remove("press");
    }
    function finish() {
      tbl.classList.add("paid");
      paidCells.forEach(c => c.classList.remove("empty"));
    }
    function run(i) {
      timers.forEach(clearTimeout);
      timers = [];
      mode = i;
      tabs.forEach((t, j) => t.classList.toggle("on", j === i));
      ways.forEach((w, j) => w.classList.toggle("on", j === i));
      reset();
      if (reduced) {
        finish();
        if (i === 1) { say.textContent = "Rename Customer to Client"; tables.classList.add("two"); who.textContent = "Client"; }
        return;
      }
      if (i === 1) {
        later(400, () => { say.textContent = "Also track when each invoice was paid"; });
        later(1400, () => { tbl.classList.add("paid"); flash(paidHead); });
        fill(1800);
        later(3400, () => { say.textContent = "Make a separate list for suppliers"; });
        later(4300, () => tables.classList.add("two"));
        later(5700, () => { say.textContent = "Rename Customer to Client"; });
        later(6500, () => { who.textContent = "Client"; flash(who); });
      } else if (i === 0) {
        later(600, () => add.classList.add("press"));
        later(900, () => { add.classList.remove("press"); paidHead.textContent = ""; paidHead.classList.add("editing"); tbl.classList.add("paid"); });
        "Paid on".split("").forEach((ch, k) => later(1200 + k * 110, () => { paidHead.textContent = "Paid on".slice(0, k + 1); }));
        later(2200, () => { paidHead.classList.remove("editing"); flash(paidHead); });
        fill(2500);
      } else {
        later(1200, () => { tbl.classList.add("paid"); flash(paidHead); });
        fill(1500);
      }
      if (started) queue();
    }
    const order = [1, 0, 2];
    const queue = () => later(mode === 1 ? 9500 : 6000, () => {
      if (!visible) { started = false; return; }
      if (Date.now() < hold) { queue(); return; }
      run(order[(order.indexOf(mode) + 1) % 3]);
    });
    tabs.forEach((t, i) => t.addEventListener("click", () => { hold = Date.now() + 14000; run(i); }));
    run(1);
    if (reduced) return;
    new IntersectionObserver(([e]) => {
      visible = e.isIntersecting;
      if (visible && !started) { started = true; run(mode); }
    }, { threshold: 0.3 }).observe(el);
  }

  function spaces() {
    const el = document.getElementById("spaces");
    const c = el.querySelector(".rooms-art canvas");
    const members = [[0, 2], [1], [0, 1, 2]];
    const labels = [...el.querySelectorAll(".room-name")];
    let S = null, cell = 16, gc = 0, gr = 0, ox = 0, oy = 0, rooms = [], cy = 0, inY = 0, walker = 0, path = [], t0 = 0, RW = 14, RH = 10;
    function build() {
      const pts = [];
      let x = 0, y = cy;
      const go = (tx, ty) => {
        while (x !== tx) { x += Math.sign(tx - x); pts.push([x, y]); }
        while (y !== ty) { y += Math.sign(ty - y); pts.push([x, y]); }
      };
      pts.push([x, y]);
      rooms.forEach((r, i) => {
        go(r.door, cy);
        if (members[walker].includes(i)) {
          go(r.door, inY);
          for (let k = 0; k < 14; k++) pts.push([x, y]);
          go(r.door, cy);
        } else for (let k = 0; k < 5; k++) pts.push([x, y]);
      });
      go(gc - 3, cy);
      path = pts;
    }
    function layout() {
      const wd = c.parentElement.clientWidth;
      const m = wd < 640;
      RW = m ? 10 : 14; RH = m ? 9 : 10;
      const gap = m ? 2 : 6;
      cell = Math.max(6, Math.min(16, Math.floor(wd / (RW * 3 + gap * 2 + 6))));
      const hh = (RH + 7) * cell + "px";
      if (c.parentElement.style.height !== hh) c.parentElement.style.height = hh;
      S = fitCanvas(c);
      gc = Math.floor(S.w / cell);
      gr = Math.floor(S.h / cell);
      ox = (S.w - gc * cell) / 2;
      oy = (S.h - gr * cell) / 2;
      const total = RW * 3 + gap * 2, x0 = Math.floor((gc - total) / 2);
      const top = 1;
      cy = top + RH + 1;
      inY = top + RH - 5;
      const rnd = seeded(41);
      rooms = [0, 1, 2].map(i => {
        const x = x0 + i * (RW + gap);
        return { x, top, door: x + Math.floor((RW - 3) / 2), bars: [RW - 4 - Math.floor(rnd() * 2), RW - 6 - Math.floor(rnd() * 3)] };
      });
      labels.forEach((l, i) => {
        const r = rooms[i];
        l.style.left = (ox + (r.x + RW / 2) * cell) + "px";
      });
      build();
    }
    function frame(t) {
      if (!t0) t0 = t;
      const { ctx, w, h } = S;
      let i = Math.floor((t - t0) / 85);
      if (reduced) i = path.findIndex(p => p[1] === inY) + 6;
      if (i > path.length + 8) { walker = (walker + 1) % members.length; build(); t0 = t; i = 0; }
      const me = path[Math.min(i, path.length - 1)];
      let j = Math.min(i, path.length - 1) - 4, ai = null;
      for (; j >= 0; j--) { const p = path[j]; if (Math.max(Math.abs(p[0] - me[0]), Math.abs(p[1] - me[1])) >= 4) { ai = p; break; } }
      ctx.clearRect(0, 0, w, h);
      const at = (x, y, col) => { ctx.fillStyle = col; pix(ctx, ox + x * cell + cell / 2, oy + y * cell + cell / 2, cell - 3); };
      ctx.fillStyle = T3;
      for (let y = 0; y < gr; y++) for (let x = 0; x < gc; x++) {
        ctx.beginPath();
        ctx.arc(ox + x * cell + cell / 2, oy + y * cell + cell / 2, 1.3, 0, Math.PI * 2);
        ctx.fill();
      }
      rooms.forEach((r, k) => {
        const open = members[walker].includes(k);
        const inside = me[1] < cy && me[0] >= r.x && me[0] < r.x + RW;
        for (let x = 0; x < RW; x++) for (let y = 0; y < RH; y++) {
          const edge = x === 0 || y === 0 || x === RW - 1 || y === RH - 1;
          if (!edge) continue;
          const isDoor = y === RH - 1 && r.x + x >= r.door && r.x + x < r.door + 3;
          if (isDoor) { if (!open) at(r.x + x, r.top + y, INK); continue; }
          at(r.x + x, r.top + y, GRAY);
        }
        r.bars.forEach((len, b) => { for (let x = 0; x < len; x++) at(r.x + 2 + x, r.top + 2 + b * 2, inside ? INK : GRAY); });
        PERSON.forEach(([px, py]) => at(r.x + RW - 5 + px, inY + py, GRAY));
      });
      if (ai) SPARK.forEach(([px, py]) => at(ai[0] + px, ai[1] + py, AIC));
      PERSON.forEach(([px, py]) => at(me[0] + px, me[1] + py, INK));
    }
    layout();
    new ResizeObserver(() => { layout(); frame(performance.now()); }).observe(c);
    frame(performance.now());
    ambient(el, frame);
  }

  function skyline() {
    const el = document.getElementById("cta");
    const c = el.querySelector("canvas");
    let S = null, cell = 24, cols = 0, rows = 0, seen = false;
    new IntersectionObserver(([e]) => { if (e.isIntersecting) seen = true; }, { threshold: 0.35 }).observe(el);
    const hash = (a, b, k) => { const v = Math.sin(a * 127.1 + b * 311.7 + k * 74.7) * 43758.5453; return v - Math.floor(v); };
    function layout() {
      S = fitCanvas(c);
      cell = S.w < 640 ? 16 : 22;
      cols = Math.ceil(S.w / cell);
      rows = Math.ceil(S.h / cell);
    }
    scene(el, {
      snap: 0.025,
      layout,
      target: () => (seen ? 1 : 0),
      draw(t) {
        if (!S) layout();
        const { ctx, w, h } = S, p = this.p;
        ctx.clearRect(0, 0, w, h);
        ctx.fillStyle = INK;
        const step = Math.floor(t / 450);
        const maxH = rows * 0.26;
        const g = 2.5;
        for (let x = 0; x < cols; x++) {
          const n = Math.sin(x * 0.21 + t / 2600) * 0.5 + Math.sin(x * 0.07 - t / 4100) * 0.8 + Math.sin(x * 0.53 + 1.3) * 0.35;
          const height = Math.max(0, p * (maxH * 0.6 + n * maxH * 0.3));
          const edge = rows - height;
          for (let y = Math.max(0, Math.floor(edge) - 4); y < rows; y++) {
            let on = y >= edge;
            if (!on && y >= edge - 4) on = hash(x, y, step) > 0.9 - (y - (edge - 4)) * 0.06;
            if (on && y < edge + 3 && hash(x, y, step + 5) > 0.92) on = false;
            if (!on) continue;
            ctx.beginPath();
            ctx.roundRect(x * cell + g / 2, h - (rows - y) * cell + g / 2, cell - g, cell - g, (cell - g) * 0.32);
            ctx.fill();
          }
        }
      }
    });
  }

  const PROMPT = `Help me set up Archive on this machine. Archive is an open-source home for a company's knowledge: it stores records and files and lets AI assistants read them over MCP. Code: https://github.com/Mulholland-Inc/archive

Please:
1. Check that Docker is installed and running (docker info). If it is not, help me install Docker Desktop or Docker Engine for my operating system first.
2. Ask me for the email address of the first owner, then run:
   ARCHIVE_ADMIN_EMAIL=<my email> sh -c "$(curl -fsSL https://raw.githubusercontent.com/Mulholland-Inc/archive/main/install.sh)"
   This pulls ghcr.io/mulholland-inc/archive:latest and runs it as a container named archive on port 8080, with all data in a Docker volume named archive. Running the same command again updates it, and the data stays.
3. Wait until it reports that it is running, then open http://localhost:8080. Without email set up, sign-in links are printed in the log: docker logs archive
4. Optional settings, passed as environment variables to the same command:
   - ARCHIVE_PORT (default 8080)
   - ARCHIVE_URL (the public address if it runs on a server, for example https://archive.example.com)
   - ARCHIVE_LOCALE (for example nl-NL)
   - ARCHIVE_SMTP_URL and ARCHIVE_SMTP_FROM (for sign-in emails)
   - ARCHIVE_OIDC (Google or Microsoft sign-in, as a JSON list of {"name","issuer","client_id","client_secret"})
   On a server, put it behind HTTPS with a reverse proxy such as Caddy, and set ARCHIVE_URL.
5. Connect an AI assistant: Archive is an MCP server at <ARCHIVE_URL>/mcp. Help me add it to Claude, ChatGPT or my coding assistant.
6. If anything fails, read docker logs archive and fix it with me.

Explain each step briefly before you run it, and ask before changing anything outside Docker.`;

  function copies() {
    const toast = document.querySelector(".toast");
    let timer = 0;
    const show = text => {
      toast.textContent = text;
      toast.classList.add("on");
      clearTimeout(timer);
      timer = setTimeout(() => toast.classList.remove("on"), 2500);
    };
    const fallback = text => {
      const t = document.createElement("textarea");
      t.value = text;
      t.setAttribute("readonly", "");
      t.style.position = "fixed";
      t.style.opacity = "0";
      document.body.appendChild(t);
      t.select();
      let ok = false;
      try { ok = document.execCommand("copy"); } catch (e) { ok = false; }
      t.remove();
      return ok;
    };
    const copy = async (b, text) => {
      let ok = false;
      try { await navigator.clipboard.writeText(text); ok = true; } catch (e) { ok = fallback(text); }
      b.classList.add("copied");
      setTimeout(() => b.classList.remove("copied"), 1600);
      return ok;
    };
    document.querySelectorAll(".copy-prompt").forEach(b => b.addEventListener("click", async () => {
      const ok = await copy(b, PROMPT);
      show(ok ? (innerWidth < 520 ? "Prompt copied" : "Prompt copied. Paste it into Claude Code or Codex.") : "Copying did not work");
    }));
    document.querySelectorAll(".copy-code").forEach(b => b.addEventListener("click", async () => {
      const ok = await copy(b, b.closest(".code").querySelector("pre").textContent);
      show(ok ? "Copied" : "Copying did not work");
    }));
  }

  reveals();
  copies();
  if (document.getElementById("hero")) {
    hero();
    flow();
    ask();
    tidy();
    own();
    edit();
    spaces();
    skyline();
  }
  if (document.fonts) document.fonts.ready.then(() => scenes.forEach(s => { s.layout && s.layout(); s.draw(performance.now(), true); }));
  kick();
})();
