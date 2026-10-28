#pragma frame {"size":[1600,720],"center":[0.0,1.1,0.0],"zoom":108,"bounds":[[-6.2,-0.45,-6.2],[6.2,3.3,6.2]]}

float plinth2(vec3 p) { return sdRect(vec2(p.x - p.z, p.x + p.z) * 0.70710678, vec2(5.6, 2.7), 1.7); }

float roads2(vec3 p) {
  float dx = abs(repC(p.x, 2.6, 3.0)) - 0.24;
  float dz = abs(repC(p.z, 2.6, 3.0)) - 0.24;
  return min(dx, dz);
}

float marks2(vec3 p) {
  float rx = repC(p.x, 2.6, 3.0), rz = repC(p.z, 2.6, 3.0);
  float alongX = max(abs(rz) - 0.012, abs(mod(p.x, 0.3) - 0.15) - 0.07);
  alongX = max(alongX, -(abs(rx) - 0.55));
  float alongZ = max(abs(rx) - 0.012, abs(mod(p.z, 0.3) - 0.15) - 0.07);
  alongZ = max(alongZ, -(abs(rz) - 0.55));
  float zx = max(max(abs(abs(rx) - 0.4) - 0.1, abs(rz) - 0.2), abs(mod(p.z, 0.08) - 0.04) - 0.02);
  float zz = max(max(abs(abs(rz) - 0.4) - 0.1, abs(rx) - 0.2), abs(mod(p.x, 0.08) - 0.04) - 0.02);
  return min(min(alongX, alongZ), min(zx, zz));
}

vec2 base(vec3 p) {
  float d2 = plinth2(p);
  vec2 r = M(extrude(d2, p.y + 0.1, 0.1), WHITE + ROUND);
  r = U(r, M(extrude(d2 - 0.07, p.y + 0.27, 0.07), WHITE + ROUND));
  r = seam(r, p.y + 0.2);
  vec2 pk = vec2(p.x - 1.3, p.z - 1.3);
  float park = max(sdRect(pk, vec2(0.98, 0.98), 0.12), d2 + 0.14);
  vec3 top = faceY(p, 0.0);
  r = decal(r, top, park, MINT);
  float road = max(roads2(p), d2 + 0.14);
  r = Sc(r, M(max(road, -(p.y + 0.025)), STEEL));
  r = decal(r, faceY(p, -0.025), max(marks2(p), d2 + 0.14), WHITE);
  vec2 lq = p.xz - vec2(-1.5, 3.3);
  r = decal(r, top, sdRect(lq, vec2(0.56, 0.3), 0.04), STEEL);
  r = decal(r, top, max(abs(repC(lq.x, 0.37, 4.0)) - 0.014, abs(lq.y) - 0.25), WHITE);
  float pond = length((pk - vec2(0.12, -0.05)) / vec2(1.0, 0.7)) - 0.42;
  r = Sc(r, M(max(pond, -(p.y + 0.04)), SKY));
  float path = abs(length((pk - vec2(0.12, -0.05)) / vec2(1.0, 0.7)) - 0.56) - 0.05;
  path = min(path, max(abs(pk.x - pk.y + 0.1) - 0.05, -(length((pk - vec2(0.12, -0.05)) / vec2(1.0, 0.7)) - 0.56)));
  r = decal(r, top, max(max(path, park), -pond), WHITE);
  return r;
}

