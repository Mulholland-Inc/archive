#pragma frame {"size":[800,640],"center":[0.0,0.55,0.05],"zoom":225,"bounds":[[-1.2,-0.1,-1.1],[1.2,1.5,1.2]]}

vec2 crate(vec3 p, vec3 b, float m) {
  vec2 r = M(sdRoundBox(p, b, 0.02), m);
  r = seam(r, abs(p.y) - b.y + 0.04);
  vec3 f = faceZ(p, b.z);
  r = decal(r, f, sdRect(f.xz, vec2(b.x * 0.45, 0.02), 0.02), m == WHITE ? STEEL : WHITE);
  return r;
}

vec2 press(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(0.5, 0.03, 0.5), 0.015), GRAPHITE);
  vec3 c = mirXZ(p) - vec3(0.44, 0.0, 0.44);
  r = U(r, M(sdBox(c - vec3(0.0, 0.5, 0.0), vec3(0.04, 0.5, 0.04)), GRAPHITE));
  vec3 t = p - vec3(0.0, 1.04, 0.0);
  r = U(r, M(sdRoundBox(t, vec3(0.56, 0.07, 0.56), 0.02), GRAPHITE));
  r = U(r, screw(faceY(c, 1.11), 0.03));
  r = U(r, M(sdBox(t - vec3(0.0, -0.085, 0.0), vec3(0.4, 0.015, 0.4)), STEEL));
  r = U(r, crate(p - vec3(0.0, 0.21, 0.0), vec3(0.36, 0.15, 0.32), WHITE));
  r = U(r, crate(p - vec3(0.0, 0.52, 0.0), vec3(0.36, 0.15, 0.32), WHITE));
  r = U(r, crate(p - vec3(0.0, 0.8, 0.0), vec3(0.36, 0.1, 0.32), WHITE));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.05), vec3(1.1, 0.06, 0.95), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  r = U(r, press(p - vec3(-0.15, 0.06, -0.25)));
  r = U(r, crate(p - vec3(0.75, 0.21, 0.35), vec3(0.18, 0.15, 0.16), LIME));
  r = U(r, crate(p - vec3(0.75, 0.52, 0.35), vec3(0.18, 0.15, 0.16), LIME));
  r = U(r, crate(p - vec3(0.75, 0.21, 0.72), vec3(0.18, 0.15, 0.16), LIME));
  r = U(r, crate(rotY(p - vec3(0.3, 0.21, 0.72), 0.25), vec3(0.18, 0.15, 0.16), LIME));
  return r;
}
