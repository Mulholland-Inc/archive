#pragma frame {"size":[420,420],"center":[-0.03,0.5,0.0],"zoom":250,"yaw":-22,"pitch":14,"bounds":[[-0.9,-0.1,-0.6],[0.9,1.3,0.6]]}
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
  vec2 m = vec2(p.x, p.y * 1.05);
  float d = 0.1;
  vec2 r = M(extrude(markL(m), p.z, d), WHITE);
  r = U(r, M(extrude(markR(m), p.z, d), LIME));
  // glasses on the front face
  vec3 f = p - vec3(-0.01, 0.27, d);
  vec2 lq = vec2(abs(f.x) - 0.135, f.y);
  float lens = length(lq) - 0.095;
  r = U(r, M(extrude(lens, f.z - 0.01, 0.01), SKY + SOLID));
  float rim = abs(lens) - 0.019;
  float bridge = max(max(abs(length(f.xy - vec2(0.0, -0.03)) - 0.058) - 0.014, -f.y), abs(f.x) - 0.06);
  r = U(r, M(extrude(min(rim, bridge), f.z - 0.022, 0.022), GRAPHITE + SOLID));
  vec2 eq = vec2(abs(f.x - 0.012) - 0.135, f.y - 0.01);
  r = U(r, M(extrude(length(eq) - 0.029, f.z - 0.022, 0.01), INK + SOLID));
  // mortarboard balanced on the apex
  vec3 h = rotZ(p - vec3(0.0, 0.8, 0.0), 0.16);
  vec2 cap = M(sdRoundCylinder(h - vec3(0.0, 0.035, 0.0), 0.095, 0.045, 0.013), GRAPHITE + ROUND);
  vec3 b = rotY(h - vec3(0.0, 0.085, 0.0), 0.785398);
  vec2 board = M(sdRoundBox(b, vec3(0.165, 0.013, 0.165), 0.008), GRAPHITE);
  vec2 knob = M(sdRoundCylinder(h - vec3(0.0, 0.107, 0.0), 0.028, 0.012, 0.008), LIME + ROUND);
  float cord = sdCapsule(h, vec3(0.0, 0.107, 0.0), vec3(0.17, 0.1, 0.12), 0.007);
  cord = min(cord, sdCapsule(h, vec3(0.17, 0.1, 0.12), vec3(0.17, 0.0, 0.12), 0.007));
  vec2 tassel = M(min(cord, sdRoundCylinder(h - vec3(0.17, -0.03, 0.12), 0.02, 0.035, 0.01)), LIME + ROUND);
  r = U(r, U(U(cap, board), U(knob, tassel)));
  return r;
}