vec2 tower(vec3 p) {
  vec3 b = p - vec3(0.0, 1.3, 0.0);
  vec2 t = M(sdBox(b, vec3(0.5, 1.3, 0.5)), WHITE);
  t = paint(t, p.y - 0.36, STEEL);
  t = seam(t, p.y - 0.36);
  vec3 fx = faceX(p, 0.5);
  float rx = repI(fx.z - 1.5, 0.19, 11.0);
  vec3 cx = vec3(repC(fx.x, 0.19, 5.0), fx.y, repC(fx.z - 1.5, 0.19, 11.0));
  t = inset(t, cx, vec2(0.075, 0.068), 0.012, 0.025, rx == 10.0 ? LIME : SKY);
  vec3 fz = faceZ(p, 0.5);
  float rz = repI(fz.z - 1.5, 0.19, 11.0);
  vec3 cz = vec3(repC(fz.x, 0.19, 5.0), fz.y, repC(fz.z - 1.5, 0.19, 11.0));
  t = inset(t, cz, vec2(0.075, 0.068), 0.012, 0.025, rz == 10.0 ? LIME : SKY);
  t = inset(t, fz - vec3(0.0, 0.0, 0.16), vec2(0.16, 0.16), 0.02, 0.03, GRAPHITE);
  t = inset(t, fx - vec3(0.0, 0.0, 0.16), vec2(0.3, 0.12), 0.02, 0.03, SKY);
  vec2 r = t;
  r = U(r, M(sdBox(p - vec3(0.0, 0.34, 0.6), vec3(0.24, 0.018, 0.12)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 2.62, 0.0), vec3(0.54, 0.035, 0.54)), WHITE));
  vec3 rb = p - vec3(-0.12, 2.78, -0.1);
  vec2 box = M(sdBox(rb, vec3(0.2, 0.13, 0.17)), WHITE);
  box = vents(box, faceX(rb, 0.2) - vec3(0.0, 0.0, 0.02), 4.0, 0.06, 0.14, 0.025, GRAPHITE);
  r = U(r, box);
  vec3 an = p - vec3(0.32, 2.65, -0.32);
  r = U(r, M(sdCylinder(an - vec3(0.0, 0.25, 0.0), 0.014, 0.25), GRAPHITE + SOLID));
  r = U(r, led(an - vec3(0.0, 0.52, 0.0), 0.035, CORAL));
  return r;
}

vec2 slab(vec3 p, float k) {
  float off = (k == 6.0 || k == 13.0) ? 0.13 : 0.0;
  vec3 q = rotY(p - vec3(off, 0.22 + k * 0.1, 0.0), 0.05 * sin(k * 2.3));
  vec2 s = M(sdRoundBox(q, vec3(0.38, 0.024, 0.31), 0.012), k == 20.0 ? LILAC : WHITE);
  return s;
}

vec2 pages(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.08, 0.0), vec3(0.44, 0.08, 0.37)), WHITE);
  r = seam(r, p.y - 0.12);
  r = U(r, M(sdBox(p - vec3(0.0, 1.2, 0.0), vec3(0.15, 1.12, 0.15)), GRAPHITE));
  r = inset(r, faceZ(p, 0.37) - vec3(0.0, 0.0, 0.08), vec2(0.12, 0.06), 0.02, 0.02, GRAPHITE);
  float k = clamp(round((p.y - 0.22) / 0.1), 0.0, 20.0);
  r = U(r, slab(p, k));
  r = U(r, slab(p, max(k - 1.0, 0.0)));
  r = U(r, slab(p, min(k + 1.0, 20.0)));
  vec3 an = p - vec3(-0.1, 2.24, -0.08);
  r = U(r, M(sdCylinder(an - vec3(0.0, 0.18, 0.0), 0.014, 0.18), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(an - vec3(0.0, 0.38, 0.0), 0.04), WHITE + SOLID));
  return r;
}

vec2 folder(vec3 p, float m) {
  vec2 f = M(sdBox(p, vec3(0.12, 0.016, 0.09)), m);
  f = U(f, M(sdBox(p - vec3(-0.06, 0.0, -0.1), vec3(0.04, 0.008, 0.014)), m));
  return seam(f, p.y - 0.004);
}

