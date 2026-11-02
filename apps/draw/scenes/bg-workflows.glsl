#pragma frame {"size":[2400,700],"center":[0.0,0.7,0.0],"zoom":142,"trim":false,"bounds":[[-40.0,-0.2,-40.0],[40.0,3.2,40.0]]}

#define ROW 3.4

float hash(vec2 c) { vec3 q = fract(vec3(c.xyx) * 0.1031); q += dot(q, q.yzx + 33.33); return fract((q.x + q.y) * q.z); }

vec2 ground(vec3 p) {
  vec2 r = M(p.y, PAPER);
  r = stripes(r, p.x, 1.7);
  return stripes(r, p.z + ROW * 0.5, 1.7);
}

vec2 parcel(vec3 p, float m) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.14, 0.0), vec3(0.2, 0.14, 0.17)), m);
  return seam(r, -(abs(p.x) - 0.035));
}

vec2 conveyor(vec3 p, float rz) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.42, 0.0), vec3(100.0, 0.05, 0.3)), WHITE);
  r = seam(r, -(abs(p.z) - 0.24));
  float lx = p.x - floor(p.x / 1.2 + 0.5) * 1.2;
  r = U(r, M(sdBox(vec3(lx, p.y - 0.19, abs(p.z) - 0.22), vec3(0.04, 0.19, 0.04)), STEEL));
  float pi = floor((p.x - 0.6) / 1.2 + 0.5);
  float k = hash(vec2(pi, rz));
  vec3 q = vec3(p.x - 0.6 - pi * 1.2, p.y - 0.47, p.z);
  if (k < 0.6) r = U(r, parcel(rotY(q, (k - 0.3) * 0.5), k < 0.12 ? SKY : WHITE));
  r.x = min(r.x, 0.6 - abs(q.x) + 0.15);
  return r;
}

vec2 machine(vec3 p, float k) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.62, 0.0), vec3(0.6, 0.6, 0.6)), WHITE);
  r = paint(r, p.y - 0.1, STEEL);
  r = Sc(r, M(sdBox(p - vec3(0.0, 0.42, 0.0), vec3(0.7, 0.13, 0.34)), GRAPHITE));
  vec3 fz = faceZ(p, 0.6);
  r = inset(r, fz - vec3(0.0, 0.0, 0.92), vec2(0.36, 0.14), 0.02, 0.04, SKY);
  r = U(r, M(extrude(length(faceX(p, 0.6).xz - vec2(0.0, 0.9)) - 0.09, faceX(p, 0.6).y - 0.02, 0.02), (k > 0.5 ? LIME : STEEL) + ROUND));
  r = U(r, M(sdBox(p - vec3(0.0, 1.24, 0.0), vec3(0.64, 0.03, 0.64)), WHITE));
  vec3 h = p - vec3(0.0, 1.27, 0.0);
  r = U(r, M(max(sdCone(h - vec3(0.0, 0.18, 0.0), 0.18, 0.18, 0.32), -sdCone(h - vec3(0.0, 0.22, 0.0), 0.18, 0.14, 0.28)), WHITE + SOLID));
  return r;
}

vec2 tanks(vec3 p, float rz) {
  float bx = floor(p.x / 2.0 + 0.5);
  vec3 t = vec3(p.x - bx * 2.0, p.y, p.z);
  float k = hash(vec2(bx, rz) + 3.0);
  float h = k > 0.5 ? 0.62 : 0.5;
  vec2 r = M(sdCylinder(t - vec3(0.0, h, 0.0), 0.6, h), WHITE + SOLID);
  r = paint(r, abs(t.y - h * 1.2) - 0.06, STEEL + SOLID);
  r = U(r, M(sdEllipsoid(t - vec3(0.0, h * 2.0, 0.0), vec3(0.6, 0.18, 0.6)), WHITE + SOLID));
  r = U(r, M(pipe(t, vec3(0.0, h * 2.0, 0.0), vec3(0.0, 1.75, 0.0), 0.09), STEEL + SOLID));
  vec3 vw = t - vec3(0.0, 1.5, 0.0);
  r = U(r, M(sdTorus(vw, 0.16, 0.03), (k > 0.6 ? LIME : GRAPHITE) + SOLID));
  r = U(r, M(sdBox(vw, vec3(0.16, 0.02, 0.02)), GRAPHITE + SOLID));
  vec3 pq = vec3(p.x, p.y - 1.75, p.z);
  r = U(r, M(length(pq.yz) - 0.13, k > 0.85 ? LIME + SOLID : SKY + SOLID));
  float fx = p.x - floor(p.x / 1.0 + 0.5) * 1.0;
  r = U(r, M(sdTorus(vec3(pq.y, fx, pq.z), 0.15, 0.035), STEEL + SOLID));
  vec3 g = t - vec3(0.0, h, 0.6);
  r = U(r, M(sdCylinder(rotX(g, PI * 0.5), 0.12, 0.04), (k < 0.3 ? LIME : WHITE) + SOLID));
  r.x = min(r.x, 1.0 - abs(t.x) + 0.3);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = ground(p);
  float rz = floor(p.z / ROW + 0.5);
  float lz = p.z - rz * ROW;
  if (mod(rz, 2.0) == 0.0) {
    float mx = floor(p.x / 4.8 + 0.5);
    vec3 m = vec3(p.x - mx * 4.8, p.y, lz);
    if (abs(lz) - 0.7 < r.x) {
      vec2 cv = conveyor(vec3(p.x, p.y, lz), rz);
      cv.x = max(cv.x, -sdBox(m - vec3(0.0, 0.8, 0.0), vec3(0.6, 0.6, 0.6)));
      r = U(r, cv);
    }
    if (near(m - vec3(0.0, 0.9, 0.0), vec3(0.7, 1.0, 0.7), r.x)) r = U(r, machine(m, hash(vec2(mx, rz))));
    r.x = min(r.x, 2.4 - abs(m.x) + 1.0);
  } else {
    if (abs(lz) - 0.8 < r.x) r = U(r, tanks(vec3(p.x, p.y, lz), rz));
  }
  r.x = min(r.x, ROW * 0.5 - abs(lz) + 0.8);
  return r;
}
