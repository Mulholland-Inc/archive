#pragma frame {"size":[800,640],"center":[0.0,0.55,0.05],"zoom":215,"bounds":[[-1.2,-0.1,-1.1],[1.2,1.8,1.2]]}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float k = 1e9;
  vec3 prev = a;
  for (int i = 1; i <= 24; i++) {
    float t = float(i) / 24.0;
    vec3 ab = mix(a, b, t), bc = mix(b, c, t), cd = mix(c, d, t);
    vec3 q = mix(mix(ab, bc, t), mix(bc, cd, t), t);
    k = min(k, sdCapsule(p, prev, q, r));
    prev = q;
  }
  return k;
}

vec2 tank(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.5, 0.0), 0.56, 0.5), WHITE + ROUND);
  r = Sc(r, M(sdCylinder(p - vec3(0.0, 1.0, 0.0), 0.5, 0.42), STEEL + ROUND));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.5, 0.0), 0.5, 0.36), LIME + ROUND));
  for (int i = 0; i < 2; i++) r = U(r, M(sdCylinder(p - vec3(0.0, 0.3 + float(i) * 0.4, 0.0), 0.575, 0.025), GRAPHITE + ROUND));
  r = U(r, M(max(sdCylinder(p - vec3(0.0, 1.0, 0.0), 0.585, 0.02), 0.5 - length(p.xz)), STEEL + ROUND));
  return r;
}

vec2 bot(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(0.2, 0.03, 0.2), 0.015), GRAPHITE);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.17, 0.0), vec3(0.13, 0.11, 0.13), 0.04), STEEL));
  vec3 h = p - vec3(0.0, 0.42, 0.0);
  r = U(r, M(sdRoundBox(h, vec3(0.17, 0.14, 0.17), 0.05), WHITE));
  vec3 f = faceX(h, 0.17);
  r = U(r, M(extrude(sdRect(f.xz - vec2(0.0, 0.05), vec2(0.12, 0.05), 0.05), f.y, 0.012), GRAPHITE));
  r = U(r, led(vec3(abs(f.x) - 0.055, f.y - 0.012, f.z - 0.05), 0.024, LILAC));
  r = U(r, M(sdCylinder(rotZ(f - vec3(0.0, 0.0, -0.07), 0.0), 0.03, 0.012), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.2, 0.0), 0.012, 0.07), GRAPHITE + ROUND));
  r = U(r, M(sdSphere(h - vec3(0.0, 0.28, 0.0), 0.035), LIME + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.05), vec3(1.1, 0.06, 0.95), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  r = U(r, tank(p - vec3(0.2, 0.06, -0.35)));
  r = U(r, bot(p - vec3(-0.55, 0.06, 0.5)));
  vec3 a = vec3(0.15, 0.8, -0.2);
  vec3 v = vec3(0.15, 1.55, -0.2);
  vec3 d = vec3(-0.36, 0.42, 0.5);
  r = U(r, M(min(sdCapsule(p, a, v, 0.016), cable(p, v, v + vec3(0.0, 0.2, 0.0), d + vec3(0.35, 0.25, 0.0), d, 0.016)), WHITE + SOLID));
  return r;
}
