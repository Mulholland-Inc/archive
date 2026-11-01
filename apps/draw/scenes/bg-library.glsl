#pragma frame {"size":[2400,700],"center":[0.0,0.7,0.0],"zoom":148,"trim":false,"bounds":[[-40.0,-0.2,-40.0],[40.0,3.2,40.0]]}

#define ROW 3.2

float hash(vec2 c) { vec3 q = fract(vec3(c.xyx) * 0.1031); q += dot(q, q.yzx + 33.33); return fract((q.x + q.y) * q.z); }

vec2 ground(vec3 p) {
  vec2 r = M(p.y, PAPER);
  return stripes(r, p.z + 0.8, 0.8);
}

vec2 archiveBox(vec3 p, float m) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.17, 0.0), vec3(0.26, 0.17, 0.22)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.31, 0.0), vec3(0.27, 0.04, 0.23)), WHITE));
  vec3 f = faceZ(p, 0.22);
  return inset(r, f - vec3(0.0, 0.0, 0.15), vec2(0.13, 0.06), 0.01, 0.01, m);
}

vec2 shelfItems(vec3 s, float k) {
  vec2 r = M(1e9, 0.0);
  if (k < 0.4) {
    r = archiveBox(s - vec3(-0.29, 0.0, 0.0), k < 0.1 ? SKY : STEEL);
    r = U(r, archiveBox(s - vec3(0.29, 0.0, 0.0), STEEL));
  } else if (k < 0.75) {
    float i = clamp(round((s.x + 0.15) / 0.12), -2.0, 2.0);
    vec3 b = vec3(s.x + 0.15 - i * 0.12, s.y, s.z);
    float bm = (i == 1.0 && k > 0.6) ? LIME : WHITE;
    r = M(sdBox(b - vec3(0.0, 0.18, 0.0), vec3(0.055, 0.18, 0.24)), bm);
    r = U(r, M(sdBox(b - vec3(0.0, 0.25, 0.24), vec3(0.035, 0.045, 0.008)), bm == LIME ? WHITE : STEEL));
    r = U(r, archiveBox(s - vec3(0.34, 0.0, 0.0), STEEL));
  } else {
    vec2 st = M(sdBox(s - vec3(-0.2, 0.1, 0.0), vec3(0.3, 0.1, 0.2)), WHITE);
    r = stripes(st, s.y, 0.05);
    r = U(r, archiveBox(s - vec3(0.34, 0.0, 0.0), STEEL));
  }
  return r;
}

vec2 bookcase(vec3 p, vec2 id) {
  float outer = sdBox(p - vec3(0.0, 0.8, 0.0), vec3(0.7, 0.8, 0.3));
  float lv = clamp(floor((p.y - 0.1) / 0.5), 0.0, 2.0);
  vec3 c = vec3(p.x, p.y - 0.1 - lv * 0.5 - 0.23, p.z);
  float hole = sdBox(c - vec3(0.0, 0.0, 0.05), vec3(0.64, 0.21, 0.3));
  vec2 r = M(max(outer, -hole), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 1.63, 0.0), vec3(0.72, 0.03, 0.33)), WHITE));
  vec3 s = vec3(p.x, p.y - 0.12 - lv * 0.5, p.z - 0.02);
  r = U(r, shelfItems(s, hash(id * 1.7 + lv * 3.1)));
  if (lv > 0.5) r = U(r, shelfItems(s + vec3(0.0, 0.5, 0.0), hash(id * 1.7 + (lv - 1.0) * 3.1)));
  return r;
}

vec2 filing(vec3 p, float k) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.66, 0.0), vec3(0.38, 0.66, 0.45)), WHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 1.34, 0.0), vec3(0.41, 0.03, 0.48)), WHITE));
  r = U(r, M(sdBox(p - vec3(0.0, 0.03, 0.0), vec3(0.35, 0.03, 0.42)), GRAPHITE));
  vec3 f = faceZ(p, 0.45);
  float i = clamp(floor((f.z - 0.08) / 0.31), 0.0, 3.0);
  vec3 e = vec3(f.x, f.y, f.z - 0.08 - 0.155 - i * 0.31);
  r = U(r, M(extrude(sdRect(e.xz, vec2(0.34, 0.14), 0.02), e.y - 0.02, 0.02), WHITE));
  r = U(r, M(sdBox(e - vec3(0.0, 0.05, 0.04), vec3(0.1, 0.02, 0.022)), STEEL));
  r = U(r, M(extrude(sdRect(e.xz - vec2(0.0, -0.05), vec2(0.08, 0.035), 0.005), e.y - 0.046, 0.008), (i == 1.0 && k > 0.5) ? LIME : WHITE));
  return r;
}

vec2 stack(vec3 p, float k) {
  vec2 r = archiveBox(p - vec3(-0.29, 0.0, 0.05), STEEL);
  r = U(r, archiveBox(p - vec3(0.29, 0.0, 0.0), STEEL));
  r = U(r, archiveBox(rotY(p - vec3(0.0, 0.35, 0.02), 0.12), k > 0.5 ? SKY : STEEL));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = ground(p);
  float rz = floor(p.z / ROW + 0.5);
  float lz = p.z - rz * ROW;
  if (mod(rz, 2.0) == 0.0) {
    float bx = floor(p.x / 1.4 + 0.5);
    vec3 q = vec3(p.x - bx * 1.4, p.y, lz);
    if (near(q - vec3(0.0, 0.85, 0.0), vec3(0.75, 0.9, 0.4), r.x)) r = U(r, bookcase(q, vec2(bx, rz)));
    r.x = min(r.x, 0.7 - abs(q.x) + 0.02);
  } else {
    float bx = floor(p.x / 1.6 + 0.5);
    vec3 q = vec3(p.x - bx * 1.6, p.y, lz);
    float k = hash(vec2(bx, rz));
    if (near(q - vec3(0.0, 0.7, 0.0), vec3(0.6, 0.75, 0.55), r.x)) {
      if (mod(bx, 3.0) == 2.0) r = U(r, stack(q, k));
      else r = U(r, filing(q, k));
    }
    r.x = min(r.x, 0.8 - abs(q.x) + 0.2);
  }
  r.x = min(r.x, ROW * 0.5 - abs(lz) + 0.9);
  return r;
}
