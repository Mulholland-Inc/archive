#pragma frame {"size":[2000,580],"center":[-0.45,0.46,-0.45],"zoom":118,"bounds":[[-12.0,-0.4,-12.0],[12.0,3.2,12.0]]}

#define R2 0.70710678

vec2 toUV(vec3 p) { return vec2(p.x - p.z, p.x + p.z) * R2; }
vec3 W(float u, float v) { return vec3((u + v) * R2, 0.0, (v - u) * R2); }

vec2 envelope(vec3 p, float m) {
  vec2 e = M(sdBox(p, vec3(0.2, 0.018, 0.14)), m);
  return seam(e, p.z + 0.14 - (0.2 - abs(p.x)) * 0.62);
}

vec2 stack(vec3 p, vec2 h, float n) {
  vec2 s = M(sdBox(p - vec3(0.0, n * 0.5, 0.0), vec3(h.x, n * 0.5, h.y)), WHITE);
  return stripes(s, p.y, 0.03);
}

vec2 tube(vec3 p, vec3 a, vec3 b) {
  vec3 ab = b - a;
  float t = clamp(dot(p - a, ab) / dot(ab, ab), 0.0, 1.0);
  float l = t * length(ab);
  float d = length(p - a - ab * t);
  vec2 g = M(d - 0.11, SKY + SOLID);
  float c = abs(mod(l + 0.3, 0.9) - 0.45) - 0.03;
  g = U(g, M(max(d - 0.14, max(c, max(0.05 - l, l - length(ab) + 0.05))), STEEL + SOLID));
  return g;
}

vec2 elbow(vec3 p, vec3 c) { return M(sdSphere(p - c, 0.15), STEEL + SOLID); }

vec2 intake(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.55, 0.0), vec3(0.95, 0.55, 0.8)), WHITE);
  r = paint(r, p.y - 0.1, STEEL);
  r = U(r, M(sdBox(p - vec3(0.0, 1.12, 0.0), vec3(0.98, 0.04, 0.83)), WHITE));
  vec3 s = faceZ(p, 0.8) - vec3(0.0, 0.0, 0.62);
  r = Sc(r, M(max(sdRect(s.xz, vec2(0.52, 0.09), 0.0), -(s.y + 0.25)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(s.xz, vec2(0.62, 0.18), 0.06), -sdRect(s.xz, vec2(0.52, 0.09), 0.0)), s.y - 0.04, 0.04), LIME));
  r = U(r, envelope(rotX(p - vec3(-0.12, 0.63, 0.92), 0.1), WHITE));
  vec3 h = p - vec3(-0.3, 1.16, 0.15);
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.1, 0.0), 0.16, 0.1), STEEL + ROUND));
  vec3 cq = h - vec3(0.0, 0.5, 0.0);
  float outer = sdCone(cq, 0.3, 0.17, 0.6);
  float inner = sdCone(cq - vec3(0.0, 0.04, 0.0), 0.3, 0.13, 0.56);
  r = U(r, M(max(outer, -inner), WHITE + SOLID));
  r = U(r, M(sdTorus(cq - vec3(0.0, 0.3, 0.0), 0.6, 0.03), STEEL + SOLID));
  vec3 th = p - vec3(0.55, 1.16, -0.4);
  r = U(r, M(sdBox(th - vec3(0.0, 0.16, 0.0), vec3(0.2, 0.16, 0.2)), WHITE));
  return r;
}