vec2 warehouse(vec3 p) {
  vec3 w = p - vec3(0.0, 0.3, 0.0);
  vec2 r = M(sdBox(w, vec3(0.85, 0.3, 0.55)), WHITE);
  r = paint(r, p.y - 0.08, STEEL);
  vec3 fz = faceZ(p, 0.55);
  float di = repI(fz.x, 0.54, 3.0);
  vec3 dq = vec3(repC(fz.x, 0.54, 3.0), fz.y, fz.z - 0.24);
  r = Sc(r, M(max(sdRect(dq.xz, vec2(0.18, 0.2), 0.0), -(dq.y + 0.3)), GRAPHITE));
  vec2 fr = M(extrude(max(sdRect(dq.xz, vec2(0.21, 0.23), 0.02), -sdRect(dq.xz, vec2(0.17, 0.19), 0.0)), dq.y, 0.02), SUN);
  r = U(r, fr);
  float open = di == 1.0 ? 0.11 : (di == 0.0 ? 0.0 : -0.1);
  vec3 sh = dq - vec3(0.0, -0.04, 0.1 + open * 0.5);
  float hh = 0.1 - open * 0.5;
  vec2 shut = M(extrude(sdRect(sh.xz, vec2(0.18, hh), 0.0), sh.y, 0.008), WHITE);
  r = U(r, stripes(shut, sh.z, 0.025));
  vec3 fx = faceX(p, 0.85);
  r = inset(r, vec3(repC(fx.x, 0.3, 3.0), fx.y, fx.z - 0.4), vec2(0.09, 0.06), 0.015, 0.02, SKY);
  r = Sc(r, M(max(sdRect(fx.xz - vec2(0.0, 0.17), vec2(0.1, 0.17), 0.0), -(fx.y + 0.03)), GRAPHITE));
  vec3 rq = vec3(p.x, p.y + 0.11, p.z);
  float barrel = max(length(rq.yz) - 0.91, abs(p.x) - 0.88);
  vec2 roof = M(max(barrel, 0.6 - p.y), STEEL + SOLID);
  roof = stripes(roof, p.x, 0.05);
  r = U(r, roof);
  vec3 sk = vec3(repC(p.x, 0.5, 3.0), p.y, p.z);
  float skl = max(max(length(rq.yz) - 0.93, abs(sk.x) - 0.12), max(abs(p.z) - 0.18, 0.6 - p.y));
  r = U(r, M(skl, SKY + SOLID));
  vec3 inside = vec3(dq.x, dq.y + 0.12, dq.z);
  if (di == 2.0) {
    r = U(r, folder(rotY(vec3(inside.x, inside.z - 0.04, inside.y), 0.1) , SUN));
  }
  return r;
}

vec2 filebox(vec3 p, float m) {
  vec2 b = M(sdBox(p - vec3(0.0, 0.07, 0.0), vec3(0.11, 0.07, 0.08)), WHITE);
  b = seam(b, p.y - 0.11);
  vec3 f = faceZ(p, 0.08);
  return inset(b, f - vec3(0.0, 0.0, 0.06), vec2(0.05, 0.018), 0.018, 0.01, m);
}

