/**
 * The family view: the tree walked one chevron at a time, the way Ancestry's family view
 * walks it (Sam, 2026-10-20: "look at how this works. we need the same").
 *
 * Generations stand in rows, oldest at the top. A couple sits side by side, joined by a short
 * line; above them sit their parents, the husband's to the left and the wife's to the right,
 * spread apart only as far as they must be. A person's brothers and sisters open out beside
 * them, on the outside of their couple, eldest first, and anyone off the line can open their
 * children below them. A line leaves a couple from the middle of the line between them, runs
 * down, along and down again with rounded corners, and meets each child from above.
 *
 * This file is pure and safe in the browser, so the page reflows at once when a chevron is
 * pressed: `familyLayout` says where every card, line, chevron and label goes, from the book
 * (`familybook.ts` makes it on the server, already private) and what is open (`FamilyState`).
 * How it looks is ui/FamilyTree.svelte's.
 *
 * Placement works on shapes: what a part of the drawing takes up in each row, as a left and a
 * right edge. A person's brothers and sisters are packed outward against their own card (and,
 * for the focus, against the focus's children); their parents stand over them, pushed outward
 * when anything of theirs comes down into the person's row; a couple's two sets of parents
 * move apart, half each way, until they clear each other. So nothing ever overlaps, and the
 * line straight up from the focus stays straight until two sides have to make room.
 */
/** The family someone was born into: the first of their parents' unions with a parent in it. */
export function birthUnion(book, h) {
  return book.people[h]?.parents.find((u) => book.unions[u] && (book.unions[u].a || book.unions[u].b)) ?? null;
}
/** The other partner in a union, or null. */
const other = (u, h) => (u.a === h ? u.b : u.a);
/** Someone's unions, one for each partner (a couple recorded twice is drawn once). */
export function partnersOf(book, h) {
  const seen = new Set();
  return (book.people[h]?.unions ?? []).filter((u) => {
    const o = other(book.unions[u], h) ?? `none:${u}`;
    if (seen.has(o))
      return false;
    seen.add(o);
    return true;
  });
}
/** The union drawn beside someone: the one chosen for them, else their first with a partner, else their first. */
export function shownUnion(book, s, h) {
  const mine = book.people[h]?.unions ?? [];
  const chosen = s.spouse[h];
  if (chosen && mine.includes(chosen))
    return chosen;
  return mine.find((u) => other(book.unions[u], h)) ?? mine[0] ?? null;
}
/** How many brothers and sisters, whole and half, someone has. */
export function siblingCount(book, h) {
  const u = birthUnion(book, h);
  if (!u)
    return 0;
  const all = new Set(book.unions[u].kids);
  for (const q of [book.unions[u].a, book.unions[u].b])
    if (q)
      for (const v of book.people[q].unions)
        for (const k of book.unions[v].kids)
          all.add(k);
  all.delete(h);
  return all.size;
}
/**
 * Where the view starts: the focus with `depth` generations of their forebears open above
 * them, and their children below.
 */
export function startState(book, focus, depth = 3) {
  const up = [];
  let gen = [focus];
  for (let i = 0; i < depth && gen.length; i++) {
    const next = [];
    for (const h of gen) {
      const u = birthUnion(book, h);
      if (!u || up.includes(h))
        continue;
      up.push(h);
      for (const q of [book.unions[u].a, book.unions[u].b])
        if (q)
          next.push(q);
    }
    gen = next;
  }
  const mine = shownUnion(book, { spouse: {} }, focus);
  return { focus, up, sibs: [], kids: mine && book.unions[mine].kids.length ? [mine] : [], spouse: {} };
}
const without = (list, x) => list.filter((y) => y !== x);
/**
 * What is open after a chevron or a switch. Brothers and sisters hang from their parents, so
 * opening them opens the parents too, and folding the parents folds them away. A fold keeps
 * what was open beyond it, so opening it again brings the same view back.
 */
