#pragma frame {"size":[800,640],"center":[0.0,0.5,0.0],"zoom":220,"bounds":[[-1.2,-0.1,-1.2],[1.2,1.8,1.2]]}

vec2 drum(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 3; i++) {
    float y = 0.14 + float(i) * 0.25;
    r = U(r, M(sdCylinder(p - vec3(0.0, y, 0.0), 0.32, 0.1), WHITE + ROUND));
    r = U(r, M(sdCylinder(p - vec3(0.0, y + 0.11, 0.0), 0.3, 0.012), LIME + ROUND));
  }
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.02, 0.0), 0.3, 0.02), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.92, 0.0), 0.012, 0.3), GRAPHITE + ROUND));
  vec3 f = p - vec3(0.0, 1.1, 0.15);
  r = U(r, M(sdBox(f, vec3(0.012, 0.1, 0.15)), LIME));
  return r;
}

vec2 fence(vec3 p) {
  vec3 q = vec3(abs(p.x), p.y, abs(p.z));
  vec3 a = vec3(repC(p.x, 0.18, 11.0), p.y, q.z - 0.9);
  vec3 b = vec3(q.x - 0.9, p.y, repC(p.z, 0.18, 9.0));
  float post = min(sdBox(a - vec3(0.0, 0.14, 0.0), vec3(0.025, 0.14, 0.025)), sdBox(b - vec3(0.0, 0.14, 0.0), vec3(0.025, 0.14, 0.025)));
  float rail = min(sdBox(vec3(p.x, p.y - 0.2, q.z - 0.9), vec3(0.9, 0.018, 0.012)), sdBox(vec3(q.x - 0.9, p.y - 0.2, p.z), vec3(0.012, 0.018, 0.9)));
  vec2 r = M(post, WHITE);
  r = U(r, M(rail, STEEL));
  return r;
}

vec2 key(vec3 p) {
  vec2 r = M(sdTorus(p, 0.09, 0.03), BLUE + ROUND);
  r = U(r, M(sdBox(p - vec3(0.27, 0.0, 0.0), vec3(0.18, 0.02, 0.025)), BLUE));
  r = U(r, M(sdBox(p - vec3(0.4, 0.0, 0.06), vec3(0.025, 0.02, 0.05)), BLUE));
  r = U(r, M(sdBox(p - vec3(0.32, 0.0, 0.05), vec3(0.025, 0.02, 0.04)), BLUE));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(1.0, 0.06, 1.0), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  vec3 g = faceY(p, 0.06);
  r = decal(r, g, sdRect(g.xz, vec2(0.86, 0.86), 0.04), MINT);
  r = U(r, fence(p - vec3(0.0, 0.06, 0.0)));
  r = U(r, drum(p - vec3(-0.1, 0.06, -0.15)));
  r = U(r, key(rotY(p - vec3(0.2, 0.09, 0.5), 0.5)));
  return r;
}
