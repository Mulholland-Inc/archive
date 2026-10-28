#pragma frame {"size":[800,640],"center":[0.0,0.06,0.0],"zoom":172,"bounds":[[-2.2,-0.1,-2.2],[2.2,1.4,2.2]]}

vec3 view(vec3 d) {
  return vec3(dot(d.xz, vec2(0.70710678, -0.70710678)), d.y, dot(d.xz, vec2(0.70710678, 0.70710678)));
}

vec3 along(vec3 d, vec2 dir) {
  return vec3(dot(d.xz, dir), d.y, dot(d.xz, vec2(-dir.y, dir.x)));
}

float half2(vec2 p, vec2 a, vec2 b) {
  vec2 e = normalize(b - a);
  return (p - a).x * e.y - (p - a).y * e.x;
}

float tri2(vec2 p, vec2 a, vec2 b, vec2 c) {
  return max(max(half2(p, a, b), half2(p, b, c)), half2(p, c, a));
}

float dot2(vec2 v) { return dot(v, v); }

float sdBezier(vec2 pos, vec2 A, vec2 B, vec2 C) {
  vec2 a = B - A;
  vec2 b = A - 2.0 * B + C;
  vec2 c = a * 2.0;
  vec2 d = A - pos;
  float kk = 1.0 / dot(b, b);
  float kx = kk * dot(a, b);
  float ky = kk * (2.0 * dot(a, a) + dot(d, b)) / 3.0;
  float kz = kk * dot(d, a);
  float res;
  float p = ky - kx * kx;
  float q = kx * (2.0 * kx * kx - 3.0 * ky) + kz;
  float h = q * q + 4.0 * p * p * p;
  if (h >= 0.0) {
    h = sqrt(h);
    vec2 x = (vec2(h, -h) - q) / 2.0;
    vec2 uv = sign(x) * pow(abs(x), vec2(1.0 / 3.0));
    float t = clamp(uv.x + uv.y - kx, 0.0, 1.0);
    res = dot2(d + (c + b * t) * t);
  } else {
    float z = sqrt(-p);
    float v = acos(q / (p * z * 2.0)) / 3.0;
    float m = cos(v);
    float n = sin(v) * 1.732050808;
    vec3 t = clamp(vec3(m + m, -n - m, n - m) * z - kx, 0.0, 1.0);
    res = min(dot2(d + (c + b * t.x) * t.x), dot2(d + (c + b * t.y) * t.y));
  }
  return sqrt(res);
}

const float CY = 0.08;

vec2 plug(vec3 p, vec3 at, vec2 dir, float m) {
  vec3 l = along(p - at, dir);
  vec2 r = M(sdRoundBox(l - vec3(0.065, 0.0, 0.0), vec3(0.055, 0.045, 0.055), 0.015), m);
  r = seam(r, l.x - 0.035);
  r = U(r, M(sdRoundBox(l - vec3(0.0, 0.0, 0.0), vec3(0.02, 0.035, 0.042), 0.008), STEEL));
  return r;
}

vec2 envelope(vec3 g) {
  vec2 r = M(sdRoundBox(g, vec3(0.17, 0.115, 0.022), 0.008), WHITE);
  float v = g.y - 0.115 + (0.17 - abs(g.x)) * 0.62;
  r = seam(r, -v);
  r = U(r, M(sdCylinder(rotX(g - vec3(0.0, 0.01, 0.026), PI * 0.5), 0.032, 0.008), LILAC + ROUND));
  return r;
}

vec2 folder(vec3 g) {
  vec2 r = M(sdRoundBox(g - vec3(0.0, 0.0, -0.025), vec3(0.18, 0.12, 0.012), 0.006), SUN);
  r = U(r, M(sdRoundBox(g - vec3(-0.1, 0.135, -0.025), vec3(0.065, 0.025, 0.012), 0.006), SUN));
  vec3 s = g - vec3(0.03, 0.0, 0.0);
  r = U(r, M(sdBox(s, vec3(0.12, 0.1, 0.004)), WHITE));
  vec3 f = rotX(g - vec3(0.0, -0.12, 0.012), -0.2) - vec3(0.0, 0.075, 0.0);
  vec2 front = M(sdRoundBox(f, vec3(0.18, 0.075, 0.012), 0.006), SUN);
  r = U(r, front);
  return r;
}

vec2 chat(vec3 g) {
  float b2 = min(sdRect(g.xy - vec2(0.0, 0.02), vec2(0.18, 0.105), 0.07), tri2(g.xy, vec2(-0.14, -0.04), vec2(-0.17, -0.15), vec2(-0.03, -0.04)));
  vec2 r = M(extrude(b2, g.z, 0.035) - 0.0, LILAC);
  vec3 d = g - vec3(0.0, 0.02, 0.04);
  d.x = repC(d.x, 0.09, 3.0);
  r = U(r, M(sdCylinder(rotX(d, PI * 0.5), 0.024, 0.008), WHITE + ROUND));
  return r;
}

