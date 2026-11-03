#pragma frame {"size":[360,360],"center":[0.0,0.433,0.0],"zoom":290,"yaw":0,"pitch":0,"bounds":[[-0.8,-0.2,-0.3],[0.8,1.2,0.3]]}
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
  // round glasses, a bit too big for the face
  vec2 q = p.xy - vec2(0.0, 0.29);
  vec2 lq = vec2(abs(q.x) - 0.145, q.y);
  float lens = length(lq) - 0.105;
  r = U(r, M(extrude(lens, p.z - 0.07, 0.012), SKY + SOLID));
  float frame = abs(lens) - 0.022;
  float bridge = abs(length(q - vec2(0.0, -0.035)) - 0.06) - 0.016;
  bridge = max(bridge, -(q.y - 0.0) );
  bridge = max(bridge, abs(q.x) - 0.06);
  r = U(r, M(extrude(min(frame, bridge), p.z - 0.09, 0.02), GRAPHITE + SOLID));
  // eyes, glancing up and to the side
  vec2 eq = vec2(abs(q.x - 0.012) - 0.145, q.y - 0.012);
  r = U(r, M(extrude(length(eq) - 0.032, p.z - 0.09, 0.012), INK + SOLID));
  // small smile
  vec2 sq = p.xy - vec2(-0.01, 0.17);
  float smile = max(abs(length(sq) - 0.045) - 0.011, sq.y + 0.012);
  r = U(r, M(extrude(smile, p.z - 0.06, 0.01), INK + SOLID));
  return r;
}