vec2 machine(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.42, 0.0), vec3(1.05, 0.4, 0.65)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(1.02, 0.02, 0.62)), GRAPHITE));
  vec3 fz = faceZ(p, 0.65);
  vec3 dq = vec3(repC(fz.x, 1.02, 2.0), fz.y, fz.z - 0.42);
  r = U(r, M(extrude(sdRect(dq.xz, vec2(0.47, 0.3), 0.03), dq.y - 0.014, 0.014), WHITE));
  vec3 dd = dq - vec3(0.0, 0.028, 0.0);
  r = Sc(r, M(max(sdRect(dd.xz - vec2(0.0, 0.17), vec2(0.13, 0.03), 0.03), -(dd.y + 0.02)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.98, 0.0), vec3(1.0, 0.16, 0.6)), WHITE));
  vec3 ex = faceX(p, 1.0);
  r = Sc(r, M(max(sdRect(ex.xz - vec2(0.0, 0.98), vec2(0.45, 0.08), 0.02), -(ex.y + 0.3)), GRAPHITE));
  vec3 ot = p - vec3(1.2, 0.95, 0.0);
  r = U(r, M(sdBox(rotZ(ot, 0.1), vec3(0.24, 0.016, 0.4)), STEEL));
  r = U(r, stack(rotZ(ot - vec3(0.02, 0.016, 0.0), 0.1), vec2(0.16, 0.22), 0.1));
  vec3 sb = p - vec3(0.0, 1.2, 0.0);
  vec2 bed = M(sdBox(sb, vec3(0.62, 0.06, 0.56)), WHITE);
  bed = decal(bed, faceY(sb, 0.06), sdRect(sb.xz - vec2(0.0, 0.04), vec2(0.5, 0.42), 0.02), SKY);
  r = U(r, bed);
  r = U(r, M(sdBox(sb - vec3(-0.15, 0.085, 0.04), vec3(0.04, 0.028, 0.42)), LIME + SOLID));
  vec3 lq = rotX(sb - vec3(0.0, 0.06, -0.56), -0.3);
  r = U(r, M(sdBox(lq - vec3(0.0, 0.04, 0.55), vec3(0.62, 0.04, 0.55)), WHITE));
  vec3 ib = vec3(abs(p.x) - 0.84, p.y - 1.14, p.z);
  r = U(r, M(sdBox(ib - vec3(0.0, 0.15, 0.0), vec3(0.15, 0.15, 0.15)), WHITE));
  r = U(r, M(sdBox(ib - vec3(0.0, 0.31, 0.0), vec3(0.18, 0.02, 0.18)), WHITE));
  return r;
}

vec2 drawerOut(vec3 d, vec2 h, float ext, float m) {
  vec2 r = M(sdBox(d - vec3(0.0, ext * 0.5, -h.y * 0.2), vec3(h.x * 0.92, ext * 0.5, h.y * 0.72)), WHITE);
  vec3 e = d - vec3(0.0, ext, 0.0);
  r = U(r, M(extrude(sdRect(e.xz, h, 0.025), e.y - 0.015, 0.015), WHITE));
  r = U(r, M(sdBox(e - vec3(0.0, 0.04, 0.02), vec3(h.x * 0.42, 0.012, 0.025)), STEEL));
  for (int i = 0; i < 3; i++) {
    float fi = float(i);
    vec3 fo = d - vec3(0.0, ext * (0.25 + 0.27 * fi), h.y * 0.55);
    float fm = i == 1 ? m : WHITE;
    r = U(r, M(sdBox(fo, vec3(h.x * 0.82, 0.022, h.y * 0.38)), fm));
    r = U(r, M(sdBox(fo - vec3(-h.x * 0.45 + h.x * 0.42 * fi, 0.0, h.y * 0.45), vec3(h.x * 0.22, 0.022, h.y * 0.1)), fm));
  }
  return r;
}

vec2 cabinet(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 1.05, 0.0), vec3(0.95, 1.01, 0.8)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.92, 0.02, 0.77)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 2.1, 0.0), vec3(1.0, 0.05, 0.85)), WHITE));
  vec3 fz = faceZ(p, 0.8);
  vec3 dz = vec3(fz.x, fz.y, fz.z - 1.05);
  vec2 cz = vec2(repI(dz.x, 0.6, 3.0), repI(dz.z, 0.48, 4.0));
  vec3 gz = vec3(repC(dz.x, 0.6, 3.0), dz.y, repC(dz.z, 0.48, 4.0));
  bool openZ = cz == vec2(2.0, 1.0);
  vec3 oz = vec3(dz.x - 0.6, dz.y, dz.z + 0.24);
  if (!openZ) {
    r = U(r, M(extrude(sdRect(gz.xz, vec2(0.26, 0.2), 0.025), gz.y - 0.015, 0.015), WHITE));
    r = U(r, M(sdBox(gz - vec3(0.0, 0.04, 0.02), vec3(0.11, 0.012, 0.025)), STEEL));
  }
  r = Sc(r, M(max(sdRect(oz.xz, vec2(0.26, 0.2), 0.0), -(oz.y + 0.4)), GRAPHITE));
  r = U(r, drawerOut(oz, vec2(0.26, 0.2), 0.7, LIME));
  vec3 fx = faceX(p, 0.95);
  vec3 dx = vec3(fx.x, fx.y, fx.z - 1.05);
  vec2 cx = vec2(repI(dx.x, 0.76, 2.0), repI(dx.z, 0.48, 4.0));
  vec3 gx = vec3(repC(dx.x, 0.76, 2.0), dx.y, repC(dx.z, 0.48, 4.0));
  bool openX = cx == vec2(0.0, 2.0);
  vec3 ox = vec3(dx.x + 0.38, dx.y, dx.z - 0.24);
  if (!openX) {
    r = U(r, M(extrude(sdRect(gx.xz, vec2(0.33, 0.2), 0.025), gx.y - 0.015, 0.015), WHITE));
    r = U(r, M(sdBox(gx - vec3(0.0, 0.04, 0.02), vec3(0.13, 0.012, 0.025)), STEEL));
  }
  r = Sc(r, M(max(sdRect(ox.xz, vec2(0.33, 0.2), 0.0), -(ox.y + 0.4)), GRAPHITE));
  r = U(r, drawerOut(ox, vec2(0.33, 0.2), 0.5, SKY));
  return r;
}

