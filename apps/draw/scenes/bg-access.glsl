#pragma frame {"size":[2400,700],"center":[0.0,0.7,0.0],"zoom":148,"trim":false,"bounds":[[-40.0,-0.2,-40.0],[40.0,3.2,40.0]]}

#define ROW 3.2

float hash(vec2 c) { vec3 q = fract(vec3(c.xyx) * 0.1031); q += dot(q, q.yzx + 33.33); return fract((q.x + q.y) * q.z); }

vec2 ground(vec3 p) {
  vec2 r = M(p.y, PAPER);
  r = stripes(r, p.x, 1.6);
  return stripes(r, p.z, 1.6);
}

vec2 turnstile(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.5, 0.0), vec3(0.16, 0.5, 0.55), 0.04), WHITE);
  r = paint(r, p.y - 0.1, STEEL);
  vec3 t = faceY(p, 1.0);
  r = decal(r, t, sdRect(t.xz - vec2(0.0, 0.24), vec2(0.08, 0.12), 0.02), GRAPHITE);
  vec3 fl = vec3(abs(p.x) - 0.32, p.y - 0.68, p.z - 0.05);
  r = U(r, M(sdRoundBox(fl, vec3(0.16, 0.25, 0.02), 0.02), SKY));
  return r;
}

vec2 booth(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.7, 0.0), vec3(0.6, 0.7, 0.6)), WHITE);
  r = paint(r, p.y - 0.12, STEEL);
  r = inset(r, faceZ(p, 0.6) - vec3(0.0, 0.0, 0.95), vec2(0.42, 0.24), 0.02, 0.04, SKY);
  r = inset(r, faceX(p, 0.6) - vec3(0.1, 0.0, 0.62), vec2(0.22, 0.6), 0.0, 0.04, GRAPHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 1.46, 0.0), vec3(0.72, 0.06, 0.72)), WHITE));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.6, 0.0), 0.12, 0.08), LIME + SOLID));
  return r;
}

vec2 padlock(vec3 p) {
  vec3 b = p - vec3(0.0, 0.42, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.5, 0.4, 0.22), 0.06), WHITE);
  r = seam(r, -(abs(b.y) - 0.3));
  vec3 s = p - vec3(0.0, 0.82, 0.0);
  r = U(r, M(max(length(vec2(length(s.xy) - 0.3, s.z)) - 0.08, -s.y), STEEL + SOLID));
  vec3 f = faceZ(b, 0.22);
  r = Sc(r, M(max(min(length(f.xz - vec2(0.0, 0.05)) - 0.07, sdRect(f.xz - vec2(0.0, -0.06), vec2(0.03, 0.09), 0.0)), -(f.y + 0.05)), GRAPHITE));
  return r;
}

vec2 doorWall(vec3 p, vec2 id) {
  float k = hash(id);
  vec2 r = M(sdBox(p - vec3(0.0, 0.8, 0.0), vec3(1.3, 0.8, 0.22)), WHITE);
  r = paint(r, p.y - 0.12, STEEL);
  r = U(r, M(sdBox(p - vec3(0.0, 1.64, 0.0), vec3(1.3, 0.04, 0.27)), WHITE));
  vec3 f = faceZ(p, 0.22);
  vec3 d = f - vec3(-0.2, 0.0, 0.66);
  if (k < 0.3) {
    r = Sc(r, M(max(sdRect(d.xz, vec2(0.4, 0.64), 0.0), -(d.y + 0.15)), GRAPHITE));
    vec2 h = rot(1.1) * vec2(d.x + 0.4, d.y);
    vec3 pq = vec3(h.x, d.z, h.y);
    r = U(r, M(sdBox(pq - vec3(0.4, 0.0, 0.035), vec3(0.4, 0.64, 0.035)), WHITE));
  } else {
    r = inset(r, d, vec2(0.4, 0.64), 0.0, 0.06, WHITE);
    r = U(r, M(sdBox(d - vec3(0.27, -0.02, 0.0), vec3(0.035, 0.03, 0.11)), STEEL));
  }
  r = U(r, M(extrude(max(sdRect(d.xz, vec2(0.48, 0.7), 0.0), -sdRect(d.xz, vec2(0.4, 0.64), 0.0)), d.y - 0.03, 0.03), WHITE));
  r = U(r, M(extrude(sdRect(d.xz - vec2(0.0, 0.86), vec2(0.3, 0.08), 0.02), d.y - 0.025, 0.025), k > 0.7 ? LIME : SKY));
  if (k > 0.3 && k < 0.5) r = U(r, padlock((vec3(d.x - 0.27, d.z + 0.28, d.y - 0.12)) * 3.2) / vec2(3.2, 1.0));
  vec3 kp = f - vec3(0.55, 0.0, 0.75);
  r = U(r, M(extrude(sdRect(kp.xz, vec2(0.09, 0.14), 0.02), kp.y - 0.03, 0.03), k > 0.55 ? LIME : STEEL));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = ground(p);
  float rz = floor(p.z / ROW + 0.5);
  float lz = p.z - rz * ROW;
  float ty = mod(rz, 3.0);
  if (ty == 0.0) {
    float bx = floor(p.x / 2.6 + 0.5);
    vec3 q = vec3(p.x - bx * 2.6, p.y, lz);
    if (near(q - vec3(0.0, 0.85, 0.3), vec3(1.35, 0.9, 0.7), r.x)) r = U(r, doorWall(q, vec2(bx, rz)));
    r.x = min(r.x, 1.3 - abs(q.x) + 0.05);
  } else if (ty == 1.0) {
    float bx = floor(p.x / 1.3 + 0.5);
    vec3 q = vec3(p.x - bx * 1.3, p.y, lz);
    if (near(q - vec3(0.0, 0.5, 0.0), vec3(0.65, 0.6, 0.6), r.x)) r = U(r, turnstile(q));
    r.x = min(r.x, 0.65 - abs(q.x) + 0.05);
  } else {
    float bx = floor(p.x / 2.4 + 0.5);
    vec3 q = vec3(p.x - bx * 2.4, p.y, lz);
    if (mod(bx, 2.0) == 0.0) {
      if (near(q - vec3(0.0, 0.85, 0.0), vec3(0.75, 0.9, 0.75), r.x)) r = U(r, booth(q));
    } else {
      if (near(q - vec3(0.0, 0.6, 0.0), vec3(0.6, 0.65, 0.35), r.x)) r = U(r, padlock(q));
    }
    r.x = min(r.x, 1.2 - abs(q.x) + 0.4);
  }
  r.x = min(r.x, ROW * 0.5 - abs(lz) + 0.6);
  return r;
}
