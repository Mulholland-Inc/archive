#pragma frame {"size":[1100,900],"center":[0.0,0.45,0.0],"zoom":200,"bounds":[[-2.2,-0.3,-2.2],[2.2,2.1,2.2]]}

const float SP = 1.15;
const float PH = 0.42;
const float K = 1.5;
const float RY = 0.1;

vec3 view(vec3 d) {
  return vec3(dot(d.xz, vec2(0.70710678, -0.70710678)), d.y, dot(d.xz, vec2(0.70710678, 0.70710678)));
}

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

vec2 base(vec3 p) {
  vec3 b = p - vec3(0.0, -0.09, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(1.78, 0.09, 1.78), 0.035), WHITE);
  r = seam(r, b.y + 0.03);
  r = paint(r, b.y + 0.03, STEEL);
  vec2 g = abs(abs(p.xz) - SP * 0.5);
  if (max(abs(p.x), abs(p.z)) < 1.7) r = groove(r, min(g.x, g.y), 0.012, 0.02);
  return r;
}

vec2 plinth(vec3 q, float k) {
  vec3 c = q - vec3(0.0, 0.1, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(PH, 0.1, PH), 0.03), WHITE);
  vec3 fz = faceZ(c, PH);
  float d = bar(fz.xz, -0.32, 0.03, 0.3 + 0.1 * mod(k, 2.0), 0.022);
  d = min(d, bar(fz.xz, -0.32, -0.04, 0.16 + 0.08 * mod(k + 1.0, 3.0), 0.022));
  r = decal(r, fz, d, STEEL);
  vec3 fx = faceX(c, PH);
  r = decal(r, fx, bar(fx.xz, -0.32, 0.0, 0.2 + 0.08 * mod(k, 3.0), 0.022), STEEL);
  return r;
}

vec2 house(vec3 q) {
  vec3 b = q - vec3(0.0, 0.15, 0.0);
  vec2 r = M(sdBox(b, vec3(0.22, 0.15, 0.17)), WHITE);
  vec3 g = q - vec3(0.0, 0.3, 0.0);
  vec3 gx = vec3(g.z, g.y, g.x);
  r = U(r, M(sdPrism(vec3(gx.x, gx.y, gx.z), 0.17, 0.16, 0.22), WHITE));
  vec3 rf = vec3(q.z, q.y - 0.28, q.x);
  float outer = sdPrism(rf, 0.24, 0.225, 0.27);
  float inner = sdPrism(rf + vec3(0.0, 0.05, 0.0), 0.24, 0.225, 0.4);
  r = U(r, M(max(outer, -inner), BLUE));
  vec3 fx = faceX(b, 0.22);
  r = inset(r, fx - vec3(0.02, 0.0, -0.04), vec2(0.06, 0.11), 0.01, 0.03, GRAPHITE);
  vec3 fz = faceZ(b, 0.17);
  r = inset(r, fz - vec3(0.0, 0.0, 0.02), vec2(0.07, 0.06), 0.01, 0.03, SKY);
  r = U(r, M(sdBox(q - vec3(-0.1, 0.5, -0.08), vec3(0.04, 0.09, 0.04)), WHITE));
  return r;
}

vec2 tower(vec3 q) {
  vec3 b = q - vec3(0.0, 0.42, 0.0);
  vec2 r = M(sdBox(b, vec3(0.22, 0.42, 0.22)), WHITE);
  float row = repC(b.y + 0.04, 0.17, 4.0);
  vec3 fx = faceX(b, 0.22);
  vec3 fz = faceZ(b, 0.22);
  float wx = sdRect(vec2(fx.x, row), vec2(0.15, 0.045), 0.0);
  float wz = sdRect(vec2(fz.x, row), vec2(0.15, 0.045), 0.0);
  r = Sc(r, M(max(wx, -(fx.y + 0.02)), SKY));
  r = Sc(r, M(max(wz, -(fz.y + 0.02)), SKY));
  r = U(r, M(sdRoundBox(q - vec3(0.0, 0.86, 0.0), vec3(0.25, 0.025, 0.25), 0.01), STEEL));
  r = U(r, M(sdBox(q - vec3(-0.06, 0.93, -0.06), vec3(0.09, 0.05, 0.09)), WHITE));
  return r;
}

vec2 receipt(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.03, 0.0), vec3(0.2, 0.03, 0.12), 0.015), STEEL);
  vec3 v = view(q - vec3(0.0, 0.06, 0.0));
  v = rotX(v, 0.18);
  vec3 s = v - vec3(0.0, 0.27, 0.0);
  float zig = abs(mod(s.x, 0.06) - 0.03);
  float d = max(sdBox(s, vec3(0.17, 0.27, 0.012)), -0.24 - zig - s.y);
  vec2 sh = M(d, WHITE);
  vec3 f = faceZ(s, 0.012);
  float l = bar(f.xz, -0.11, 0.17, 0.12, 0.022);
  l = min(l, bar(f.xz, -0.11, 0.07, 0.22, 0.016));
  l = min(l, bar(f.xz, -0.11, 0.0, 0.18, 0.016));
  l = min(l, bar(f.xz, -0.11, -0.07, 0.2, 0.016));
  sh = decal(sh, f, l, STEEL);
  sh = decal(sh, f, bar(f.xz, 0.0, -0.16, 0.11, 0.022), LIME);
  return U(r, sh);
}

