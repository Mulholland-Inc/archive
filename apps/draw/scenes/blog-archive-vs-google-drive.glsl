#pragma frame {"size":[1400,760],"center":[0.0,0.55,0.0],"zoom":175,"bounds":[[-3.0,-0.1,-3.0],[3.0,2.8,3.0]]}

float bar(vec2 p, float x0, float y, float len) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, 0.02), 0.02);
}

vec2 plinth(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.09, 0.0), vec3(1.2, 0.09, 1.2), 0.05), WHITE);
  r = seam(r, p.y - 0.13);
  r = paint(r, p.y - 0.05, STEEL);
  r = vents(r, faceX(p, 1.2) - vec3(-0.55, 0.0, 0.095), 6.0, 0.07, 0.045, 0.028, GRAPHITE);
  r = vents(r, faceZ(p, 1.2) - vec3(0.55, 0.0, 0.095), 6.0, 0.07, 0.045, 0.028, GRAPHITE);
  vec3 lz = faceZ(p, 1.2) - vec3(-0.8, 0.0, 0.095);
  r = U(r, led(vec3(repC(lz.x, 0.09, 3.0), lz.y, lz.z), 0.022, LIME));
  r = U(r, screw(faceY(mirXZ(p), 0.18) - vec3(1.09, 0.0, 1.09), 0.026));
  return r;
}

vec2 handle(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.05, 0.0), vec3(0.13, 0.016, 0.022), 0.012), STEEL);
  vec3 s = vec3(abs(q.x) - 0.11, q.y - 0.025, q.z);
  r = U(r, M(sdBox(s, vec3(0.014, 0.025, 0.014)), STEEL));
  return r;
}

vec2 label(vec3 q) {
  vec2 r = M(extrude(max(sdRect(q.xz, vec2(0.1, 0.045), 0.008), -sdRect(q.xz, vec2(0.08, 0.028), 0.004)), q.y - 0.006, 0.006), STEEL);
  return r;
}

vec2 folder(vec3 p, float m, float tab) {
  p.y -= 0.08;
  vec2 f = M(sdBox(p, vec3(0.32, 0.15, 0.012)), m);
  f = U(f, M(sdRoundBox(p - vec3(tab, 0.17, 0.0), vec3(0.07, 0.035, 0.012), 0.01), m));
  return f;
}

vec2 drawer(vec3 p) {
  vec3 b = p - vec3(0.0, 0.0, -0.1);
  vec2 tray = M(max(sdBox(b, vec3(0.37, 0.18, 0.5)), -sdBox(b - vec3(0.0, 0.05, 0.0), vec3(0.34, 0.2, 0.47))), WHITE);
  vec3 fp = p - vec3(0.0, 0.0, 0.42);
  vec2 face = M(sdRoundBox(fp, vec3(0.41, 0.21, 0.025), 0.012), WHITE);
  vec3 fq = faceZ(fp, 0.025);
  face = inset(face, fq, vec2(0.36, 0.16), 0.02, 0.012, PAPER);
  vec2 r = U(tray, face);
  r = U(r, handle(fq - vec3(0.0, 0.0, -0.04)));
  r = U(r, label(fq - vec3(0.0, 0.0, 0.1)));
  r = U(r, folder(p - vec3(0.0, 0.0, 0.33), CORAL, 0.18));
  r = U(r, folder(p - vec3(0.0, 0.0, 0.25), WHITE, -0.06));
  r = U(r, folder(p - vec3(0.0, 0.0, 0.17), MINT, 0.12));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.01), BLUE, -0.2));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.09), WHITE, 0.2));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.17), LILAC, 0.0));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.25), PINK, -0.15));
  return r;
}

