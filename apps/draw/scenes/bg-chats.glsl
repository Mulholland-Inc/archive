#pragma frame {"size":[2400,700],"center":[0.0,0.7,0.0],"zoom":148,"trim":false,"bounds":[[-40.0,-0.2,-40.0],[40.0,3.2,40.0]]}

#define ROW 3.2

float hash(vec2 c) { vec3 q = fract(vec3(c.xyx) * 0.1031); q += dot(q, q.yzx + 33.33); return fract((q.x + q.y) * q.z); }

vec2 ground(vec3 p) {
  vec2 r = M(p.y, PAPER);
  r = stripes(r, p.x, 1.6);
  return stripes(r, p.z, 1.6);
}

vec2 flatEnvelope(vec3 p, float m) {
  vec2 e = M(sdRoundBox(p, vec3(0.42, 0.03, 0.3), 0.01), m);
  vec3 t = faceY(p, 0.03);
  return seam(e, t.z + 0.3 - (0.42 - abs(t.x)) * 0.62);
}

vec2 pigeonholes(vec3 p, vec2 id) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.8, 0.0), vec3(0.65, 0.8, 0.3)), WHITE);
  r = paint(r, p.y - 0.12, STEEL);
  r = U(r, M(sdBox(p - vec3(0.0, 1.63, 0.0), vec3(0.65, 0.03, 0.34)), WHITE));
  vec3 f = faceZ(p, 0.3);
  float ci = repI(f.x, 0.62, 2.0), ri = repI(f.z - 0.88, 0.46, 3.0);
  vec3 g = vec3(repC(f.x, 0.62, 2.0), f.y, repC(f.z - 0.88, 0.46, 3.0));
  r = Sc(r, M(max(sdRect(g.xz, vec2(0.26, 0.18), 0.0), -(g.y + 0.4)), PAPER));
  float h = hash(id * 1.3 + vec2(ci, ri * 3.0));
  if (h > 0.4) {
    vec3 e = vec3(g.x, g.z + 0.15, g.y + 0.12);
    r = U(r, M(sdBox(e, vec3(0.2, 0.025, 0.2)), h > 0.88 ? SKY : WHITE));
  }
  return r;
}

vec2 mailbox(vec3 p) {
  vec3 b = p - vec3(0.0, 0.62, 0.0);
  float body = min(sdBox(b, vec3(0.4, 0.38, 0.38)), max(length(b.xy - vec2(0.0, 0.38)) - 0.4, abs(b.z) - 0.38));
  vec2 r = M(body, WHITE + SOLID);
  r = seam(r, -(abs(b.y + 0.08) - 0.06));
  vec3 f = faceZ(b, 0.38);
  r = Sc(r, M(max(sdRect(f.xz - vec2(0.0, 0.34), vec2(0.22, 0.03), 0.03), -(f.y + 0.06)), GRAPHITE));
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.0, 0.08), vec2(0.14, 0.08), 0.02), f.y - 0.015, 0.015), LIME));
  vec3 l = vec3(abs(p.x) - 0.3, p.y - 0.12, abs(p.z) - 0.28);
  r = U(r, M(sdBox(l, vec3(0.05, 0.12, 0.05)), STEEL));
  return r;
}

vec2 letter(vec3 p) {
  vec3 b = p - vec3(0.0, 0.55, -0.05);
  vec2 r = M(sdRoundBox(b, vec3(0.62, 0.42, 0.05), 0.02), WHITE);
  vec3 f = faceZ(b, 0.05);
  r = seam(r, f.y > -0.01 ? f.z - 0.42 + (0.62 - abs(f.x)) * 0.62 : -1.0);
  r = U(r, M(extrude(length(f.xz - vec2(0.0, 0.02)) - 0.08, f.y - 0.01, 0.01), LIME));
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.44, 0.24), vec2(0.08, 0.09), 0.01), f.y - 0.012, 0.012), SKY));
  r = U(r, M(sdBox(p - vec3(0.0, 0.07, -0.05), vec3(0.5, 0.07, 0.12)), STEEL));
  r = U(r, flatEnvelope(rotY(p - vec3(0.05, 0.03, 0.4), -0.1), WHITE));
  return r;
}

