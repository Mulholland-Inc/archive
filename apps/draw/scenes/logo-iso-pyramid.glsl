#pragma frame {"size":[400,400],"center":[0.0,0.4,0.0],"zoom":240,"bounds":[[-0.9,-0.1,-0.9],[0.9,1.0,0.9]]}
// The Archive mark in 2D, side 1, base on y=0, apex at (0, 0.866):
// an equilateral triangle cut by a slash that falls three down for one across.
float sdTri(vec2 p) {
  const float k = 1.7320508;
  p.y -= 0.288675;
  p.x = abs(p.x) - 0.5;
  p.y = p.y + 0.5 / k;
  if (p.x + k * p.y > 0.0) p = vec2(p.x - k * p.y, -k * p.x - p.y) / 2.0;
  p.x -= clamp(p.x, -1.0, 0.0);
  return -length(p) * sign(p.y);
}
const vec2 SLASH_C = vec2(0.2198, 0.0);
const vec2 SLASH_N = vec2(0.9488, 0.3158);
const float SLASH_H = 0.0363;
float slashD(vec2 p) { return dot(p - SLASH_C, SLASH_N); }
// left piece (the big triangle) and right piece (the sliver)
float markL(vec2 p) { return max(sdTri(p), slashD(p) + SLASH_H); }
float markR(vec2 p) { return max(sdTri(p), -(slashD(p) - SLASH_H)); }
float sdPyramid(vec3 p, float h) {
  float m2 = h * h + 0.25;
  p.xz = abs(p.xz);
  p.xz = (p.z > p.x) ? p.zx : p.xz;
  p.xz -= 0.5;
  vec3 q = vec3(p.z, h * p.y - 0.5 * p.x, h * p.x + 0.5 * p.y);
  float s = max(-q.x, 0.0);
  float t = clamp((q.y - 0.5 * p.z) / (m2 + 0.25), 0.0, 1.0);
  float a = m2 * (q.x + s) * (q.x + s) + q.y * q.y;
  float b = m2 * (q.x + 0.5 * t) * (q.x + 0.5 * t) + (q.y - m2 * t) * (q.y - m2 * t);
  float d2 = min(q.y, -q.x * m2 - q.y * 0.5) > 0.0 ? 0.0 : min(a, b);
  return sqrt((d2 + q.z * q.z) / m2) * sign(max(q.z, -p.y));
}
vec2 map(vec3 p) {
  // the slash, as a plane through the pyramid: the front-left face keeps the mark, the rest turns blue
  vec3 q = p;
  float py = sdPyramid(q, 0.82);
  float s = dot(vec2(q.x, q.y * 1.056) - SLASH_C, SLASH_N);
  vec2 r = M(max(py, s + SLASH_H), WHITE);
  r = U(r, M(max(py, -(s - SLASH_H)), LIME));
  return r;
}