vec2 cabinet(vec3 p) {
  vec3 b = p - vec3(0.0, 0.8, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.45, 0.72, 0.42), 0.02), WHITE);
  vec3 fz = faceZ(p, 0.42);
  r = Sc(r, M(max(sdRect(fz.xz - vec2(0.0, 1.26), vec2(0.41, 0.21), 0.012), -(fz.y + 1.2)), GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.32, 0.43), vec3(0.41, 0.21, 0.02), 0.012), WHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.79, 0.43), vec3(0.41, 0.21, 0.02), 0.012), WHITE));
  vec3 f1 = faceZ(p, 0.45);
  r = inset(r, f1 - vec3(0.0, 0.0, 0.32), vec2(0.36, 0.16), 0.02, 0.012, PAPER);
  r = inset(r, f1 - vec3(0.0, 0.0, 0.79), vec2(0.36, 0.16), 0.02, 0.012, PAPER);
  r = U(r, handle(f1 - vec3(0.0, 0.0, 0.28)));
  r = U(r, label(f1 - vec3(0.0, 0.0, 0.42)));
  r = U(r, handle(f1 - vec3(0.0, 0.0, 0.75)));
  r = U(r, label(f1 - vec3(0.0, 0.0, 0.89)));
  vec3 kick = p - vec3(0.0, 0.04, 0.0);
  r = U(r, M(sdBox(kick, vec3(0.41, 0.04, 0.37)), GRAPHITE));
  vec3 fx = faceX(p, 0.45);
  r = inset(r, fx - vec3(0.0, 0.0, 0.86), vec2(0.3, 0.56), 0.03, 0.012, WHITE);
  r = vents(r, vec3(fx.x, fx.y, repC(fx.z - 0.42, 0.06, 3.0)), 5.0, 0.07, 0.12, 0.028, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.24, fx.y, abs(fx.z - 0.86) - 0.48), 0.022));
  r = seam(r, -(abs(b.y) - 0.68));
  vec3 t = faceY(p, 1.52);
  r = U(r, screw(mirXZ(t) - vec3(0.38, 0.0, 0.35), 0.024));
  return r;
}

vec2 sheet(vec3 p, float m) {
  vec2 s = M(sdBox(p, vec3(0.26, 0.14, 0.004)), m);
  vec3 q = faceZ(p, 0.004);
  s = decal(s, q, sdRect(q.xz - vec2(-0.06, 0.08), vec2(0.13, 0.016), 0.016), STEEL);
  s = decal(s, q, sdRect(q.xz - vec2(0.0, 0.03), vec2(0.19, 0.012), 0.012), STEEL);
  return s;
}

vec2 lifted(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.0, -0.03), vec3(0.32, 0.15, 0.012)), SUN);
  r = U(r, M(sdBox(rotX(p - vec3(0.0, -0.15, 0.0), 0.25) - vec3(0.0, 0.15, 0.012), vec3(0.32, 0.15, 0.012)), SUN));
  r = U(r, M(sdRoundBox(p - vec3(0.18, 0.17, -0.03), vec3(0.07, 0.035, 0.012), 0.01), SUN));
  r = U(r, sheet(rotX(p - vec3(0.0, 0.07, 0.0), 0.04), WHITE));
  r = U(r, sheet(rotZ(rotX(p - vec3(0.03, 0.1, 0.02), 0.12), -0.06), WHITE));
  return r;
}

vec2 lens(vec3 q) {
  vec2 r = M(sdTorus(q, 0.28, 0.034), WHITE + SOLID);
  r = U(r, M(max(sdTorus(q, 0.28, 0.046), abs(q.y) - 0.018), GRAPHITE + SOLID));
  vec3 g = q - vec3(0.0, 0.01, 0.0);
  vec3 h = q - vec3(0.32, 0.0, 0.0);
  r = U(r, M(sdCylinder(rotZ(h - vec3(0.04, 0.0, 0.0), PI * 0.5), 0.05, 0.04), STEEL + ROUND));
  r = U(r, M(sdCylinder(rotZ(h - vec3(0.1, 0.0, 0.0), PI * 0.5), 0.065, 0.02), STEEL + ROUND));
  r = U(r, M(sdRoundCylinder(rotZ(h - vec3(0.36, 0.0, 0.0), PI * 0.5), 0.07, 0.24, 0.05), GRAPHITE + ROUND));
  r = U(r, M(sdTorus(rotZ(h - vec3(0.25, 0.0, 0.0), PI * 0.5), 0.07, 0.012), STEEL + SOLID));
  return r;
}

