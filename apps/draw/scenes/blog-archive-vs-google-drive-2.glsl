#pragma frame {"size":[1200,720],"center":[0.0,0.6,0.0],"zoom":230,"bounds":[[-2.4,-0.1,-2.4],[2.4,2.6,2.4]]}

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

vec2 arm(vec3 p, vec3 w, out vec3 grip) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.04, 0.0), 0.16, 0.04, 0.012), GRAPHITE + ROUND);
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.12, 0.0), 0.11, 0.05, 0.015), WHITE + ROUND));
  vec3 a = vec3(0.0, 0.2, 0.0);
  vec3 e = vec3(0.12, 0.66, -0.06);
  r = U(r, M(sdSphere(p - a, 0.085), STEEL + SOLID));
  r = U(r, M(pipe(p, a, e, 0.06), WHITE + SOLID));
  r = U(r, M(sdSphere(p - e, 0.07), LIME + SOLID));
  r = U(r, M(pipe(p, e, w, 0.045), WHITE + SOLID));
  r = U(r, M(sdSphere(p - w, 0.05), STEEL + SOLID));
  vec3 g = p - w - vec3(0.0, -0.07, 0.0);
  r = U(r, M(sdRoundBox(g, vec3(0.06, 0.025, 0.06), 0.01), GRAPHITE));
  vec3 fn = vec3(g.x, g.y + 0.07, abs(g.z) - 0.05);
  r = U(r, M(sdBox(fn, vec3(0.03, 0.06, 0.008)), GRAPHITE));
  grip = w - vec3(0.0, 0.16, 0.0);
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

vec2 tileK(vec3 p, float kind, float k) {
  vec2 t = docTile(p / k, kind);
  return vec2(t.x * k, t.y);
}

vec2 strip(vec3 p, float m) {
  vec2 r = M(sdRoundBox(p, vec3(0.34, 0.018, 0.035), 0.012), m);
  return r;
}

vec2 doc(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.02, 0.0), vec3(0.62, 0.02, 0.8), 0.01), WHITE);
  vec3 t = faceY(p, 0.04);
  vec2 s = t.xz;
  r = decal(r, t, sdRect(s - vec2(-0.2, -0.64), vec2(0.3, 0.045), 0.045), GRAPHITE);
  r = decal(r, t, bar(s, -0.5, -0.5, 0.96), STEEL);
  r = decal(r, t, bar(s, -0.5, -0.42, 0.88), STEEL);
  r = inset(r, t - vec3(-0.12, 0.0, -0.32), vec2(0.36, 0.045), 0.02, 0.02, STEEL);
  r = decal(r, t, bar(s, -0.5, -0.22, 0.7), STEEL);
  vec3 tb = t - vec3(0.0, 0.0, 0.04);
  vec2 c = vec2(repC(tb.x, 0.32, 3.0), repC(tb.z, 0.1, 3.0));
  vec2 ci = vec2(repI(tb.x, 0.32, 3.0), repI(tb.z, 0.1, 3.0));
  r = decal(r, tb, max(sdRect(c, vec2(0.15, 0.043), 0.006), sdRect(tb.xz, vec2(0.48, 0.15), 0.0)), ci.y == 0.0 ? SKY : PAPER);
  r = decal(r, t, bar(s, -0.5, 0.28, 0.92), STEEL);
  r = decal(r, t, bar(s, -0.5, 0.36, 0.8), STEEL);
  vec3 im = t - vec3(0.24, 0.0, 0.58);
  float fr = sdRect(im.xz, vec2(0.22, 0.14), 0.02);
  r = inset(r, im, vec2(0.22, 0.14), 0.02, 0.01, SKY);
  vec3 fl = im + vec3(0.0, 0.01, 0.0);
  r = decal(r, fl, max(fr, min(sdCircle(fl.xz - vec2(0.1, 0.3), 0.24), sdCircle(fl.xz - vec2(-0.14, 0.34), 0.26))), LILAC);
  r = decal(r, fl, sdCircle(fl.xz - vec2(-0.12, -0.05), 0.03), WHITE);
  r = decal(r, t, bar(s, -0.5, 0.48, 0.38), STEEL);
  r = decal(r, t, bar(s, -0.5, 0.56, 0.42), STEEL);
  r = decal(r, t, bar(s, -0.5, 0.64, 0.3), STEEL);
  return r;
}

vec2 map(vec3 p) {
  vec2 pl = plinth(p / 0.9);
  vec2 r = vec2(pl.x * 0.9, pl.y);
  vec3 g = p - vec3(0.0, 0.162, 0.0);
  r = U(r, M(sdBox(g - vec3(0.05, 0.012, 0.05), vec3(0.62, 0.012, 0.8)), PAPER));
  r = U(r, doc(g - vec3(0.15, 0.024, 0.2)));
  r = U(r, tileK(g - vec3(-0.55, 0.0, -0.75), 1.0, 1.0));
  r = U(r, tileK(g - vec3(0.4, 0.0, -0.78), 2.0, 1.0));
  vec3 grip;
  float k = 1.3;
  vec3 ab = (g - vec3(-0.75, 0.0, 0.15)) / k;
  vec2 ar = arm(ab, vec3(0.6, 0.42, -0.21), grip);
  r = U(r, vec2(ar.x * k, ar.y));
  vec3 gw = vec3(-0.75, 0.0, 0.15) + grip * k;
  r = U(r, strip(g - gw - vec3(0.0, 0.04, 0.0), LIME));
  r = U(r, strip(rotY(g - vec3(-0.8, 0.018, 0.72), PI * 0.5 + 0.12), STEEL));
  return r;
}
