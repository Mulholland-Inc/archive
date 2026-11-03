#pragma frame {"size":[400,400],"center":[0.0,0.42,0.0],"zoom":270,"yaw":-45,"bounds":[[-0.8,-0.1,-0.5],[0.8,1.0,0.5]]}
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
vec2 map(vec3 p) {
  // squash a hair so the slopes read as tops, never flicker to sides
  vec2 m = vec2(p.x, p.y * 1.05);
  float d = 0.075;
  vec2 r = M(extrude(markL(m), p.z, d), WHITE);
  r = U(r, M(extrude(markR(m), p.z, d), LIME));
  return r;
}
