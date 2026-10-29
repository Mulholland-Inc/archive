#pragma frame {"size":[800,640],"center":[0.0,0.2,0.08],"zoom":250,"bounds":[[-1.2,-0.1,-1.0],[1.2,0.9,1.2]]}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float k = 1e9;
  vec3 prev = a;
  for (int i = 1; i <= 20; i++) {
    float t = float(i) / 20.0;
    vec3 ab = mix(a, b, t), bc = mix(b, c, t), cd = mix(c, d, t);
    vec3 q = mix(mix(ab, bc, t), mix(bc, cd, t), t);
    k = min(k, sdCapsule(p, prev, q, r));
    prev = q;
  }
  return k;
}

vec2 table(vec3 p, float cols, float rows, float hl) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.025, 0.0), vec3(cols * 0.12 + 0.03, 0.025, rows * 0.08 + 0.03), 0.02), STEEL);
  float row = repI(p.z, 0.16, rows);
  vec3 c = vec3(repC(p.x, 0.24, cols), p.y - 0.05, repC(p.z, 0.16, rows));
  float m = row == 0.0 ? LILAC : row == hl ? LIME : WHITE;
  float h = 0.035;
  r = U(r, M(sdRoundBox(c - vec3(0.0, h, 0.0), vec3(0.11, h, 0.07), 0.015), m));
  vec3 t = faceY(c, h * 2.0);
  float len = 0.05 + 0.025 * mod(repI(p.x, 0.24, cols) * 3.0 + row * 2.0, 3.0);
  r = decal(r, t, sdRect(t.xz - vec2(-0.07 + len, 0.0), vec2(len, 0.016), 0.016), m == WHITE ? STEEL : WHITE);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.0, 0.1), vec3(1.1, 0.06, 0.95), 0.02), WHITE);
  r = seam(r, -(p.y + 0.02));
  vec3 a = p - vec3(-0.55, 0.06, -0.32);
  if (near(a, vec3(0.45, 0.2, 0.5), r.x)) r = U(r, table(a, 3.0, 5.0, 3.0));
  vec3 b = p - vec3(0.42, 0.06, 0.42);
  if (near(b, vec3(0.55, 0.2, 0.5), r.x)) r = U(r, table(b, 4.0, 5.0, 2.0));
  vec3 s = vec3(-0.2, 0.23, -0.16);
  vec3 e = vec3(-0.07, 0.23, 0.42);
  r = U(r, M(cable(p, s, s + vec3(0.0, 0.45, 0.0), e + vec3(0.0, 0.45, 0.0), e, 0.022), LIME + SOLID));
  r = U(r, M(sdSphere(p - s, 0.045), BLUE + SOLID));
  r = U(r, M(sdSphere(p - e, 0.045), BLUE + SOLID));
  return r;
}
