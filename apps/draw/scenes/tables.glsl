#pragma frame {"size":[800,640],"center":[0.0,0.0,0.05],"zoom":200,"bounds":[[-1.6,-0.1,-1.4],[1.6,1.0,1.6]]}

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

float rowH(float i) {
  return i == 0.0 ? 0.1 : 0.05 + 0.018 * mod(i * 7.0, 3.0);
}

vec2 tile(vec3 p, float h, float m, float k) {
  vec3 c = p - vec3(0.0, h, 0.0);
  vec2 t = M(sdRoundBox(c, vec3(0.2, h, 0.125), 0.02), m);
  vec3 q = faceY(c, h);
  float b = m == WHITE ? STEEL : WHITE;
  float len = 0.12 + 0.07 * mod(k * 5.0, 3.0);
  t = decal(t, q, bar(q.xz, -0.15, 0.0, len, 0.022), m == LILAC ? WHITE : k < 0.0 ? LILAC : b);
  return t;
}

vec2 slab(vec3 p) {
  vec3 b = p - vec3(0.0, -0.09, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(1.35, 0.09, 1.05), 0.04), WHITE);
  r = seam(r, b.y + 0.03);
  r = paint(r, b.y + 0.05, STEEL);
  r = vents(r, faceX(p, 1.35) - vec3(-0.55, 0.0, -0.06), 6.0, 0.07, 0.04, 0.028, GRAPHITE);
  vec3 lz = faceZ(p, 1.05) - vec3(-0.95, 0.0, -0.06);
  r = U(r, led(vec3(repC(lz.x, 0.09, 3.0), lz.y, lz.z), 0.022, LIME));
  vec3 cs = mirXZ(faceY(p, 0.0));
  r = U(r, screw(vec3(cs.x - 1.27, cs.y, cs.z - 0.97), 0.026));
  vec3 slot = faceY(p, 0.0) - vec3(0.15, 0.0, 0.15);
  r = inset(r, slot, vec2(1.105, 0.145), 0.03, 0.03, STEEL);
  if (sdRect(slot.xz, vec2(1.105, 0.145), 0.03) < 0.0) {
    r = seam(r, sdRect(vec2(repC(slot.x, 0.44, 5.0), slot.z), vec2(0.2, 0.125), 0.02));
  }
  return r;
}

vec2 grid(vec3 p) {
  vec2 r = M(1e9, 0.0);
  float xg = p.x - 0.15;
  float col = repI(xg, 0.44, 5.0);
  float rx = repC(xg, 0.44, 5.0);
  for (int i = 0; i < 6; i++) {
    float row = float(i);
    float rz = p.z + 0.75 - 0.3 * row;
    float lift = row == 3.0 ? 0.3 : 0.0;
    float h = row == 3.0 ? 0.045 : rowH(row);
    float m = row == 0.0 ? LILAC : row == 3.0 ? LIME : (row == 5.0 && col == 3.0) ? SUN : WHITE;
    r = U(r, tile(vec3(rx, p.y - lift, rz), h, m, col == 2.0 && m == WHITE ? -1.0 : row + col * 2.0));
    vec3 mk = vec3(p.x + 1.12, p.y, rz);
    r = U(r, M(sdRoundBox(mk - vec3(0.0, 0.035, 0.0), vec3(0.08, 0.035, 0.11), 0.02), row == 0.0 ? LILAC : row == 3.0 ? LIME : STEEL));
  }
  return r;
}

vec2 handle(vec3 p) {
  vec3 h = p - vec3(0.15, -0.06, 1.13);
  vec2 r = M(sdRoundBox(h, vec3(0.16, 0.05, 0.1), 0.03), LILAC);
  vec3 t = faceY(h, 0.05);
  r = vents(r, vec3(t.x, t.y, t.z - 0.01), 3.0, 0.06, 0.1, 0.022, GRAPHITE);
  r = U(r, M(sdBox(p - vec3(0.15, -0.06, 1.06), vec3(0.06, 0.03, 0.02)), STEEL));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = slab(p);
  if (near(p - vec3(0.0, 0.2, 0.0), vec3(1.3, 0.25, 1.0), r.x)) r = U(r, grid(p));
  r = U(r, handle(p));
  return r;
}