vec2 dome(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.05, 0.0), 0.92, 0.05), WHITE + SOLID);
  vec2 drum = M(sdCylinder(p - vec3(0.0, 0.42, 0.0), 0.78, 0.32), WHITE + SOLID);
  drum = stripes(drum, p.y - 0.1, 0.21);
  vec3 f = faceX(rotY(p, -PI * 0.25), 0.78);
  float row = repI(f.z - 0.42, 0.21, 3.0);
  vec3 lq = vec3(f.x, f.y, repC(f.z - 0.42, 0.21, 3.0));
  drum = vents(drum, lq - vec3(-0.14, 0.0, 0.0), 4.0, 0.05, 0.1, 0.02, GRAPHITE);
  r = U(r, drum);
  r = U(r, led(lq - vec3(0.12, 0.0, 0.0), 0.03, row == 2.0 ? LIME : STEEL));
  r = U(r, M(sdTorus(p - vec3(0.0, 0.76, 0.0), 0.79, 0.022), STEEL + SOLID));
  vec2 dm = M(max(sdEllipsoid(p - vec3(0.0, 0.76, 0.0), vec3(0.74, 0.6, 0.74)), 0.76 - p.y), WHITE + SOLID);
  dm = stripes(dm, atan(p.z, p.x) * 4.0, PI * 0.25);
  dm = paint(dm, 1.18 - p.y, LIME + SOLID);
  r = U(r, dm);
  vec3 ln = p - vec3(0.0, 1.38, 0.0);
  r = U(r, M(sdCylinder(ln, 0.12, 0.06), LIME + SOLID));
  r = U(r, M(sdCylinder(ln - vec3(0.0, 0.08, 0.0), 0.15, 0.02), WHITE + SOLID));
  r = U(r, M(sdSphere(ln - vec3(0.0, 0.14, 0.0), 0.04), GRAPHITE + SOLID));
  vec3 en = p - vec3(-0.25, 0.25, 0.72);
  vec2 porch = M(sdBox(en, vec3(0.2, 0.25, 0.16)), WHITE);
  porch = inset(porch, faceZ(en, 0.16) - vec3(0.0, 0.0, -0.06), vec2(0.1, 0.16), 0.09, 0.03, GRAPHITE);
  r = U(r, porch);
  r = U(r, M(sdBox(en - vec3(0.0, 0.27, 0.02), vec3(0.24, 0.025, 0.2)), STEEL));
  return r;
}

vec2 agents(vec3 p) {
  vec3 b = p - vec3(0.0, 0.45, 0.0);
  vec2 r = M(sdBox(b, vec3(0.5, 0.45, 0.5)), WHITE);
  r = paint(r, p.y - 0.1, STEEL);
  r = seam(r, p.y - 0.47);
  vec3 fz = faceZ(p, 0.5);
  r = inset(r, vec3(repC(fz.x, 0.3, 3.0), fz.y, fz.z - 0.68), vec2(0.1, 0.1), 0.02, 0.025, SKY);
  r = inset(r, vec3(fz.x - 0.3, fz.y, fz.z - 0.27), vec2(0.1, 0.1), 0.02, 0.025, SKY);
  r = inset(r, fz - vec3(-0.15, 0.0, 0.2), vec2(0.1, 0.2), 0.02, 0.03, GRAPHITE);
  vec3 fx = faceX(p, 0.5);
  r = inset(r, vec3(repC(fx.x, 0.3, 3.0), fx.y, fx.z - 0.68), vec2(0.1, 0.1), 0.02, 0.025, SKY);
  r = inset(r, vec3(repC(fx.x, 0.3, 3.0), fx.y, fx.z - 0.27), vec2(0.1, 0.1), 0.02, 0.025, SKY);
  vec3 rt = p - vec3(0.0, 0.93, 0.0);
  r = U(r, M(max(sdBox(rt, vec3(0.52, 0.04, 0.52)), -sdBox(rt - vec3(0.0, 0.03, 0.0), vec3(0.47, 0.06, 0.47))), WHITE));
  vec3 c = p - vec3(0.0, 0.9, 0.0);
  r = U(r, M(sdCylinder(c - vec3(0.0, 0.08, 0.0), 0.16, 0.08), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(c - vec3(0.0, 0.26, 0.0), 0.06, 0.12), STEEL + SOLID));
  vec3 a = normalize(vec3(-0.3, 1.0, 0.6));
  vec3 e1 = normalize(cross(a, vec3(1.0, 0.0, 0.0)));
  vec3 e2 = cross(a, e1);
  vec3 dc = p - vec3(0.0, 1.38, 0.0);
  float R = 0.62;
  float shell = abs(length(dc - a * R) - R) - 0.022;
  vec2 dish = M(max(shell, dot(dc, a) - 0.13), LILAC + SOLID);
  r = U(r, dish);
  r = U(r, M(pipe(p, vec3(0.0, 1.2, 0.0), vec3(0.0, 1.38, 0.0) + a * 0.02, 0.05), GRAPHITE + SOLID));
  vec3 fp = a * 0.34;
  for (int i = 0; i < 3; i++) {
    float an = TAU * float(i) / 3.0 + 0.4;
    vec3 rim = a * 0.11 + (e1 * cos(an) + e2 * sin(an)) * 0.37;
    r = U(r, M(pipe(dc, rim, fp, 0.01), GRAPHITE + SOLID));
  }
  r = U(r, M(sdSphere(dc - fp, 0.045), GRAPHITE + SOLID));
  vec3 rl = vec3(dot(dc, e1), dot(dc, a) - 0.13, dot(dc, e2));
  r = U(r, M(sdTorus(rl, 0.385, 0.018), LILAC + SOLID));
  r = U(r, led(faceY(rt, 0.04) - vec3(0.38, 0.0, 0.38), 0.03, LIME));
  return r;
}

