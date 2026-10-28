#define PI 3.14159265
#define TAU 6.28318531
#define ROUND 16.0
#define SOLID 32.0
#define ALT 64.0

vec2 M(float d, float m) { return vec2(d, m); }

float sdSphere(vec3 p, float r) { return length(p) - r; }

float sdBox(vec3 p, vec3 b) {
  vec3 q = abs(p) - b;
  return length(max(q, 0.0)) + min(max(q.x, max(q.y, q.z)), 0.0);
}

float sdRoundBox(vec3 p, vec3 b, float r) { return sdBox(p, b - r) - r; }

float sdChamferBox(vec3 p, vec3 b, float c) {
  vec3 a = abs(p);
  float e = max(max(a.x + a.y - (b.x + b.y - c), a.y + a.z - (b.y + b.z - c)), a.x + a.z - (b.x + b.z - c));
  return max(sdBox(p, b), e * 0.70710678);
}

float sdCylinder(vec3 p, float r, float h) {
  vec2 d = abs(vec2(length(p.xz), p.y)) - vec2(r, h);
  return min(max(d.x, d.y), 0.0) + length(max(d, 0.0));
}

float sdRoundCylinder(vec3 p, float r, float h, float e) {
  vec2 d = vec2(length(p.xz) - r + e, abs(p.y) - h + e);
  return min(max(d.x, d.y), 0.0) + length(max(d, 0.0)) - e;
}

float sdChamferCylinder(vec3 p, float r, float h, float c) {
  return max(sdCylinder(p, r, h), (length(p.xz) + abs(p.y) - (r + h - c)) * 0.70710678);
}

float sdCapsule(vec3 p, vec3 a, vec3 b, float r) {
  vec3 pa = p - a, ba = b - a;
  float h = clamp(dot(pa, ba) / dot(ba, ba), 0.0, 1.0);
  return length(pa - ba * h) - r;
}

float sdTorus(vec3 p, float R, float r) { return length(vec2(length(p.xz) - R, p.y)) - r; }

float sdCone(vec3 p, float h, float r1, float r2) {
  vec2 q = vec2(length(p.xz), p.y);
  vec2 k1 = vec2(r2, h);
  vec2 k2 = vec2(r2 - r1, 2.0 * h);
  vec2 ca = vec2(q.x - min(q.x, q.y < 0.0 ? r1 : r2), abs(q.y) - h);
  vec2 cb = q - k1 + k2 * clamp(dot(k1 - q, k2) / dot(k2, k2), 0.0, 1.0);
  float s = (cb.x < 0.0 && ca.y < 0.0) ? -1.0 : 1.0;
  return s * sqrt(min(dot(ca, ca), dot(cb, cb)));
}

float sdWedge(vec3 p, vec3 b) { return max(sdBox(p, b), dot(p.xy, normalize(b.yx))); }

float sdPrism(vec3 p, float w, float h, float d) {
  vec2 n = normalize(vec2(h, w));
  return max(max(-p.y, abs(p.z) - d), dot(vec2(abs(p.x), p.y), n) - w * n.x);
}

float sdSlab(vec3 p, float h) { return abs(p.y) - h; }

float sdEllipsoid(vec3 p, vec3 r) {
  float k0 = length(p / r);
  float k1 = length(p / (r * r));
  return k0 * (k0 - 1.0) / k1;
}

float sdHexPrism(vec3 p, float r, float h) {
  const vec3 k = vec3(-0.8660254, 0.5, 0.57735);
  vec3 a = abs(p.xzy);
  a.xy -= 2.0 * min(dot(k.xy, a.xy), 0.0) * k.xy;
  vec2 d = vec2(length(a.xy - vec2(clamp(a.x, -k.z * r, k.z * r), r)) * sign(a.y - r), a.z - h);
  return min(max(d.x, d.y), 0.0) + length(max(d, 0.0));
}

float sdRect(vec2 p, vec2 b, float r) {
  vec2 q = abs(p) - b + r;
  return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r;
}

float sdCircle(vec2 p, float r) { return length(p) - r; }

float sdSeg(vec2 p, vec2 a, vec2 b) {
  vec2 pa = p - a, ba = b - a;
  return length(pa - ba * clamp(dot(pa, ba) / dot(ba, ba), 0.0, 1.0));
}

float extrude(float d2, float y, float h) {
  vec2 w = vec2(d2, abs(y) - h);
  return min(max(w.x, w.y), 0.0) + length(max(w, 0.0));
}

float smin(float a, float b, float k) { float h = max(k - abs(a - b), 0.0) / k; return min(a, b) - h * h * k * 0.25; }
float smax(float a, float b, float k) { return -smin(-a, -b, k); }
float cmin(float a, float b, float k) { return min(min(a, b), (a + b - k) * 0.70710678); }
float cmax(float a, float b, float k) { return max(max(a, b), (a + b + k) * 0.70710678); }

vec2 U(vec2 a, vec2 b) { return a.x < b.x ? a : b; }
vec2 S(vec2 a, vec2 b) { return vec2(max(a.x, -b.x), a.y); }
vec2 Sc(vec2 a, vec2 b) { return -b.x > a.x ? vec2(-b.x, b.y) : a; }
vec2 I(vec2 a, vec2 b) { return a.x > b.x ? a : b; }
vec2 SU(vec2 a, vec2 b, float k) { return vec2(smin(a.x, b.x, k), a.x < b.x ? a.y : b.y); }
vec2 SS(vec2 a, vec2 b, float k) { return vec2(smax(a.x, -b.x, k), a.y); }
vec2 SI(vec2 a, vec2 b, float k) { return vec2(smax(a.x, b.x, k), a.x > b.x ? a.y : b.y); }
vec2 CU(vec2 a, vec2 b, float k) { return vec2(cmin(a.x, b.x, k), a.x < b.x ? a.y : b.y); }
vec2 CS(vec2 a, vec2 b, float k) { return vec2(cmax(a.x, -b.x, k), a.y); }
vec2 CI(vec2 a, vec2 b, float k) { return vec2(cmax(a.x, b.x, k), a.x > b.x ? a.y : b.y); }