vec2 docTile(vec3 p, float kind) {
  vec2 h = kind == 2.0 ? vec2(0.4, 0.28) : vec2(0.3, 0.4);
  vec3 b = p - vec3(0.0, h.y + 0.08, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(h.x, h.y, 0.03), 0.015), WHITE);
  vec3 f = faceZ(b, 0.03);
  vec2 s = vec2(f.x, -f.z);
  if (kind == 0.0) {
    r = decal(r, f, sdRect(f.xz - vec2(-0.08, 0.28), vec2(0.14, 0.035), 0.035), GRAPHITE);
    r = decal(r, f, bar(f.xz, -0.22, 0.16, 0.44), STEEL);
    r = decal(r, f, bar(f.xz, -0.22, 0.08, 0.38), STEEL);
    r = decal(r, f, bar(f.xz, -0.22, 0.0, 0.42), STEEL);
    r = decal(r, f, bar(f.xz, -0.22, -0.08, 0.26), STEEL);
    r = decal(r, f, sdRect(f.xz - vec2(0.0, -0.16), vec2(0.23, 0.032), 0.01), LILAC);
    r = decal(r, f, bar(f.xz, -0.22, -0.26, 0.4), STEEL);
  } else if (kind == 1.0) {
    vec2 c = vec2(repC(f.x, 0.14, 4.0), repC(f.z + 0.02, 0.1, 6.0));
    vec2 ci = vec2(repI(f.x, 0.14, 4.0), repI(f.z + 0.02, 0.1, 6.0));
    float cell = sdRect(c, vec2(0.06, 0.038), 0.006);
    float m = ci.y == 5.0 ? GRAPHITE : ci.x == 0.0 ? STEEL : (ci.x == 2.0 && ci.y == 2.0) ? LIME : PAPER;
    r = decal(r, f, cell, m);
  } else {
    r = decal(r, f, sdRect(f.xz - vec2(-0.12, 0.17), vec2(0.2, 0.035), 0.035), GRAPHITE);
    float x = f.x + 0.18;
    float i = repLI(x, 0.1, 4.0);
    float hgt = 0.05 + 0.04 * i;
    r = decal(r, f, sdRect(vec2(repL(x, 0.1, 4.0), f.z + 0.18 - hgt), vec2(0.03, hgt), 0.006), i == 3.0 ? LILAC : STEEL);
    r = decal(r, f, sdCircle(f.xz - vec2(0.26, -0.04), 0.08), STEEL);
  }
  vec3 ft = vec3(abs(p.x) - h.x + 0.08, p.y - 0.04, p.z);
  r = U(r, M(sdRoundBox(ft, vec3(0.05, 0.04, 0.1), 0.012), STEEL));
  return r;
}

vec2 standFolder(vec3 p, float m, float tab) {
  vec2 f = M(sdBox(p - vec3(0.0, 0.2, 0.0), vec3(0.4, 0.2, 0.012)), m);
  f = U(f, M(sdRoundBox(p - vec3(tab, 0.42, 0.0), vec3(0.09, 0.035, 0.012), 0.01), m));
  return f;
}

vec2 crate(vec3 p) {
  vec3 b = p - vec3(0.0, 0.2, 0.0);
  vec2 r = M(max(sdRoundBox(b, vec3(0.5, 0.2, 0.5), 0.025), -sdBox(b - vec3(0.0, 0.05, 0.0), vec3(0.46, 0.2, 0.46))), WHITE);
  vec3 fz = faceZ(b, 0.5);
  r = inset(r, fz - vec3(0.0, 0.0, 0.02), vec2(0.4, 0.12), 0.02, 0.01, PAPER);
  r = U(r, M(sdRoundBox(fz - vec3(0.0, 0.015, 0.05), vec3(0.12, 0.015, 0.035), 0.01), STEEL));
  vec3 fx = faceX(b, 0.5);
  r = vents(r, fx - vec3(0.0, 0.0, 0.0), 6.0, 0.08, 0.16, 0.035, GRAPHITE);
  float z = p.z + 0.36;
  float i = repLI(z, 0.12, 7.0);
  float m = i == 1.0 ? SUN : i == 3.0 ? CORAL : i == 5.0 ? SUN : WHITE;
  float tab = mod(i, 3.0) * 0.2 - 0.2;
  r = U(r, standFolder(vec3(p.x, p.y - 0.08 - (i == 4.0 ? 0.08 : 0.0), repL(z, 0.12, 7.0)), m, tab));
  return r;
}