vec2 lab(vec3 p) {
  vec3 b = p - vec3(0.0, 0.3, 0.0);
  vec2 r = M(sdBox(b, vec3(0.55, 0.3, 0.4)), WHITE);
  r = paint(r, p.y - 0.08, STEEL);
  vec3 fz = faceZ(p, 0.4);
  r = inset(r, vec3(fz.x - 0.22, fz.y, fz.z - 0.32), vec2(0.22, 0.12), 0.02, 0.025, SKY);
  r = inset(r, fz - vec3(-0.3, 0.0, 0.2), vec2(0.09, 0.2), 0.02, 0.03, GRAPHITE);
  vec3 fx = faceX(p, 0.55);
  r = inset(r, vec3(repC(fx.x, 0.3, 2.0), fx.y, fx.z - 0.32), vec2(0.09, 0.1), 0.045, 0.025, SKY);
  vec3 rq = vec3(p.z, p.y - 0.6, p.x);
  vec2 roof = M(sdPrism(rq, 0.48, 0.28, 0.6), CORAL);
  roof = stripes(roof, p.x + 0.6, 0.1);
  r = U(r, roof);
  vec3 vs = p - vec3(-0.25, 0.0, -0.1);
  r = U(r, M(sdCylinder(vs - vec3(0.0, 0.92, 0.0), 0.05, 0.12), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(vs - vec3(0.0, 1.05, 0.0), 0.08, 0.015), GRAPHITE + SOLID));
  vec3 tk = p - vec3(0.78, 0.0, 0.0);
  r = U(r, M(sdCylinder(tk - vec3(0.0, 0.03, 0.0), 0.2, 0.03), WHITE + SOLID));
  vec2 tank = M(sdCylinder(tk - vec3(0.0, 0.3, 0.0), 0.15, 0.24), SKY + SOLID);
  tank = paint(tank, tk.y - 0.3, MINT + SOLID);
  r = U(r, tank);
  r = U(r, M(sdSphere(tk - vec3(0.0, 0.54, 0.0), 0.15), WHITE + SOLID));
  r = U(r, M(sdTorus(tk - vec3(0.0, 0.3, 0.0), 0.155, 0.012), GRAPHITE + SOLID));
  r = U(r, M(pipe(p, vec3(0.55, 0.45, 0.0), vec3(0.78, 0.66, 0.0), 0.025), STEEL + SOLID));
  r = U(r, M(pipe(p, vec3(0.78, 0.66, 0.0), vec3(0.78, 0.7, 0.0), 0.025), STEEL + SOLID));
  for (int i = 0; i < 3; i++) {
    vec3 bq = tk - vec3(0.04 * float(i) - 0.04, 0.42 + 0.08 * float(i), 0.06 - 0.04 * float(i));
    r = U(r, M(sdSphere(bq, 0.022 + 0.008 * float(i)), WHITE + SOLID));
  }
  return r;
}