mat2 rot(float a) { float c = cos(a), s = sin(a); return mat2(c, -s, s, c); }
vec3 rotX(vec3 p, float a) { p.yz = rot(a) * p.yz; return p; }
vec3 rotY(vec3 p, float a) { p.zx = rot(a) * p.zx; return p; }
vec3 rotZ(vec3 p, float a) { p.xy = rot(a) * p.xy; return p; }

vec3 mirX(vec3 p) { return vec3(abs(p.x), p.yz); }
vec3 mirZ(vec3 p) { return vec3(p.xy, abs(p.z)); }
vec3 mirXZ(vec3 p) { return vec3(abs(p.x), p.y, abs(p.z)); }

float repC(float x, float s, float n) {
  float o = 0.5 * (n - 1.0);
  return x - s * (clamp(round(x / s + o), 0.0, n - 1.0) - o);
}
float repI(float x, float s, float n) {
  float o = 0.5 * (n - 1.0);
  return clamp(round(x / s + o), 0.0, n - 1.0);
}
float repL(float x, float s, float n) { return x - s * clamp(round(x / s), 0.0, n - 1.0); }
float repLI(float x, float s, float n) { return clamp(round(x / s), 0.0, n - 1.0); }
vec2 rep2(vec2 p, vec2 s, vec2 n) { return vec2(repC(p.x, s.x, n.x), repC(p.y, s.y, n.y)); }
vec3 polarY(vec3 p, float n) {
  float a = TAU / n;
  float an = mod(atan(p.z, p.x) + a * 0.5, a) - a * 0.5;
  float r = length(p.xz);
  return vec3(r * cos(an), p.y, r * sin(an));
}

vec3 faceX(vec3 p, float x) { return vec3(p.z, p.x - x, p.y); }
vec3 faceNX(vec3 p, float x) { return vec3(p.z, x - p.x, p.y); }
vec3 faceZ(vec3 p, float z) { return vec3(p.x, p.z - z, p.y); }
vec3 faceNZ(vec3 p, float z) { return vec3(p.x, z - p.z, p.y); }
vec3 faceY(vec3 p, float y) { return vec3(p.x, p.y - y, p.z); }

bool near(vec3 p, vec3 h, float d) { return sdBox(p, h) < d; }

float flipAlt(float m) { return mod(m, 128.0) >= ALT ? m - ALT : m + ALT; }
vec2 seam(vec2 o, float d) { return d > 0.0 ? vec2(o.x, flipAlt(o.y)) : o; }
vec2 stripes(vec2 o, float x, float s) { return seam(o, mod(x, 2.0 * s) - s); }
vec2 paint(vec2 o, float d, float m) { return d < 0.0 ? vec2(o.x, m) : o; }
vec2 decal(vec2 o, vec3 q, float d2, float m) { return paint(o, max(d2, -q.y - 0.006), m); }
vec2 groove(vec2 o, float d, float w, float depth) { return vec2(max(o.x, -max(abs(d) - w, -o.x - depth)), o.y); }

vec2 inset(vec2 o, vec3 q, vec2 size, float r, float depth, float m) {
  return Sc(o, M(max(sdRect(q.xz, size, r), -(q.y + depth)), m));
}
vec2 plate(vec3 q, vec2 size, float r, float h, float m) {
  return M(extrude(sdRect(q.xz, size, r), q.y, h), m);
}
vec2 screw(vec3 q, float r) {
  vec2 h = M(extrude(length(q.xz) - r, q.y, r * 0.35), STEEL + ROUND);
  return seam(h, q.x + q.z);
}
vec2 vents(vec2 o, vec3 q, float n, float pitch, float len, float w, float m) {
  float x = repC(q.x, pitch, n);
  return Sc(o, M(max(sdRect(vec2(x, q.z), vec2(w * 0.5, len * 0.5), w * 0.5), -(q.y + max(w, len * 0.4))), m));
}
vec2 grille(vec2 o, vec3 q, vec2 n, float pitch, float r, float m) {
  vec2 c = rep2(q.xz, vec2(pitch), n);
  return Sc(o, M(max(length(c) - r, -(q.y + r)), m));
}
vec2 button(vec3 q, float r, float h, float m) {
  return M(sdRoundCylinder(q, r, h, min(h, r) * 0.5), m + ROUND);
}
vec2 led(vec3 q, float r, float m) {
  return M(max(sdSphere(q, r), -q.y - r * 0.2), m + SOLID);
}
float pipe(vec3 p, vec3 a, vec3 b, float r) { return sdCapsule(p, a, b, r); }
float ring(vec3 p, float r, float h, float e) { return sdRoundCylinder(p, r, h, e); }
float ladder(vec3 p, float h, float w, float pitch, float r) {
  vec3 q = vec3(abs(p.x) - w, p.y - h * 0.5, p.z);
  float rails = sdBox(q, vec3(r, h * 0.5, r));
  float n = floor(h / pitch);
  float y = repC(p.y - h * 0.5, pitch, n);
  float rungs = sdBox(vec3(p.x, y, p.z), vec3(w, r * 0.7, r * 0.7));
  return min(rails, rungs);
}
