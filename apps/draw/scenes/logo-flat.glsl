#pragma frame {"size":[360,360],"center":[0.0,0.433,0.0],"zoom":300,"yaw":0,"pitch":0,"bounds":[[-0.8,-0.2,-0.3],[0.8,1.2,0.3]]}
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
  vec2 r = M(extrude(markL(p.xy), p.z, 0.05), WHITE + SOLID);
  r = U(r, M(extrude(markR(p.xy), p.z, 0.05), LIME + SOLID));
  return r;
}
