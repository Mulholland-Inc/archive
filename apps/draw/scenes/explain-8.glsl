#pragma frame {"size":[800,640],"center":[0.0,0.5,0.0],"zoom":220,"bounds":[[-1.3,-0.1,-1.0],[1.3,1.6,1.0]]}

vec2 tank(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.45, 0.0), 0.44, 0.45), WHITE + ROUND);
  r = Sc(r, M(sdCylinder(p - vec3(0.0, 0.9, 0.0), 0.38, 0.4), STEEL + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.45, 0.0), 0.38, 0.36), LIME + ROUND));
  for (int i = 0; i < 2; i++) r = U(r, M(sdCylinder(p - vec3(0.0, 0.3 + float(i) * 0.4, 0.0), 0.455, 0.025), GRAPHITE + ROUND));
  r = U(r, M(max(sdCylinder(p - vec3(0.0, 0.9, 0.0), 0.465, 0.02), 0.38 - length(p.xz)), STEEL + ROUND));
  return r;
}

vec2 chute(vec3 p) {
  vec3 q = rotZ(p, -0.3);
  vec2 r = M(sdBox(q, vec3(0.44, 0.07, 0.24)), WHITE);
  r = Sc(r, M(sdBox(q - vec3(0.0, 0.08, 0.0), vec3(0.52, 0.1, 0.18)), STEEL));
  r = U(r, M(sdBox(q - vec3(0.0, 0.0, 0.0), vec3(0.44, 0.03, 0.18)), LIME));
  vec3 t = faceY(q, 0.03);
  float x = mod(t.x + 0.1, 0.2) - 0.1;
  float chev = abs(x + abs(t.z) * 0.8) - 0.024;
  r = decal(r, t, max(chev, abs(t.z) - 0.13), WHITE);
  return r;
}

vec2 bot(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(0.24, 0.03, 0.24), 0.015), GRAPHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.19, 0.0), vec3(0.16, 0.13, 0.16), 0.04), STEEL));
  vec3 h = p - vec3(0.0, 0.5, 0.0);
  vec2 head = M(sdRoundBox(h, vec3(0.22, 0.18, 0.22), 0.06), WHITE);
  head = Sc(head, M(sdBox(h - vec3(0.0, 0.2, 0.0), vec3(0.15, 0.1, 0.15)), GRAPHITE));
  r = U(r, head);
  r = U(r, M(sdBox(h - vec3(0.0, 0.08, 0.0), vec3(0.15, 0.02, 0.15)), LIME));
  vec3 f = faceX(h, 0.22);
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.0, -0.02), vec2(0.15, 0.07), 0.06), f.y, 0.012), GRAPHITE));
  r = U(r, led(vec3(abs(f.x) - 0.07, f.y - 0.012, f.z + 0.02), 0.032, LIME));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.0), vec3(1.2, 0.06, 0.65), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  r = U(r, tank(p - vec3(-0.65, 0.06, 0.0)));
  r = U(r, chute(p - vec3(0.24, 0.9, 0.0)));
  r = U(r, M(sdBox(p - vec3(0.24, 0.45, 0.0), vec3(0.03, 0.4, 0.03)), GRAPHITE));
  r = U(r, bot(p - vec3(0.72, 0.06, 0.0)));
  return r;
}
