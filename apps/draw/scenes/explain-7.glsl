#pragma frame {"size":[800,640],"center":[0.0,0.45,0.1],"zoom":225,"bounds":[[-1.3,-0.1,-1.1],[1.3,1.6,1.3]]}

vec2 grid(vec3 p) {
  float col = repI(p.x, 0.26, 5.0);
  float row = repI(p.z, 0.18, 5.0);
  vec3 c = vec3(repC(p.x, 0.26, 5.0), p.y, repC(p.z, 0.18, 5.0));
  bool hit = (row == 1.0 && col > 0.0) || (row == 3.0 && col < 3.0) || (row == 2.0 && col == 2.0);
  float h = 0.035;
  float m = hit ? LIME : WHITE;
  vec2 r = M(sdRoundBox(c - vec3(0.0, h, 0.0), vec3(0.115, h, 0.075), 0.015), m);
  vec3 t = faceY(c, 2.0 * h);
  float len = 0.04 + 0.025 * mod(col * 3.0 + row * 2.0, 3.0);
  r = decal(r, t, sdRect(t.xz - vec2(-0.08 + len, 0.0), vec2(len, 0.016), 0.016), hit ? WHITE : STEEL);
  return r;
}

vec2 screen(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.05, 0.0), vec3(0.3, 0.05, 0.1), 0.02), GRAPHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.18, -0.02), vec3(0.04, 0.12, 0.03)), GRAPHITE));
  vec3 s = rotX(p - vec3(0.0, 0.62, 0.0), 0.12);
  r = U(r, M(sdRoundBox(s, vec3(0.62, 0.36, 0.03), 0.02), GRAPHITE));
  vec3 f = faceZ(s, 0.03);
  r = inset(r, f, vec2(0.56, 0.3), 0.02, 0.01, GRAPHITE);
  f.y += 0.01;
  float lines[8] = float[](0.3, 0.42, 0.24, 0.5, 0.36, 0.28, 0.44, 0.2);
  float ind[8] = float[](0.0, 0.08, 0.16, 0.16, 0.08, 0.16, 0.16, 0.0);
  for (int i = 0; i < 8; i++) {
    float y = 0.24 - float(i) * 0.068;
    float x0 = -0.48 + ind[i];
    float l = lines[i];
    float m = i == 0 || i == 4 ? LIME : i == 2 || i == 6 ? LILAC : i == 7 ? STEEL : WHITE;
    r = decal(r, f, sdRect(f.xz - vec2(x0 + l * 0.5, y), vec2(l * 0.5, 0.017), 0.017), m);
  }
  r = decal(r, f, sdRect(f.xz - vec2(-0.27, -0.24), vec2(0.012, 0.024), 0.004), LIME);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.1), vec3(1.05, 0.06, 0.9), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  vec3 t = p - vec3(0.0, 0.06, 0.35);
  r = U(r, M(sdRoundBox(t - vec3(0.0, 0.025, -0.2), vec3(0.7, 0.025, 0.68), 0.02), STEEL));
  if (near(t - vec3(0.0, 0.2, 0.0), vec3(0.7, 0.2, 0.48), r.x)) r = U(r, grid(t - vec3(0.0, 0.05, 0.0)));
  r = U(r, screen(p - vec3(0.0, 0.11, -0.38)));
  vec3 up = t - vec3(-0.26, 0.05, 0.09);
  vec2 lc = M(sdRoundBox(up - vec3(0.0, 0.2, 0.0), vec3(0.115, 0.035, 0.075), 0.015), LIME);
  vec3 lt = faceY(up, 0.235);
  lc = decal(lc, lt, sdRect(lt.xz, vec2(0.08, 0.016), 0.016), WHITE);
  r = U(r, lc);
  return r;
}