vec2 calendar(vec3 g) {
  vec2 r = M(sdRoundBox(g, vec3(0.15, 0.14, 0.03), 0.01), WHITE);
  r = paint(r, -(g.y - 0.07), STEEL);
  r = seam(r, g.y - 0.07);
  vec3 c = g - vec3(0.0, -0.035, 0.03);
  vec2 cell = vec2(repC(c.x, 0.075, 4.0), repC(c.y, 0.06, 2.0));
  float ix = repI(c.x, 0.075, 4.0), iy = repI(c.y, 0.06, 2.0);
  float m = (ix == 2.0 && iy == 1.0) ? LIME : SKY;
  r = Sc(r, M(max(sdRect(cell, vec2(0.026, 0.02), 0.006), -(c.z + 0.012)), m));
  vec3 k = vec3(abs(g.x) - 0.075, g.y - 0.14, g.z);
  r = U(r, M(sdTorus(rotZ(k, PI * 0.5).yxz, 0.035, 0.011), GRAPHITE + SOLID));
  return r;
}

vec2 ticket(vec3 g) {
  float b = sdRoundBox(g, vec3(0.19, 0.095, 0.02), 0.006);
  vec3 n = vec3(abs(g.x) - 0.19, g.y, g.z);
  b = max(b, -sdCylinder(rotX(n, PI * 0.5), 0.035, 0.05));
  vec2 r = M(b, WHITE);
  r = paint(r, -(g.x - 0.08), SUN);
  vec3 h = vec3(g.x - 0.08, repC(g.y, 0.032, 5.0), g.z);
  r = Sc(r, M(sdCylinder(rotX(h, PI * 0.5), 0.008, 0.05), GRAPHITE + ROUND));
  vec3 s = g - vec3(-0.06, 0.03, 0.02);
  r = U(r, M(extrude(sdRect(s.xy, vec2(0.08, 0.012), 0.012), s.z, 0.004), STEEL));
  r = U(r, M(extrude(sdRect(s.xy - vec2(-0.02, -0.06), vec2(0.06, 0.012), 0.012), s.z, 0.004), STEEL));
  return r;
}

vec2 tile(vec3 p, vec3 c, float k) {
  vec3 q = p - c;
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.07, 0.0), vec3(0.28, 0.07, 0.28), 0.03), WHITE);
  r = seam(r, q.y - 0.045);
  vec3 tq = faceY(q, 0.14);
  r = inset(r, tq, vec2(0.21, 0.21), 0.03, 0.015, WHITE);
  vec3 cs = vec3(abs(tq.x) - 0.245, tq.y, abs(tq.z) - 0.245);
  r = U(r, screw(cs, 0.016));
  vec3 sq = faceX(q, 0.28);
  r = U(r, led(sq - vec3(0.14, 0.0, 0.07), 0.022, k == 0.0 ? LIME : STEEL));
  r = U(r, screw(sq - vec3(-0.12, 0.0, 0.07), 0.016));
  vec3 v = view(q - vec3(0.0, 0.14, 0.0));
  r = U(r, M(sdRoundBox(v - vec3(0.0, 0.015, 0.0), vec3(0.17, 0.015, 0.06), 0.008), GRAPHITE));
  r = Sc(r, M(sdBox(v - vec3(0.0, 0.03, 0.0), vec3(0.15, 0.02, 0.035)), INK));
  float sc = 1.4;
  vec3 g = (v - vec3(0.0, 0.035 + 0.13 * sc, 0.0)) / sc;
  vec2 o;
  if (k == 0.0) o = envelope(g);
  else if (k == 1.0) o = folder(g);
  else if (k == 2.0) o = chat(g - vec3(0.0, 0.02, 0.0));
  else if (k == 3.0) o = calendar(g - vec3(0.0, -0.01, 0.0));
  else o = ticket(g - vec3(0.0, -0.03, 0.0));
  o.x *= sc;
  return U(r, o);
}

