#pragma frame {"size":[800,640],"center":[0.0,0.13,0.0],"zoom":275,"bounds":[[-1.4,-0.1,-0.6],[1.4,1.4,0.6]]}

float houseShape(vec2 p) {
  return max(sdRect(p - vec2(0.0, -0.01), vec2(0.17, 0.22), 0.02), (abs(p.x) + p.y - 0.2) * 0.70710678);
}

vec2 slab(float d2, vec3 p, float m) {
  return M(extrude(d2, p.z, 0.034) - 0.004, m);
}

vec2 envelopeTile(vec3 p) {
  vec2 r = slab(sdRect(p.xy, vec2(0.2, 0.135), 0.02), p, WHITE);
  vec3 f = faceZ(p, 0.038);
  r = seam(r, -max(-(f.z - 0.135 + (0.2 - abs(f.x)) * 0.62), -(f.y + 0.003)));
  r = decal(r, f, sdRect(f.xz - vec2(0.135, 0.075), vec2(0.035, 0.03), 0.006), LIME);
  return r;
}

vec2 phoneTile(vec3 p) {
  vec2 r = slab(sdRect(p.xy, vec2(0.14, 0.27), 0.045), p, WHITE);
  vec3 f = faceZ(p, 0.038);
  r = inset(r, f - vec3(0.0, 0.0, 0.0), vec2(0.11, 0.21), 0.02, 0.01, GRAPHITE);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.24), vec2(0.03, 0.008), 0.008), STEEL);
  vec3 s = f + vec3(0.0, 0.01, 0.0);
  r = decal(r, s, sdRect(s.xz - vec2(-0.025, 0.12), vec2(0.06, 0.024), 0.024), LILAC);
  r = decal(r, s, sdRect(s.xz - vec2(0.025, 0.05), vec2(0.06, 0.024), 0.024), WHITE);
  r = decal(r, s, sdRect(s.xz - vec2(-0.03, -0.02), vec2(0.055, 0.024), 0.024), LILAC);
  r = U(r, M(sdBox(p - vec3(0.14, 0.12, 0.0), vec3(0.01, 0.04, 0.014)), STEEL));
  return r;
}

vec2 cardTile(vec3 p) {
  vec2 r = slab(sdRect(p.xy, vec2(0.2, 0.13), 0.022), p, LILAC);
  vec3 f = faceZ(p, 0.038);
  r = decal(r, f, sdRect(f.xz - vec2(-0.11, 0.015), vec2(0.045, 0.036), 0.01), STEEL);
  r = decal(r, f, sdRect(f.xz - vec2(0.03, -0.075), vec2(0.15, 0.016), 0.016), WHITE);
  r = decal(r, f, sdRect(f.xz - vec2(0.12, 0.07), vec2(0.045, 0.016), 0.016), WHITE);
  return r;
}

vec2 houseTile(vec3 p) {
  vec2 r = slab(houseShape(p.xy), p, WHITE);
  vec3 f = faceZ(p, 0.038);
  r = inset(r, f - vec3(0.05, 0.0, -0.145), vec2(0.045, 0.085), 0.012, 0.012, GRAPHITE);
  r = inset(r, f - vec3(-0.07, 0.0, -0.04), vec2(0.04, 0.04), 0.008, 0.01, STEEL);
  r = U(r, M(sdBox(p - vec3(-0.09, 0.14, 0.0), vec3(0.028, 0.08, 0.026)), WHITE));
  return r;
}

vec2 coinTile(vec3 p) {
  vec2 r = M(sdRoundCylinder(rotX(p, PI * 0.5), 0.2, 0.036, 0.012), SUN + SOLID);
  vec3 f = faceZ(p, 0.036);
  r = seam(r, -max(length(f.xz) - 0.155, -(f.y + 0.004)));
  r = U(r, M(extrude(sdCircle(f.xz, 0.085), f.y, 0.014), SUN + SOLID));
  r = seam(r, -max(length(f.xz) - 0.045, -(f.y - 0.01)));
  return r;
}

vec2 tray(vec3 p) {
  vec3 b = p - vec3(0.0, 0.09, 0.0);
  float outer = sdRoundBox(b, vec3(1.24, 0.09, 0.36), 0.03);
  float inner = sdBox(b - vec3(0.0, 0.06, 0.0), vec3(1.19, 0.09, 0.31));
  vec2 r = M(max(outer, -inner), WHITE);
  r = seam(r, b.y + 0.05);
  r = paint(r, b.y + 0.05, STEEL);
  vec3 dv = vec3(repC(p.x, 0.48, 4.0), p.y - 0.12, p.z);
  r = U(r, M(sdRoundBox(dv, vec3(0.014, 0.09, 0.31), 0.01), WHITE));
  float slot = repI(p.x, 0.48, 5.0);
  vec3 g = vec3(repC(p.x, 0.48, 5.0), p.y - 0.06, p.z + 0.04);
  r = Sc(r, M(sdBox(g, vec3(0.2, 0.02, 0.045)), GRAPHITE));
  vec3 fz = faceZ(b, 0.36);
  vec3 lq = vec3(repC(fz.x, 0.48, 5.0), fz.y, fz.z - 0.01);
  r = U(r, plate(lq, vec2(0.1, 0.035), 0.01, 0.01, slot == 3.0 ? SUN : STEEL));
  vec3 fx = faceX(b, 1.24);
  r = vents(r, fx - vec3(0.0, 0.0, 0.01), 5.0, 0.07, 0.07, 0.026, GRAPHITE);
  vec3 ft = vec3(abs(p.x) - 1.1, p.y - 0.0, abs(p.z) - 0.26);
  r = U(r, M(sdRoundCylinder(ft, 0.06, 0.02, 0.008), GRAPHITE + ROUND));
  vec3 sc = faceY(b, 0.09);
  r = U(r, screw(vec3(abs(sc.x) - 1.215, sc.y, sc.z - 0.0), 0.016));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = tray(p - vec3(0.0, 0.04, 0.0));
  float y0 = 0.09;
  r = U(r, envelopeTile(p - vec3(-0.96, y0 + 0.135, -0.04)));
  r = U(r, phoneTile(p - vec3(-0.48, y0 + 0.27, -0.04)));
  r = U(r, cardTile(p - vec3(0.0, y0 + 0.13, -0.04)));
  r = U(r, houseTile(p - vec3(0.96, y0 + 0.23, -0.04)));
  vec3 c = p - vec3(0.48, 0.82, 0.0);
  c = rotZ(rotY(c, 0.3), 0.2);
  r = U(r, coinTile(c));
  return r;
}