vec2 tree(vec3 p, float s) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.25 * s, 0.0), 0.03 * s, 0.25 * s), GRAPHITE + SOLID);
  r = U(r, M(sdEllipsoid(p - vec3(0.0, 0.62 * s, 0.0), vec3(0.26, 0.32, 0.26) * s), MINT + SOLID));
  return r;
}

vec2 planter(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.1, 0.0), vec3(0.45, 0.1, 0.2)), WHITE);
  vec3 s = vec3(repC(p.x, 0.3, 3.0), p.y - 0.22, p.z);
  r = U(r, M(sdEllipsoid(s, vec3(0.15, 0.12, 0.15)), MINT + SOLID));
  return r;
}

vec2 ground(vec3 p, vec2 q) {
  vec2 r = M(max(max(p.y, -p.y - 0.3), -(q.y + 2.9)), MINT);
  return decal(r, faceY(p, 0.0), abs(q.y - 1.35) - 0.22, WHITE);
}

vec2 map(vec3 p) {
  vec2 q = toUV(p);
  vec2 r = p.y < 0.2 ? ground(p, q) : M(p.y - 0.1, MINT);
  vec3 a = W(-5.6, -0.5), m = W(0.0, -0.6), f = W(5.6, -0.6);
  if (near(p - a - vec3(0.0, 0.9, 0.0), vec3(1.05, 0.95, 0.9), r.x)) r = U(r, intake(p - a));
  if (near(p - m - vec3(0.1, 1.0, 0.0), vec3(1.75, 1.05, 1.0), r.x)) { vec2 mm = machine((p - m) / 1.25); r = U(r, M(mm.x * 1.25, mm.y)); }
  if (near(p - f - vec3(0.0, 1.1, 0.0), vec3(1.15, 1.15, 1.2), r.x)) r = U(r, cabinet(p - f));
  float ty = 1.9;
  vec3 a0 = a + vec3(0.55, 1.48, -0.4), a1 = vec3(a0.x, ty, a0.z);
  vec3 b0 = m + vec3(-0.84, 1.47, 0.0) * 1.25, b1 = vec3(b0.x, ty, b0.z);
  vec3 c0 = m + vec3(0.84, 1.47, 0.0) * 1.25, c1 = vec3(c0.x, ty, c0.z);
  vec3 d1 = f + vec3(-0.9, ty, 0.0);
  if (abs(p.y - 1.8) - 0.45 < r.x) {
    r = U(r, tube(p, a0, a1));
    r = U(r, tube(p, a1, b1));
    r = U(r, tube(p, b1, b0));
    r = U(r, tube(p, c0, c1));
    r = U(r, tube(p, c1, d1));
    r = U(r, elbow(p, a1));
    r = U(r, elbow(p, b1));
    r = U(r, elbow(p, c1));
  }
  vec3 t = W(-3.0, 0.55);
  if (near(p - t - vec3(0.0, 0.55, 0.0), vec3(0.32, 0.6, 0.32), r.x)) r = U(r, tree(p - t, 1.15));
  vec3 pl = W(2.9, 0.75);
  if (near(p - pl - vec3(0.0, 0.18, 0.0), vec3(0.5, 0.22, 0.25), r.x)) r = U(r, planter(p - pl));
  return r;
}