vec2 cart(vec3 p, float m) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.08, 0.0), vec3(0.17, 0.035, 0.1), 0.02), WHITE);
  vec3 w = vec3(repC(p.x, 0.22, 2.0), p.y - 0.045, abs(p.z) - 0.09);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.045, 0.02), GRAPHITE + SOLID));
  r = U(r, led(faceX(p - vec3(0.0, 0.08, 0.0), 0.19) - vec3(0.06, 0.0, 0.0), 0.02, SUN));
  r = U(r, M(sdRoundBox(p - vec3(-0.02, 0.2, 0.0), vec3(0.085, 0.085, 0.085), 0.015), m));
  return r;
}

vec2 drone(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.11, 0.04, 0.11), 0.03), WHITE);
  vec3 m = mirXZ(p);
  r = U(r, M(pipe(m, vec3(0.0), vec3(0.17, 0.02, 0.17), 0.016), GRAPHITE + SOLID));
  vec3 ro = m - vec3(0.17, 0.05, 0.17);
  r = U(r, M(sdCylinder(ro, 0.02, 0.03), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(ro - vec3(0.0, 0.03, 0.0), 0.12, 0.006), STEEL + SOLID));
  r = U(r, led(faceY(p, 0.04) - vec3(0.0, 0.0, 0.06), 0.025, LIME));
  r = U(r, M(pipe(p, vec3(0.0, -0.04, 0.0), vec3(0.0, -0.22, 0.0), 0.008), GRAPHITE + SOLID));
  r = U(r, M(sdRoundBox(p - vec3(0.0, -0.3, 0.0), vec3(0.08, 0.08, 0.08), 0.015), LILAC));
  return r;
}

vec2 lamp(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.015, 0.0), 0.05, 0.015), GRAPHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.32, 0.0), 0.014, 0.32), GRAPHITE + SOLID));
  r = U(r, M(pipe(p, vec3(0.0, 0.62, 0.0), vec3(0.1, 0.62, 0.1), 0.012), GRAPHITE + SOLID));
  vec3 h = p - vec3(0.12, 0.6, 0.12);
  r = U(r, M(sdCone(h, 0.025, 0.07, 0.025), STEEL + SOLID));
  r = U(r, M(max(sdSphere(h + vec3(0.0, 0.02, 0.0), 0.045), h.y + 0.02), SUN + SOLID));
  return r;
}

vec2 tree(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.02, 0.0), 0.1 * s, 0.02), WHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.2 * s, 0.0), 0.025 * s, 0.2 * s), GRAPHITE + SOLID));
  r = U(r, M(sdEllipsoid(p - vec3(0.0, 0.42 * s, 0.0), vec3(0.18, 0.22, 0.18) * s), MINT + SOLID));
  return r;
}

vec2 pine(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.02, 0.0), 0.09 * s, 0.02), WHITE + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.1 * s, 0.0), 0.024 * s, 0.1 * s), GRAPHITE + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.28 * s, 0.0), 0.11 * s, 0.17 * s, 0.07 * s), MINT + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.44 * s, 0.0), 0.09 * s, 0.13 * s, 0.035 * s), MINT + SOLID));
  r = U(r, M(sdCone(p - vec3(0.0, 0.57 * s, 0.0), 0.08 * s, 0.09 * s, 0.0), MINT + SOLID));
  return r;
}