vec2 bubble(vec3 p) {
  vec3 b = p - vec3(0.0, 0.8, 0.0);
  float d2 = sdRect(b.xy, vec2(0.62, 0.36), 0.3);
  vec2 tq = b.xy - vec2(-0.2, -0.3);
  d2 = min(d2, max(max(-tq.y - 0.5, tq.y), max(-tq.x - 0.02, tq.x + tq.y * 0.55 - 0.2)));
  vec2 r = M(max(d2, abs(b.z) - 0.12), WHITE);
  vec3 f = vec3(repC(b.x, 0.3, 3.0), b.z - 0.12, b.y);
  r = U(r, M(extrude(length(f.xz) - 0.075, f.y - 0.015, 0.015), LIME));
  return r;
}

vec2 cart(vec3 p, float k) {
  vec3 b = p - vec3(0.0, 0.48, 0.0);
  vec2 r = M(max(sdRoundBox(b, vec3(0.55, 0.32, 0.4), 0.06), -sdRoundBox(b - vec3(0.0, 0.12, 0.0), vec3(0.5, 0.32, 0.35), 0.05)), WHITE);
  r = U(r, M(sdBox(b - vec3(0.0, 0.08, 0.0), vec3(0.5, 0.02, 0.35)), PAPER));
  vec3 w = vec3(abs(p.x) - 0.42, p.y - 0.08, abs(p.z) - 0.3);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.08, 0.03), GRAPHITE + SOLID));
  for (int i = 0; i < 2; i++) {
    float fi = float(i);
    vec3 e = rotZ(b - vec3(-0.12 + fi * 0.22, 0.3 - fi * 0.06, -0.12 + fi * 0.22), 0.12 - fi * 0.24);
    vec2 en = M(sdBox(e, vec3(0.34, 0.22, 0.025)), (i == 1 && k > 0.5) ? SKY : WHITE);
    vec3 ef = faceZ(e, 0.025);
    en = seam(en, ef.y > -0.01 ? ef.z - 0.22 + (0.34 - abs(ef.x)) * 0.6 : -1.0);
    r = U(r, en);
  }
  return r;
}

vec2 map(vec3 p) {
  vec2 r = ground(p);
  float rz = floor(p.z / ROW + 0.5);
  float lz = p.z - rz * ROW;
  float ty = mod(rz, 3.0);
  if (ty == 0.0) {
    float bx = floor(p.x / 1.3 + 0.5);
    vec3 q = vec3(p.x - bx * 1.3, p.y, lz);
    if (near(q - vec3(0.0, 0.85, 0.0), vec3(0.7, 0.9, 0.4), r.x)) r = U(r, pigeonholes(q, vec2(bx, rz)));
    r.x = min(r.x, 0.65 - abs(q.x) + 0.02);
  } else if (ty == 1.0) {
    float bx = floor(p.x / 2.3 + 0.5);
    vec3 q = vec3(p.x - bx * 2.3, p.y, lz);
    float t = mod(bx, 3.0);
    if (near(q - vec3(0.0, 0.7, 0.1), vec3(0.75, 0.8, 0.75), r.x)) {
      if (t == 0.0) r = U(r, mailbox(q));
      else if (t == 1.0) r = U(r, letter(q));
      else r = U(r, bubble(q));
    }
    r.x = min(r.x, 1.15 - abs(q.x) + 0.35);
  } else {
    float bx = floor(p.x / 2.0 + 0.5);
    vec3 q = vec3(p.x - bx * 2.0, p.y, lz);
    if (near(q - vec3(0.0, 0.6, 0.0), vec3(0.75, 0.7, 0.6), r.x)) r = U(r, cart(rotY(q, 0.2 * sin(bx * 2.1)), hash(vec2(bx, rz))));
    r.x = min(r.x, 1.0 - abs(q.x) + 0.2);
  }
  r.x = min(r.x, ROW * 0.5 - abs(lz) + 0.6);
  return r;
}