vec2 loose(vec3 p, float m) {
  vec2 f = M(sdRoundBox(p, vec3(0.32, 0.012, 0.22), 0.006), m);
  f = U(f, M(sdRoundBox(p - vec3(-0.15, 0.0, -0.25), vec3(0.09, 0.012, 0.035), 0.006), m));
  return f;
}

vec2 op(vec3 p, float m) {
  vec2 r = M(sdRoundBox(p, vec3(0.1, 0.07, 0.03), 0.012), m);
  vec3 f = faceZ(p, 0.03);
  r = decal(r, f, bar(f.xz, -0.06, 0.02, 0.12), STEEL);
  r = decal(r, f, bar(f.xz, -0.06, -0.025, 0.08), STEEL);
  return r;
}

vec2 tileK(vec3 p, float kind, float k) {
  vec2 t = docTile(p / k, kind);
  return vec2(t.x * k, t.y);
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 a = p - vec3(-0.9, 0.0, 0.9);
  r = U(r, crate(a));
  r = U(r, loose(rotY(a - vec3(0.15, 0.012, 0.85), 0.25), WHITE));
  r = U(r, loose(rotY(a - vec3(0.2, 0.036, 0.88), -0.1), SUN));
  r = U(r, M(min(pipe(p, vec3(-0.38, 0.12, 0.9), vec3(0.55, 0.12, 0.9), 0.04), pipe(p, vec3(0.55, 0.12, 0.9), vec3(0.55, 0.12, -0.15), 0.04)), LILAC + SOLID));
  r = U(r, M(sdRoundBox(p - vec3(-0.38, 0.12, 0.9), vec3(0.04, 0.07, 0.07), 0.012), STEEL));
  r = U(r, M(sdRoundBox(p - vec3(0.55, 0.12, -0.18), vec3(0.07, 0.07, 0.04), 0.012), STEEL));
  r = U(r, op(p - vec3(0.08, 0.23, 0.9), WHITE));
  r = U(r, op(rotY(p - vec3(0.55, 0.23, 0.35), PI * 0.5), WHITE));
  vec3 b = p - vec3(1.2, 0.0, -1.2);
  if (near(b - vec3(0.0, 1.0, 0.0), vec3(1.3, 1.2, 1.3), r.x)) {
    vec2 pl = plinth(b / 0.85);
    r = U(r, vec2(pl.x * 0.85, pl.y));
    vec3 g = b - vec3(0.0, 0.153, 0.0);
    float k = 0.72;
    vec3 cq = (g - vec3(-0.38, 0.0, -0.45)) / k;
    vec2 cb = cabinet(cq);
    cb = U(cb, drawer(cq - vec3(0.0, 1.26, 0.6)));
    cb = U(cb, lifted(rotY(rotZ(cq - vec3(0.05, 1.84, 0.69), 0.1), 0.15)));
    r = U(r, vec2(cb.x * k, cb.y));
    r = U(r, tileK(g - vec3(-0.52, 0.0, 0.58), 0.0, 1.25));
    r = U(r, tileK(g - vec3(0.28, 0.0, 0.42), 1.0, 1.25));
    r = U(r, tileK(g - vec3(0.45, 0.0, -0.5), 2.0, 1.25));
    vec3 lq = rotY(rotX(g - vec3(0.48, 0.72, 0.74), PI * 0.5), 0.5) / 1.2;
    vec2 ln = lens(lq);
    r = U(r, vec2(ln.x * 1.2, ln.y));
  }
  return r;
}