vec2 bench(vec3 p) {
  vec3 s = p - vec3(0.0, 0.1, 0.0);
  vec2 r = M(sdBox(s, vec3(0.16, 0.01, 0.05)), WHITE);
  r = stripes(r, s.z + 0.05, 0.025);
  r = U(r, M(sdBox(p - vec3(0.0, 0.17, -0.055), vec3(0.16, 0.03, 0.008)), WHITE));
  vec3 l = vec3(abs(p.x) - 0.13, p.y - 0.085, p.z);
  r = U(r, M(sdBox(l, vec3(0.01, 0.085, 0.045)), GRAPHITE));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(-1.3, 1.5, -1.3), vec3(0.65, 1.55, 0.7), r.x)) r = U(r, tower(p - vec3(-1.3, 0.0, -1.3)));
  if (near(p - vec3(1.45, 1.25, -3.55), vec3(0.6, 1.4, 0.5), r.x)) r = U(r, pages(p - vec3(1.4, 0.0, -3.55)));
  if (near(p - vec3(-1.3, 0.4, 1.3), vec3(0.95, 0.45, 0.65), r.x)) r = U(r, warehouse(p - vec3(-1.3, 0.0, 1.3)));
  if (near(p - vec3(1.3, 0.75, -1.3), vec3(1.0, 0.8, 1.0), r.x)) r = U(r, dome(p - vec3(1.3, 0.0, -1.3)));
  if (near(p - vec3(-3.55, 0.85, 1.05), vec3(0.65, 0.9, 0.65), r.x)) r = U(r, agents(p - vec3(-3.55, 0.0, 1.05)));
  if (near(p - vec3(3.6, 0.5, -1.05), vec3(1.05, 0.6, 0.5), r.x)) r = U(r, lab(p - vec3(3.45, 0.0, -1.05)));
  if (near(p - vec3(0.0, 0.2, 1.2), vec3(0.3, 0.3, 1.2), r.x)) {
    r = U(r, cart(rotY(p - vec3(0.12, -0.025, 1.7), PI * 0.5), LIME));
    r = U(r, cart(rotY(p - vec3(-0.12, -0.025, 0.85), -PI * 0.5), CORAL));
  }
  if (near(p - vec3(1.2, 0.2, 0.0), vec3(1.2, 0.3, 0.3), r.x)) r = U(r, cart(p - vec3(1.75, -0.025, 0.12), BLUE));
  if (near(p - vec3(-1.6, 0.2, 0.0), vec3(1.2, 0.3, 0.3), r.x)) r = U(r, cart(p - vec3(-1.9, -0.025, -0.12), LIME));
  if (near(p - vec3(2.6, 0.2, -2.5), vec3(0.3, 0.3, 0.9), r.x)) r = U(r, cart(rotY(p - vec3(2.48, -0.025, -2.2), -PI * 0.5), LILAC));
  if (near(p - vec3(-2.6, 0.2, 2.6), vec3(0.3, 0.3, 0.9), r.x)) r = U(r, cart(rotY(p - vec3(-2.72, -0.025, 2.7), -PI * 0.5), WHITE));
  if (near(p - vec3(1.3, 0.35, 1.3), vec3(0.95, 0.4, 0.95), r.x)) {
    r = U(r, tree(p - vec3(0.6, 0.0, 0.6), 1.1));
    r = U(r, tree(p - vec3(2.0, 0.0, 0.62), 1.0));
    r = U(r, pine(p - vec3(0.62, 0.0, 1.95), 1.0));
    r = U(r, bench(rotY(p - vec3(1.4, 0.0, 0.55), 0.0)));
    r = U(r, bench(rotY(p - vec3(0.55, 0.0, 1.35), -PI * 0.5)));
  }
  if (near(p - vec3(0.0, 0.35, 0.0), vec3(2.0, 0.4, 2.0), r.x)) {
    r = U(r, lamp(vec3(0.38 - abs(p.x), p.y, 0.38 - abs(p.z))));
  }
  if (near(p - vec3(-1.4, 0.4, 3.5), vec3(0.9, 0.5, 0.6), r.x)) {
    r = U(r, pine(p - vec3(-0.75, 0.0, 3.25), 1.1));
    r = U(r, tree(p - vec3(-2.1, 0.0, 3.75), 1.0));
    r = U(r, cart(rotY(p - vec3(-1.87, 0.0, 3.36), -PI * 0.5), WHITE));
    r = U(r, cart(rotY(p - vec3(-1.5, 0.0, 3.36), -PI * 0.5), LIME));
    vec3 cp = vec3(repC(p.x + 1.5, 0.37, 3.0), p.y, p.z - 3.08);
    r = U(r, M(sdBox(cp - vec3(0.0, 0.13, 0.0), vec3(0.045, 0.13, 0.035)), STEEL));
    r = U(r, led(faceZ(cp, 0.035) - vec3(0.0, 0.0, 0.2), 0.022, LIME));
  }
  if (near(p - vec3(-3.5, 0.4, 3.5), vec3(0.6, 0.5, 0.6), r.x)) {
    r = U(r, pine(p - vec3(-3.7, 0.0, 3.6), 1.2));
    r = U(r, tree(p - vec3(-3.2, 0.0, 3.3), 1.0));
  }
  if (near(p - vec3(3.5, 0.4, -3.5), vec3(0.6, 0.5, 0.6), r.x)) {
    r = U(r, tree(p - vec3(3.3, 0.0, -3.3), 1.1));
    r = U(r, pine(p - vec3(3.75, 0.0, -3.6), 1.1));
  }
  if (near(p - vec3(1.2, 0.4, -3.0), vec3(0.2, 0.5, 0.2), r.x)) r = U(r, filebox(p - vec3(1.2, 0.0, -3.0), LIME));
  if (near(p - vec3(-1.3, 0.1, 2.05), vec3(0.6, 0.2, 0.2), r.x)) {
    r = U(r, filebox(p - vec3(-1.75, 0.0, 2.05), SUN));
    r = U(r, filebox(p - vec3(-1.5, 0.0, 2.08), BLUE));
    r = U(r, filebox(p - vec3(-1.62, 0.14, 2.06), LILAC));
  }
  vec3 dq = p - vec3(-2.25, 1.45, 0.15);
  if (near(dq - vec3(0.0, -0.15, 0.0), vec3(0.4, 0.4, 0.4), r.x)) { vec2 dr = drone(dq / 1.3); r = U(r, M(dr.x * 1.3, dr.y)); }
  if (near(p - vec3(4.0, 0.4, -2.0), vec3(0.75, 0.5, 0.45), r.x)) { r = U(r, tree(p - vec3(4.45, 0.0, -1.85), 1.0)); r = U(r, pine(p - vec3(3.55, 0.0, -2.1), 0.9)); }
  if (near(p - vec3(-3.85, 0.4, 2.0), vec3(0.3, 0.5, 0.3), r.x)) r = U(r, tree(p - vec3(-3.85, 0.0, 2.0), 0.95));
  if (near(p - vec3(2.1, 0.4, -3.0), vec3(0.3, 0.5, 0.3), r.x)) r = U(r, pine(p - vec3(2.1, 0.0, -3.0), 1.0));
  if (near(p - vec3(-2.05, 0.4, -0.6), vec3(0.3, 0.5, 0.3), r.x)) r = U(r, tree(p - vec3(-2.05, 0.0, -0.6), 0.9));
  vec3 fq = p - vec3(1.42, -0.04, 1.25);
  if (near(fq - vec3(0.0, 0.1, 0.0), vec3(0.15, 0.15, 0.15), r.x)) {
    r = U(r, M(max(sdCylinder(fq - vec3(0.0, 0.04, 0.0), 0.11, 0.04), -sdCylinder(fq - vec3(0.0, 0.08, 0.0), 0.085, 0.02)), WHITE + SOLID));
    r = U(r, M(sdCylinder(fq - vec3(0.0, 0.1, 0.0), 0.02, 0.06), WHITE + SOLID));
    r = U(r, M(sdSphere(fq - vec3(0.0, 0.19, 0.0), 0.045), SKY + SOLID));
  }
  if (max(plinth2(p) - 0.08, max(p.y - 0.01, -p.y - 0.35)) < r.x) r = U(r, base(p));
  return r;
}
