#pragma frame {"size":[800,640],"center":[0.05,0.6,0.35],"zoom":225,"bounds":[[-0.9,-0.05,-0.8],[1.0,1.4,1.8]]}

vec2 drum(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 3; i++) {
    float y = 0.17 + float(i) * 0.27;
    r = U(r, M(sdCylinder(p - vec3(0.0, y, 0.0), 0.3, 0.11), WHITE + ROUND));
    r = U(r, M(sdCylinder(p - vec3(0.0, y + 0.12, 0.0), 0.28, 0.012), LIME + ROUND));
  }
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.03, 0.0), 0.28, 0.03), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.85, 0.0), 0.12, 0.03), LIME + ROUND));
  return r;
}

vec2 body(vec3 p) {
  vec3 b = p - vec3(0.0, 0.5, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.4, 0.46, 0.46), 0.03), WHITE);
  r = Sc(r, M(sdBox(b - vec3(0.08, 0.08, 0.0), vec3(0.4, 0.46, 0.4)), STEEL));
  r = U(r, drum(p - vec3(0.02, 0.06, 0.0)));
  r = U(r, M(sdBox(p - vec3(0.0, 0.02, 0.0), vec3(0.42, 0.02, 0.48)), GRAPHITE));
  return r;
}

vec2 screen(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.025, 0.46, 0.46), 0.015), WHITE);
  vec3 f = faceX(p, 0.025);
  r = inset(r, f, vec2(0.4, 0.4), 0.02, 0.01, SKY);
  f.y += 0.01;
  vec2 u = vec2(-f.x, f.z);
  r = decal(r, f, sdRect(u - vec2(0.0, 0.32), vec2(0.38, 0.06), 0.015), LIME);
  r = decal(r, f, sdCircle(u - vec2(-0.3, 0.32), 0.03), WHITE);
  r = decal(r, f, sdRect(u - vec2(0.2, 0.32), vec2(0.12, 0.014), 0.014), WHITE);
  r = decal(r, f, sdRect(u - vec2(-0.28, -0.07), vec2(0.09, 0.27), 0.015), WHITE);
  for (int i = 0; i < 3; i++) {
    vec2 c = u - vec2(0.1, 0.16 - float(i) * 0.15);
    r = decal(r, f, sdRect(c, vec2(0.25, 0.055), 0.015), WHITE);
    r = decal(r, f, sdCircle(c - vec2(-0.18, 0.0), 0.03), i == 0 ? LILAC : STEEL);
    r = decal(r, f, sdRect(c - vec2(0.03, 0.0), vec2(0.13, 0.013), 0.013), STEEL);
  }
  r = decal(r, f, sdRect(u - vec2(0.22, -0.3), vec2(0.12, 0.045), 0.045), LIME);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.05, 0.0, 0.45), vec3(0.85, 0.06, 1.2), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  r = U(r, body(p - vec3(-0.2, 0.06, -0.15)));
  r = U(r, M(sdBox(p - vec3(0.27, 0.09, 0.42), vec3(0.04, 0.03, 1.0)), STEEL));
  r = U(r, screen(p - vec3(0.27, 0.58, 0.68)));
  return r;
}