export function toggle(book, s, act) {
  const h = act.id;
  if (act.kind === 'up')
    return s.up.includes(h) ? { ...s, up: without(s.up, h), sibs: without(s.sibs, h) } : { ...s, up: [...s.up, h] };
  if (act.kind === 'sibs')
    return s.sibs.includes(h) ? { ...s, sibs: without(s.sibs, h) } : { ...s, sibs: [...s.sibs, h], up: s.up.includes(h) ? s.up : [...s.up, h] };
  if (act.kind === 'kids')
    return s.kids.includes(h) ? { ...s, kids: without(s.kids, h) } : { ...s, kids: [...s.kids, h] };
  // the next husband or wife; their children are open when the last one's were
  const all = partnersOf(book, h);
  const now = shownUnion(book, s, h);
  const next = all[(all.indexOf(now ?? '') + 1) % all.length];
  if (!next || next === now)
    return s;
  const kids = now && s.kids.includes(now) && !s.kids.includes(next) ? [...s.kids, next] : s.kids;
  return { ...s, spouse: { ...s.spouse, [h]: next }, kids };
}
/** A state from the outside world (a page's history), kept only where it still makes sense. */
export function cleanState(book, raw) {
  if (!raw || typeof raw !== 'object')
    return null;
  const r = raw;
  const ids = (v, of) => (Array.isArray(v) ? [...new Set(v.filter((x) => typeof x === 'string' && x in of))] : []);
  if (typeof r.focus !== 'string' || !(r.focus in book.people))
    return null;
  const spouse = {};
  if (r.spouse && typeof r.spouse === 'object')
    for (const [k, v] of Object.entries(r.spouse))
      if (k in book.people && typeof v === 'string' && book.people[k].unions.includes(v))
        spouse[k] = v;
  return { focus: r.focus, up: ids(r.up, book.people), sibs: ids(r.sibs, book.people), kids: ids(r.kids, book.unions), spouse };
}
// -- sizes --------------------------------------------------------------------------------
/** Sizes in pixels at zoom 1. A card is a portrait with the name (two lines at most) and the years under it. */
export const FV = {
  /** a card */
  w: 100,
  h: 152,
  /** between a husband's card and his wife's: the line that joins them */
  pair: 36,
  /** between two cards or groups that are not a couple */
  gap: 40,
  /** room beside a card for its brothers-and-sisters chevron */
  side: 30,
  /** from one generation's top to the next */
  row: 300,
  /** how far down a card the line between a couple runs */
  spouseY: 112,
  /** how far above a child's card the line from its parents runs along */
  bus: 50,
  /** a second line along in the same gap sits this much higher */
  stagger: 10,
  /** how round a line's corners are */
  corner: 10,
  /** a chevron's radius, and how far its centre sits off the card */
  chevron: 11,
  off: 18
};
/** Who is drawn where, from what is open: each person once. */
class Builder {
  book;
  s;
  drawn = new Set();
  up;
  sibs;
  kids;
  constructor(book, s) {
    this.book = book;
    this.s = s;
    this.up = new Set(s.up);
    this.sibs = new Set(s.sibs);
    this.kids = new Set(s.kids);
  }
  claim(h) {
    if (this.drawn.has(h))
      return false;
    this.drawn.add(h);
    return true;
  }
  /** a person with the husband or wife drawn beside them, partners in the union's order */
  unit(row, h, line, union = shownUnion(this.book, this.s, h)) {
    const u = union ? this.book.unions[union] : null;
    const partner = u ? other(u, h) : null;
    const people = partner && this.claim(partner) ? [u.a, u.b] : [h];
    return { row, people, union, blood: h, line, focus: false, above: new Map(), kids: [] };
  }
  /** The focus and their forebears, as far as they are open: drawn before anyone else. */
  focusUnit() {
    this.claim(this.s.focus);
    const f = this.unit(0, this.s.focus, true);
    f.focus = true;
    this.rise(f);
    return f;
  }
  rise(u) {
    for (const h of u.people) {
      if (!this.up.has(h))
        continue;
      const pu = birthUnion(this.book, h);
      if (!pu)
        continue;
      const { a, b } = this.book.unions[pu];
      const ps = [a, b].filter((q) => !!q && this.claim(q));
      const parents = ps.length ? { row: u.row - 1, people: ps, union: pu, blood: ps[0], line: true, focus: false, above: new Map(), kids: [] } : null;
      u.above.set(h, { union: pu, parents, sibs: [], halves: [] });
      if (parents)
        this.rise(parents);
    }
  }
  /** Everyone off the line: the focus's children, then brothers and sisters, nearest generation first. */
  around(f) {
    if (f.union && this.kids.has(f.union))
      f.kids = this.children(f.union, 1);
    let level = [f];
    while (level.length) {
      const next = [];
      for (const u of level) {
        for (const h of u.people) {
          const a = u.above.get(h);
          if (!a)
            continue;
          if (this.sibs.has(h)) {
            const pu = this.book.unions[a.union];
            a.sibs = pu.kids.filter((k) => k !== h && this.claim(k)).map((k) => this.relative(u.row, k));
            for (const q of [pu.a, pu.b]) {
              if (!q)
                continue;
              for (const v of this.book.people[q].unions) {
                if (v === a.union)
                  continue;
                const units = this.book.unions[v].kids.filter((k) => k !== h && this.claim(k)).map((k) => this.relative(u.row, k));
                if (units.length)
                  a.halves.push({ parent: q, units });
              }
            }
          }
          if (a.parents)
            next.push(a.parents);
        }
      }
      level = next;
    }
  }
  children(union, row) {
    return this.book.unions[union].kids.filter((k) => this.claim(k)).map((k) => this.relative(row, k));
  }
  /** someone off the line, with their husband or wife and, when open, their children */
  relative(row, h) {
    const u = this.unit(row, h, false);
    if (u.union && this.kids.has(u.union))
      u.kids = this.children(u.union, row + 1);
    return u;
  }
}
// -- shapes ---------------------------------------------------------------------------------
/** What part of the drawing takes up in each row, and the units in it with their left edges. */
class Shape {
  rows = new Map();
  units = [];
  span(row, lo, hi) {
    const e = this.rows.get(row);
    this.rows.set(row, e ? [Math.min(e[0], lo), Math.max(e[1], hi)] : [lo, hi]);
    return this;
  }
  shift(dx) {
    if (!dx)
      return this;
    for (const e of this.rows.values()) {
      e[0] += dx;
      e[1] += dx;
    }
    for (const it of this.units)
      it.x += dx;
    return this;
  }
  /** the rows alone, as a copy: room taken, with nothing in it to draw twice */
  ghost() {
    const g = new Shape();
    for (const [r, [lo, hi]] of this.rows)
      g.rows.set(r, [lo, hi]);
    return g;
  }
  merge(o) {
    for (const [r, [lo, hi]] of o.rows)
      this.span(r, lo, hi);
    this.units.push(...o.units);
    return this;
  }
}
/** How far `b` must move right to stand `gap` clear of `a` in every row they share; null when they share none. */
function clearance(a, b, gap) {
  let need = null;
  for (const [r, [, hi]] of a.rows) {
    const e = b.rows.get(r);
    if (e)
      need = Math.max(need ?? -Infinity, hi + gap - e[0]);
  }
  return need;
}
const widthOf = (u) => (u.people.length === 2 ? FV.w * 2 + FV.pair : FV.w);
/** a person's centre, from their unit's left edge */
const centreIn = (u, h) => Math.max(0, u.people.indexOf(h)) * (FV.w + FV.pair) + FV.w / 2;
/** where the line to the children leaves: the middle of a couple's line, or the middle of a card */
const dropIn = (u) => widthOf(u) / 2;
/** Lays a drawing out; the sideways chevrons need room, so it asks the book who has brothers and sisters. */
class Layout {
  book;
  sibCount = new Map();
  constructor(book) {
    this.book = book;
  }
  hasSibs(h) {
    if (!this.sibCount.has(h))
      this.sibCount.set(h, siblingCount(this.book, h));
    return this.sibCount.get(h) > 0;
  }
  /** which way a person on the line opens their brothers and sisters: the husband's to the left, the wife's to the right */
  sideOf(u, h) {
    if (u.people.length === 2)
      return u.people[0] === h ? 'left' : 'right';
    return this.book.people[h].sex === 'f' ? 'right' : 'left';
  }
  /** room for the sideways chevrons at a unit's two ends */
  pads(u) {
    if (!u.line)
      return [0, 0];
    let l = 0;
    let r = 0;
    for (const h of u.people) {
      if (!this.hasSibs(h) || !birthUnion(this.book, h))
        continue;
      if (this.sideOf(u, h) === 'left')
        l = FV.side;
      else
        r = FV.side;
    }
    return [l, r];
  }
  /** Someone off the line with their children below: origin at their own centre. */
  desc(u) {
    const S = new Shape();
    const left = -centreIn(u, u.blood);
    const [pl, pr] = this.pads(u);
    S.span(u.row, left - pl, left + widthOf(u) + pr);
    S.units.push({ u, x: left });
    const K = this.kidsBlock(u);
    if (K)
      S.merge(K.shift(left + dropIn(u)));
    return S;
  }
  /** A unit's children side by side, centred under the point their line leaves from (the origin). */
  kidsBlock(u) {
    if (!u.kids.length)
      return null;
    const K = new Shape();
    const at = [];
    for (const k of u.kids) {
      const ks = this.desc(k);
      const dx = at.length ? (clearance(K, ks, FV.gap) ?? 0) : 0;
      ks.shift(dx);
      at.push(dx);
      K.merge(ks);
    }
    return K.shift(-(at[0] + at[at.length - 1]) / 2);
  }
  /**
   * Everything that hangs off one person on the line, origin at their centre: their card, their
   * brothers and sisters packed outward on `side` (kept clear of whatever stands under the
   * person, `under`), and, separately, their parents' whole shape, standing over them unless
   * something of it reaches down into their row, which then goes outside.
   */
  branch(u, h, side, under) {
    const r = u.row;
    const R = new Shape();
    const room = this.hasSibs(h) && birthUnion(this.book, h) ? FV.side : 0;
    R.span(r, -FV.w / 2 - (side === 'left' ? room : 0), FV.w / 2 + (side === 'right' ? room : 0));
    if (under)
      R.merge(under);
    const a = u.above.get(h);
    if (!a)
      return { R, Q: null };
    const halves = a.halves.flatMap((x) => x.units);
    const near = side === 'left' ? [...halves, ...a.sibs].reverse() : [...a.sibs, ...halves];
    if (near.length) {
      const shapes = near.map((s) => this.desc(s));
      // the brothers' and sisters' own children stay on their side of this person, clear of anything under them
      const wall = R.ghost();
      const deepest = Math.max(...shapes.flatMap((s) => [...s.rows.keys()]));
      for (let row = r + 1; row <= deepest; row++) {
        const e = wall.rows.get(row);
        wall.rows.set(row, side === 'left' ? [Math.min(e?.[0] ?? Infinity, -FV.w / 2), Infinity] : [-Infinity, Math.max(e?.[1] ?? -Infinity, FV.w / 2)]);
      }
      for (const sh of shapes) {
        if (side === 'left')
          sh.shift(-(clearance(sh, wall, FV.gap) ?? 0));
        else
          sh.shift(clearance(wall, sh, FV.gap) ?? 0);
        wall.merge(sh.ghost());
        R.merge(sh);
      }
    }
    if (!a.parents)
      return { R, Q: null };
    const Q = this.lineUnit(a.parents, null);
    const need = side === 'left' ? clearance(Q, R, FV.gap) : clearance(R, Q, FV.gap);
    if (need !== null && need > 0)
      Q.shift(side === 'left' ? -need : need);
    return { R, Q };
  }
  /** A unit on the line with everything above and beside it; origin at the point its children's line leaves from. */
  lineUnit(u, under) {
    const S = new Shape();
    if (u.people.length === 1) {
      const h = u.people[0];
      const { R, Q } = this.branch(u, h, this.sideOf(u, h), under);
      S.merge(R);
      if (Q)
        S.merge(Q);
      S.units.push({ u, x: -FV.w / 2 });
      return S;
    }
    const [h, w] = u.people;
    const half = (FV.w + FV.pair) / 2;
    const L = this.branch(u, h, 'left', under && under.ghost().shift(half));
    const R = this.branch(u, w, 'right', under && under.ghost().shift(-half));
    L.R.shift(-half);
    L.Q?.shift(-half);
    R.R.shift(half);
    R.Q?.shift(half);
    // the husband's forebears and the wife's move apart, half each way, until they clear each other
    if (L.Q && R.Q) {
      const need = clearance(L.Q, R.Q, FV.gap);
      if (need !== null && need > 0) {
        L.Q.shift(-need / 2);
        R.Q.shift(need / 2);
      }
    }
    for (const part of [L.R, L.Q, R.R, R.Q])
      if (part)
        S.merge(part);
    S.units.push({ u, x: -half - FV.w / 2 });
    return S;
  }
}
// -- lines ----------------------------------------------------------------------------------
/** Down from a parent's point, along at `bus`, and down into a child's card, with rounded corners. */
export function elbow(x0, y0, x1, y1, bus, corner = FV.corner) {
  const n = (v) => Math.round(v * 10) / 10;
  if (Math.abs(x1 - x0) < 0.5)
    return `M${n(x0)},${n(y0)}V${n(y1)}`;
  const dir = Math.sign(x1 - x0);
  const r = Math.max(0, Math.min(corner, Math.abs(x1 - x0) / 2, bus - y0, y1 - bus));
  return `M${n(x0)},${n(y0)}V${n(bus - r)}Q${n(x0)},${n(bus)} ${n(x0 + dir * r)},${n(bus)}H${n(x1 - dir * r)}Q${n(x1)},${n(bus)} ${n(x1)},${n(bus + r)}V${n(y1)}`;
}
// -- relationships to the home person ----------------------------------------------------------
/** How many generations above `home` each of their forebears is (1: a parent); the nearest way, when there are two. */
export function forebears(book, home) {
  const gen = new Map();
  let level = [home];
  for (let n = 1; level.length && n <= 40; n++) {
    const next = [];
    for (const h of level) {
      for (const u of book.people[h]?.parents ?? []) {
        for (const q of [book.unions[u]?.a, book.unions[u]?.b]) {
          if (q && q !== home && !gen.has(q)) {
            gen.set(q, n);
            next.push(q);
          }
        }
      }
    }
    level = next;
  }
  return gen;
}
const ordinal = (n) => {
  const t = n % 100;
  return `${n}${t >= 11 && t <= 13 ? 'th' : { 1: 'st', 2: 'nd', 3: 'rd' }[n % 10] ?? 'th'}`;
};
/** "parents", "grandparents", "great-grandparents", "2nd great-grandparents"; one person: "father", "great-grandmother". */
export function forebearWord(n, sex) {
  const one = sex === 'm' ? 'father' : sex === 'f' ? 'mother' : 'parent';
  const base = sex === null ? 'parents' : one;
  if (n === 1)
    return base;
  if (n === 2)
    return `grand${base}`;
  return `${n === 3 ? '' : `${ordinal(n - 2)} `}great-grand${base}`;
}
/** A first name for "Sam's grandparents": the first word of the name. */
const firstName = (name) => name.trim().split(/\s+/)[0] || name;
// -- the whole drawing ----------------------------------------------------------------------
/**
 * The drawing for what is open: cards, the lines between them, the chevrons that open and
 * fold, the switches between husbands and wives, and under each couple of `home`'s forebears
 * what they are to them ("Sam's grandparents"). Origin at the focus's couple (or the focus).
 */