vec2 hub(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.08, 0.0), vec3(0.56, 0.08, 0.56), 0.025), WHITE);
  r = seam(r, p.y - 0.135);
  vec3 b = p - vec3(0.0, 0.42, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.46, 0.28, 0.46), 0.04), WHITE);
  body = seam(body, b.y - 0.12);
  vec3 fx = faceX(p, 0.46);
  body = vents(body, fx - vec3(0.12, 0.0, 0.54), 6.0, 0.05, 0.12, 0.022, GRAPHITE);
  vec3 fz = faceZ(p, 0.46);
  body = vents(body, fz - vec3(0.0, 0.0, 0.54), 7.0, 0.05, 0.12, 0.022, GRAPHITE);
  r = U(r, body);
  vec3 lq = fx - vec3(-0.27, 0.0, 0.54);
  r = U(r, led(vec3(lq.x, lq.y, lq.z), 0.026, LIME));
  r = U(r, led(vec3(lq.x - 0.08, lq.y, lq.z), 0.026, STEEL));
  vec3 t = faceY(p, 0.68);
  r = inset(r, t, vec2(0.36, 0.36), 0.04, 0.02, STEEL);
  vec3 cap = t - vec3(0.0, -0.02, 0.0);
  r = U(r, M(sdRoundCylinder(cap - vec3(0.0, 0.05, 0.0), 0.24, 0.05, 0.02), WHITE + ROUND));
  r = U(r, M(sdRoundCylinder(cap - vec3(0.0, 0.12, 0.0), 0.12, 0.03, 0.012), LIME + ROUND));
  r = U(r, screw(faceY(polarY(cap, 6.0), 0.1) - vec3(0.19, 0.0, 0.0), 0.016));
  vec3 cs = vec3(abs(t.x) - 0.41, t.y, abs(t.z) - 0.41);
  r = U(r, screw(cs, 0.02));
  vec3 an = t - vec3(-0.3, 0.0, -0.3);
  r = U(r, M(sdRoundCylinder(an - vec3(0.0, 0.02, 0.0), 0.05, 0.02, 0.008), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(an - vec3(0.0, 0.2, 0.0), 0.012, 0.18), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(an - vec3(0.0, 0.4, 0.0), 0.035), WHITE + SOLID));
  r = U(r, M(sdTorus(an - vec3(0.0, 0.22, 0.0), 0.03, 0.008), STEEL + SOLID));
  return r;
}

vec2 link(vec3 p, vec2 s, vec2 n, vec2 pt, vec2 h, float m) {
  vec2 a = s + n * 0.12;
  vec2 e = pt + h * 0.12;
  float d2 = abs(dot(n, h)) > 0.5 ? sdSeg(p.xz, a, e) : sdBezier(p.xz, a, abs(n.x) > 0.5 ? vec2(e.x, a.y) : vec2(a.x, e.y), e);
  vec2 r = M(length(vec2(d2, p.y - CY)) - 0.032, m + SOLID);
  float pm = m == LIME ? LIME : WHITE;
  r = U(r, plug(p, vec3(s.x, CY, s.y), n, pm));
  r = U(r, plug(p, vec3(pt.x, CY, pt.y), h, pm));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.55, 0.0), vec3(0.6, 0.6, 0.6), r.x)) r = U(r, hub(p));
  vec3 T0 = vec3(-1.12, 0.0, 1.12);
  vec3 T1 = vec3(-1.62, 0.0, -0.18);
  vec3 T2 = vec3(-0.18, 0.0, -1.62);
  vec3 T3 = vec3(1.12, 0.0, -1.12);
  vec3 T4 = vec3(1.18, 0.0, 1.18);
  if (near(p - T0 - vec3(0.0, 0.25, 0.0), vec3(0.32, 0.3, 0.32), r.x)) r = U(r, tile(p, T0, 0.0));
  if (near(p - T1 - vec3(0.0, 0.25, 0.0), vec3(0.32, 0.3, 0.32), r.x)) r = U(r, tile(p, T1, 1.0));
  if (near(p - T2 - vec3(0.0, 0.25, 0.0), vec3(0.32, 0.3, 0.32), r.x)) r = U(r, tile(p, T2, 2.0));
  if (near(p - T3 - vec3(0.0, 0.25, 0.0), vec3(0.32, 0.3, 0.32), r.x)) r = U(r, tile(p, T3, 3.0));
  if (near(p - T4 - vec3(0.0, 0.25, 0.0), vec3(0.32, 0.3, 0.32), r.x)) r = U(r, tile(p, T4, 4.0));
  if (near(p - vec3(0.0, 0.08, 0.0), vec3(2.0, 0.08, 2.0), r.x)) {
    r = U(r, link(p, vec2(-0.22, 0.56), vec2(0.0, 1.0), T0.xz + vec2(0.28, 0.0), vec2(1.0, 0.0), LIME));
    r = U(r, link(p, vec2(-0.56, -0.18), vec2(-1.0, 0.0), T1.xz + vec2(0.28, 0.0), vec2(1.0, 0.0), GRAPHITE));
    r = U(r, link(p, vec2(-0.18, -0.56), vec2(0.0, -1.0), T2.xz + vec2(0.0, 0.28), vec2(0.0, 1.0), GRAPHITE));
    r = U(r, link(p, vec2(0.56, -0.22), vec2(1.0, 0.0), T3.xz + vec2(0.0, 0.28), vec2(0.0, 1.0), GRAPHITE));
    r = U(r, link(p, vec2(0.22, 0.56), vec2(0.0, 1.0), T4.xz + vec2(-0.28, 0.0), vec2(-1.0, 0.0), GRAPHITE));
  }
  return r;
}
