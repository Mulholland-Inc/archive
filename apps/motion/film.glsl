#define IN(r, f, c, s) if ((s) > 0.002) { vec2 h_ = f((p - (c)) / (s)); h_.x *= (s); r = U(r, h_); }

float ramp(float a, float d) { return clamp((T - a) / d, 0.0, 1.0); }
float glide(float a, float d) { float x = ramp(a, d); return x * x * (3.0 - 2.0 * x); }
float pop(float a, float d) { float x = ramp(a, d) - 1.0; return 1.0 + x * x * (2.7 * x + 1.7); }
float hop(float a, float d) { return sin(PI * ramp(a, d)); }
vec3 bez(vec3 a, vec3 b, vec3 c, vec3 d, float t) {
  vec3 ab = mix(a, b, t), bc = mix(b, c, t), cd = mix(c, d, t);
  return mix(mix(ab, bc, t), mix(bc, cd, t), t);
}

vec2 fSlice(vec3 p) {
  vec2 r = M(sdRoundCylinder(p, 1.0, 0.34, 0.02), WHITE + ROUND);
  return U(r, M(sdCylinder(p - vec3(0.0, 0.36, 0.0), 0.93, 0.02), LIME + ROUND));
}
vec2 fDrum(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.03, 0.0), 0.95, 0.03), GRAPHITE + ROUND);
  for (int i = 0; i < 3; i++) r = U(r, fSlice(p - vec3(0.0, 0.42 + float(i) * 0.8, 0.0)));
  return r;
}
vec2 fKey(vec3 p, float m) {
  vec2 r = M(sdTorus(p - vec3(-0.5, 0.0, 0.0), 0.3, 0.1), m + ROUND);
  r = U(r, M(sdBox(p - vec3(0.2, 0.0, 0.0), vec3(0.45, 0.07, 0.08)), m));
  r = U(r, M(sdBox(p - vec3(0.58, 0.0, 0.18), vec3(0.08, 0.07, 0.14)), m));
  r = U(r, M(sdBox(p - vec3(0.32, 0.0, 0.15), vec3(0.08, 0.07, 0.1)), m));
  return r;
}
vec2 fBot(vec3 p) {
  vec3 m = mirX(p);
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.48, 0.0), vec3(0.36, 0.34, 0.28), 0.05), WHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.5, 0.27), vec3(0.28, 0.24, 0.03), 0.02), SKY));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 1.08, 0.0), vec3(0.3, 0.2, 0.24), 0.08), WHITE));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.86, 0.0), 0.09, 0.05), GRAPHITE + ROUND));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 1.08, 0.23), vec3(0.23, 0.12, 0.02), 0.03), GRAPHITE));
  r = U(r, M(sdSphere(m - vec3(0.11, 1.09, 0.26), 0.042), LILAC + SOLID));
  r = U(r, M(sdCapsule(p, vec3(0.0, 1.28, 0.0), vec3(0.0, 1.46, 0.0), 0.016), GRAPHITE));
  r = U(r, M(sdSphere(p - vec3(0.0, 1.5, 0.0), 0.055), LIME + SOLID));
  r = U(r, M(sdRoundBox(m - vec3(0.2, 0.08, 0.03), vec3(0.11, 0.08, 0.17), 0.04), GRAPHITE));
  return r;
}