export function familyLayout(book, s, home = null) {
  const b = new Builder(book, s);
  const focus = b.focusUnit();
  b.around(focus);
  const L = new Layout(book);
  const D = L.kidsBlock(focus);
  const S = L.lineUnit(focus, D && D.ghost());
  if (D)
    S.merge(D);
  const top = (row) => row * FV.row;
  const at = new Map(S.units.map(({ u, x }) => [u, x]));
  const centre = (u, h) => at.get(u) + centreIn(u, h);
  const drop = (u) => ({ x: at.get(u) + dropIn(u), y: top(u.row) + (u.people.length === 2 ? FV.spouseY : FV.h) });
  const kin = home && book.people[home] ? forebears(book, home) : new Map();
  const homeName = home && book.people[home] ? firstName(book.people[home].name) : '';
  const cards = [];
  const chevrons = [];
  const switches = [];
  const lines = [];
  const labels = [];
  const links = [];
  for (const { u, x } of S.units) {
    const y = top(u.row);
    u.people.forEach((h, i) => {
      const p = book.people[h];
      cards.push({ id: h, x: x + i * (FV.w + FV.pair), y, name: p.name, years: p.years, sex: p.sex, living: p.living, photo: p.photo, records: p.records, focus: h === s.focus, home: h === home });
    });
    // the line between a couple: the husband's half, then the wife's
    if (u.people.length === 2) {
      const x0 = x + FV.w;
      const mid = x0 + FV.pair / 2;
      const ly = y + FV.spouseY;
      lines.push({ key: `h:${u.union}`, d: `M${x0},${ly}H${mid}`, kind: 'husband' }, { key: `w:${u.union}`, d: `M${mid},${ly}H${x0 + FV.pair}`, kind: 'wife' });
    }
    const name = (h) => book.people[h].name;
    // parents, and brothers and sisters, of each person on the line
    if (u.line) {
      for (const h of u.people) {
        const cx = centre(u, h);
        const pu = birthUnion(book, h);
        if (!pu)
          continue;
        const a = u.above.get(h);
        // parents already drawn elsewhere (cousins who married) get no second chevron
        if (!a || a.parents)
          chevrons.push({ key: `up:${h}`, x: cx, y: y - FV.off, dir: a ? 'down' : 'up', act: { kind: 'up', id: h }, open: !!a, label: a ? `Fold away ${name(h)}'s parents` : `Show ${name(h)}'s parents`, anchor: h });
        if (L.hasSibs(h) && (!a || a.parents)) {
          const side = L.sideOf(u, h);
          const open = !!a && b.sibs.has(h);
          const out = side === 'left' ? 'left' : 'right';
          const back = side === 'left' ? 'right' : 'left';
          chevrons.push({ key: `sibs:${h}`, x: side === 'left' ? cx - FV.w / 2 - FV.off : cx + FV.w / 2 + FV.off, y: y + 50, dir: open ? back : out, act: { kind: 'sibs', id: h }, open, label: open ? `Fold away ${name(h)}'s brothers and sisters` : `Show ${name(h)}'s brothers and sisters`, anchor: h });
        }
        if (a?.parents) {
          const to = [{ x: cx, y }, ...a.sibs.map((k) => ({ x: centre(k, k.blood), y }))];
          links.push({ key: `p:${h}`, from: drop(a.parents), to, row: u.row });
          for (const half of a.halves) {
            if (!a.parents.people.includes(half.parent))
              continue;
            links.push({ key: `half:${h}:${half.parent}`, from: { x: centre(a.parents, half.parent), y: top(a.parents.row) + FV.h }, to: half.units.map((k) => ({ x: centre(k, k.blood), y })), row: u.row });
          }
        }
      }
    }
    // children: under the focus, and under anyone off the line
    if (u.union && (!u.line || u.focus) && book.unions[u.union].kids.length) {
      const open = b.kids.has(u.union);
      const d = drop(u);
      const who = u.people.length === 2 ? `${firstName(name(u.people[0]))} and ${firstName(name(u.people[1]))}` : name(u.blood);
      chevrons.push({ key: `kids:${u.union}`, x: d.x, y: y + FV.h + FV.off, dir: open ? 'up' : 'down', act: { kind: 'kids', id: u.union }, open, label: open ? `Fold away ${who}'s children` : `Show ${who}'s children`, anchor: u.blood });
      if (u.kids.length)
        links.push({ key: `k:${u.union}`, from: d, to: u.kids.map((k) => ({ x: centre(k, k.blood), y: top(k.row) })), row: u.row + 1 });
    }
    // more than one husband or wife: the next one
    if ((!u.line || u.focus) && partnersOf(book, u.blood).length > 1) {
      const right = centre(u, u.blood) + FV.w / 2;
      switches.push({ key: `sp:${u.blood}`, x: right, y, act: { kind: 'spouse', id: u.blood }, label: `Show ${name(u.blood)}'s other husband or wife`, anchor: u.blood });
    }
    // what a couple of the home person's forebears are to them
    const gens = u.people.map((h) => kin.get(h) ?? 0);
    const both = u.people.length === 2 && gens[0] > 0 && gens[0] === gens[1];
    const one = u.people.findIndex((_, i) => gens[i] > 0);
    if (homeName && one >= 0) {
      const word = both ? forebearWord(gens[0], null) : forebearWord(gens[one], book.people[u.people[one]].sex);
      const lx = both ? drop(u).x : centre(u, u.people[one]);
      const kidsChevron = u.union && (!u.line || u.focus) && book.unions[u.union].kids.length;
      labels.push({ key: `l:${u.people.join('+')}`, x: lx, y: y + FV.h + (kidsChevron ? 48 : 26), text: `${homeName}'s ${word}` });
    }
  }
  // the lines along: one height per gap, a little higher for each that would run over another
  const byRow = new Map();
  for (const l of links)
    byRow.set(l.row, [...(byRow.get(l.row) ?? []), l]);
  for (const [row, ls] of byRow) {
    const spans = ls.map((l) => ({ l, lo: Math.min(l.from.x, ...l.to.map((t) => t.x)), hi: Math.max(l.from.x, ...l.to.map((t) => t.x)) })).sort((p, q) => p.lo - q.lo);
    const ends = [];
    for (const sp of spans) {
      const straight = sp.hi - sp.lo < 0.5;
      let level = straight ? 0 : ends.findIndex((e) => e + 12 < sp.lo);
      if (!straight) {
        if (level < 0)
          level = ends.length;
        ends[level] = sp.hi;
      }
      const bus = top(row) - FV.bus - level * FV.stagger;
      sp.l.to.forEach((t, i) => lines.push({ key: `${sp.l.key}:${i}`, d: elbow(sp.l.from.x, sp.l.from.y, t.x, t.y, bus), kind: 'child' }));
    }
  }
  let x0 = Infinity;
  let y0 = Infinity;
  let x1 = -Infinity;
  let y1 = -Infinity;
  const take = (ax, ay, bx, by) => {
    x0 = Math.min(x0, ax);
    y0 = Math.min(y0, ay);
    x1 = Math.max(x1, bx);
    y1 = Math.max(y1, by);
  };
  for (const c of cards)
    take(c.x, c.y, c.x + FV.w, c.y + FV.h);
  for (const c of chevrons)
    take(c.x - FV.chevron, c.y - FV.chevron, c.x + FV.chevron, c.y + FV.chevron);
  for (const l of labels)
    take(l.x - Math.min(110, l.text.length * 4.2), l.y - 22, l.x + Math.min(110, l.text.length * 4.2), l.y + 22);
  const f = cards.find((c) => c.focus);
  return { cards, chevrons, switches, lines, labels, bounds: { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }, focus: { x: f.x, y: f.y } };
}