vec2 folder(vec3 q) {
  vec3 v = view(q);
  vec3 g = v - vec3(0.0, 0.22, 0.0);
  vec2 r = M(sdRoundBox(g - vec3(0.0, 0.0, -0.04), vec3(0.24, 0.2, 0.015), 0.008), LILAC);
  r = U(r, M(sdRoundBox(g - vec3(-0.13, 0.21, -0.04), vec3(0.09, 0.03, 0.015), 0.008), LILAC));
  r = U(r, M(sdBox(g - vec3(0.03, 0.03, -0.012), vec3(0.17, 0.2, 0.006)), WHITE));
  vec3 f = rotX(g - vec3(0.0, -0.2, 0.01), 0.22) - vec3(0.0, 0.14, 0.0);
  r = U(r, M(sdRoundBox(f, vec3(0.24, 0.14, 0.015), 0.008), LILAC));
  vec3 seal = f - vec3(0.12, -0.02, 0.02);
  r = U(r, M(sdCylinder(rotX(seal, PI * 0.5), 0.055, 0.012), LIME + ROUND));
  r = U(r, M(sdRoundBox(q - vec3(0.0, 0.025, 0.0), vec3(0.22, 0.025, 0.14), 0.012), STEEL));
  return r;
}

vec2 parcel(vec3 q) {
  vec3 b = q - vec3(0.0, 0.18, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.22, 0.18, 0.2), 0.015), PAPER);
  r = seam(r, b.y - 0.17);
  r = paint(r, abs(b.x) - 0.05, LIME);
  vec3 fz = faceZ(b, 0.2);
  r = U(r, plate(fz - vec3(-0.11, 0.0, -0.06), vec2(0.07, 0.05), 0.01, 0.006, WHITE));
  return r;
}

vec2 node(vec3 p, vec2 at, float k) {
  vec3 q = p - vec3(at.x, 0.0, at.y);
  vec2 r = plinth(q, k);
  vec3 o = (q - vec3(0.0, 0.2, 0.0)) / K;
  vec2 b;
  if (k == 0.0) b = tower(o);
  else if (k == 1.0) b = house(o);
  else if (k == 2.0) b = receipt(o);
  else if (k == 3.0) b = folder(o);
  else b = parcel(o);
  b.x *= K;
  return U(r, b);
}

vec2 rod(vec3 p, vec3 a, vec3 b, float m) {
  return M(pipe(p, a, b, 0.05), m + SOLID);
}

vec2 collar(vec3 p, vec3 at, vec3 axis) {
  vec3 l = p - at;
  vec3 c = abs(axis.x) > 0.5 ? vec3(l.y, l.x, l.z) : vec3(l.x, l.z, l.y);
  return M(sdRoundCylinder(c, 0.085, 0.03, 0.012), WHITE + ROUND);
}

vec2 joint(vec3 p, vec3 at, float m) {
  return M(sdSphere(p - at, 0.095), m + SOLID);
}

vec2 links(vec3 p) {
  vec3 B = vec3(-SP, RY, -SP), H = vec3(SP, RY, -SP), I = vec3(-SP, RY, SP), C = vec3(SP, RY, SP);
  vec3 tb = vec3(0.0, RY, -SP), tf = vec3(0.0, RY, SP);
  vec3 X = vec3(PH, 0.0, 0.0), Z = vec3(0.0, 0.0, PH);
  vec2 r = rod(p, B + X, H - X, LIME);
  r = U(r, rod(p, B + Z, I - Z, GRAPHITE));
  r = U(r, rod(p, I + X, C - X, GRAPHITE));
  r = U(r, rod(p, H + Z, C - Z, GRAPHITE));
  r = U(r, rod(p, tb, vec3(0.0, RY, -PH), GRAPHITE));
  r = U(r, rod(p, tf, vec3(0.0, RY, PH), LIME));
  r = U(r, joint(p, tb, LIME));
  r = U(r, joint(p, tf, WHITE));
  vec3 x = vec3(1.0, 0.0, 0.0), z = vec3(0.0, 0.0, 1.0);
  vec3 e = vec3(0.02, 0.0, 0.02);
  r = U(r, collar(p, B + X + e * x, x));
  r = U(r, collar(p, H - X - e * x, x));
  r = U(r, collar(p, B + Z + e * z, z));
  r = U(r, collar(p, I - Z - e * z, z));
  r = U(r, collar(p, I + X + e * x, x));
  r = U(r, collar(p, C - X - e * x, x));
  r = U(r, collar(p, H + Z + e * z, z));
  r = U(r, collar(p, C - Z - e * z, z));
  r = U(r, collar(p, vec3(0.0, RY, -PH - 0.02), z));
  r = U(r, collar(p, vec3(0.0, RY, PH + 0.02), z));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = base(p);
  r = U(r, links(p));
  vec2 B = vec2(-SP, -SP), H = vec2(SP, -SP), I = vec2(-SP, SP), C = vec2(SP, SP);
  if (near(p - vec3(B.x, 0.9, B.y), vec3(0.5, 0.95, 0.5), r.x)) r = U(r, node(p, B, 0.0));
  if (near(p - vec3(H.x, 0.55, H.y), vec3(0.5, 0.6, 0.5), r.x)) r = U(r, node(p, H, 1.0));
  if (near(p - vec3(I.x, 0.55, I.y), vec3(0.5, 0.6, 0.5), r.x)) r = U(r, node(p, I, 2.0));
  if (near(p - vec3(C.x, 0.55, C.y), vec3(0.5, 0.6, 0.5), r.x)) r = U(r, node(p, C, 3.0));
  if (near(p - vec3(0.0, 0.55, 0.0), vec3(0.5, 0.6, 0.5), r.x)) r = U(r, node(p, vec2(0.0), 4.0));
  return r;
}
