#pragma frame {"size":[2400,700],"center":[0.0,0.7,0.0],"zoom":142,"trim":false,"bounds":[[-40.0,-0.2,-40.0],[40.0,3.2,40.0]]}

#define CELL 2.7

float hash(vec2 c) { vec3 q = fract(vec3(c.xyx) * 0.1031); q += dot(q, q.yzx + 33.33); return fract((q.x + q.y) * q.z); }

vec2 ground(vec3 p) {
  vec2 r = M(p.y, PAPER);
  r = stripes(r, p.x + CELL * 0.5, CELL * 0.5);
  return stripes(r, p.z + CELL * 0.5, CELL * 0.5);
}

vec2 fronts(vec2 r, vec3 f, float rows, float y0, float skip) {
  if (f.y < -0.1) return r;
  vec2 pitch = vec2(0.56, 0.38);
  float id = repI(f.x, pitch.x, 2.0) + repI(f.z - y0, pitch.y, rows) * 2.0;
  if (id == skip) return r;
  vec3 g = vec3(repC(f.x, pitch.x, 2.0), f.y, repC(f.z - y0, pitch.y, rows));
  r = U(r, M(extrude(sdRect(g.xz, pitch * 0.5 - 0.035, 0.02), g.y - 0.02, 0.02), WHITE));
  r = U(r, M(sdBox(g - vec3(0.0, 0.05, -0.05), vec3(0.08, 0.02, 0.022)), STEEL));
  vec3 lb = g - vec3(0.0, 0.04, 0.06);
  r = decal(r, lb, sdRect(lb.xz, vec2(0.09, 0.04), 0.01), STEEL);
  return r;
}

vec2 cabinet(vec3 p, float k) {
  float rows = k > 0.55 ? 4.0 : 3.0;
  float top = 0.12 + rows * 0.38;
  vec2 r = M(sdBox(p - vec3(0.0, top * 0.5, 0.0), vec3(0.6, top * 0.5, 0.6)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.04, 0.0), vec3(0.56, 0.04, 0.56)), GRAPHITE));
  r = U(r, M(sdBox(p - vec3(0.0, top + 0.03, 0.0), vec3(0.64, 0.04, 0.64)), WHITE));
  float y0 = 0.1 + rows * 0.19;
  float pulled = k < 0.35 ? 1.0 : -1.0;
  r = fronts(r, faceZ(p, 0.6), rows, y0, pulled);
  r = fronts(r, faceX(p, 0.6), rows, y0, -1.0);
  if (pulled >= 0.0) {
    vec3 d = faceZ(p, 0.6) - vec3(0.28, 0.0, y0 - (rows - 1.0) * 0.19);
    r = Sc(r, M(max(sdRect(d.xz, vec2(0.24, 0.155), 0.0), -(d.y + 0.6)), GRAPHITE));
    vec3 o = d - vec3(0.0, 0.25, 0.0);
    r = U(r, M(max(sdBox(o, vec3(0.22, 0.25, 0.14)), -sdBox(o - vec3(0.0, 0.0, 0.05), vec3(0.19, 0.24, 0.14))), WHITE));
    r = U(r, M(sdBox(o - vec3(0.0, 0.0, 0.02), vec3(0.17, 0.2, 0.12)), WHITE));
    r = U(r, M(sdBox(o - vec3(0.06, -0.02, 0.17), vec3(0.06, 0.012, 0.04)), LIME));
    vec3 e = d - vec3(0.0, 0.5, 0.0);
    r = U(r, M(extrude(sdRect(e.xz, vec2(0.245, 0.155), 0.02), e.y - 0.02, 0.02), WHITE));
    r = U(r, M(sdBox(e - vec3(0.0, 0.05, -0.05), vec3(0.08, 0.02, 0.022)), STEEL));
  }
  return r;
}

vec2 disks(vec3 p, float k) {
  float n = 2.0 + floor(k * 2.99);
  float i = clamp(floor(p.y / 0.44), 0.0, n - 1.0);
  vec3 q = p - vec3(0.0, 0.2 + i * 0.44, 0.0);
  vec2 r = M(sdCylinder(q, 0.68, 0.2), (i == 1.0 ? SKY : WHITE) + SOLID);
  if (i > 0.0) r = U(r, M(sdCylinder(q + vec3(0.0, 0.44, 0.0), 0.68, 0.2), WHITE + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = ground(p);
  vec2 c = floor(p.xz / CELL + 0.5);
  vec2 l = p.xz - c * CELL;
  vec3 q = vec3(l.x, p.y, l.y);
  float k = hash(c * 1.37 + 4.0);
  if (near(q - vec3(0.0, 0.9, 0.2), vec3(0.75, 1.0, 1.0), r.x)) {
    if (k > 0.62) r = U(r, disks(q, fract(k * 7.0)));
    else r = U(r, cabinet(q, k / 0.62));
  }
  r.x = min(r.x, CELL * 0.5 - max(abs(l.x), abs(l.y)) + 0.2);
  return r;
}
