#pragma frame {"size":[800,640],"center":[0.0,0.6,0.0],"zoom":200,"bounds":[[-1.2,-0.05,-1.0],[1.2,1.7,1.0]]}

vec2 screen(vec3 p, float m) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.08, 0.0), vec3(0.03, 0.08, 0.03)), GRAPHITE);
  r = U(r, M(sdBox(p - vec3(0.0, 0.01, 0.0), vec3(0.12, 0.01, 0.08)), STEEL));
  vec3 s = p - vec3(0.0, 0.36, 0.0);
  r = U(r, M(sdRoundBox(s, vec3(0.24, 0.2, 0.025), 0.012), WHITE));
  vec3 f = faceZ(s, 0.025);
  r = inset(r, f, vec2(0.2, 0.16), 0.015, 0.01, SKY);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.11), vec2(0.18, 0.03), 0.01), m);
  r = decal(r, f, sdRect(f.xz - vec2(-0.05, 0.0), vec2(0.13, 0.014), 0.014), STEEL);
  r = decal(r, f, sdRect(f.xz - vec2(-0.08, -0.07), vec2(0.1, 0.014), 0.014), STEEL);
  return r;
}

vec2 drum(vec3 p) {
  vec2 r = M(1e9, 0.0);
  for (int i = 0; i < 3; i++) {
    float y = 0.2 + float(i) * 0.36;
    r = U(r, M(sdCylinder(p - vec3(0.0, y, 0.0), 0.36, 0.15), WHITE + ROUND));
    r = U(r, M(sdCylinder(p - vec3(0.0, y + 0.165, 0.0), 0.34, 0.015), GRAPHITE + ROUND));
  }
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.06, 0.0), 0.14, 0.03), STEEL + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 1.11, 0.0), 0.05, 0.03), LIME + ROUND));
  return r;
}

vec2 padlock(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.1, 0.085, 0.035), 0.015), LIME);
  r = U(r, M(sdTorus(rotX(p - vec3(0.0, 0.09, 0.0), PI * 0.5).xzy, 0.065, 0.016), STEEL));
  r = U(r, M(sdCylinder(rotX(p - vec3(0.0, -0.01, 0.036), PI * 0.5), 0.018, 0.006), GRAPHITE + ROUND));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.0), vec3(1.1, 0.06, 0.95), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  vec3 w = p - vec3(0.0, 0.5, -0.05);
  r = U(r, M(sdBox(w, vec3(1.05, 0.44, 0.04)), STEEL));
  r = U(r, screw(vec3(abs(faceZ(p, -0.01).x) - 0.95, faceZ(p, -0.01).y, abs(p.y - 0.5) - 0.34), 0.022));
  r = U(r, drum(p - vec3(-0.45, 0.06, -0.5)));
  r = U(r, padlock(p - vec3(0.6, 0.76, 0.03)));
  r = U(r, screen(p - vec3(-0.62, 0.06, 0.45), LIME));
  r = U(r, screen(p - vec3(0.0, 0.06, 0.5), GRAPHITE));
  r = U(r, screen(p - vec3(0.62, 0.06, 0.45), LILAC));
  return r;
}
