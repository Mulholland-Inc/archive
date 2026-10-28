#pragma frame {"size":[800,640],"center":[0.0,0.45,0.0],"zoom":182,"bounds":[[-1.0,-0.1,-1.0],[1.8,2.2,1.8]]}

vec2 sheet(vec3 p, float m) {
  vec2 s = M(sdBox(p, vec3(0.12, 0.007, 0.16)), m);
  vec3 q = faceY(p, 0.007);
  float bar = m == WHITE ? STEEL : WHITE;
  s = decal(s, q, sdRect(q.xz - vec2(0.0, -0.08), vec2(0.07, 0.01), 0.01), bar);
  s = decal(s, q, sdRect(q.xz - vec2(-0.02, -0.03), vec2(0.05, 0.01), 0.01), bar);
  return decal(s, q, sdRect(q.xz - vec2(0.0, 0.07), vec2(0.07, 0.04), 0.01), bar);
}

vec2 chute(vec3 p, vec3 a, vec3 b, float m) {
  vec3 d = b - a;
  vec3 q = rotY(p - a, -atan(d.z, d.x));
  float L = length(d.xz);
  q = rotZ(q, atan(d.y, L));
  float l = length(d);
  vec3 c = q - vec3(l * 0.5, 0.0, 0.0);
  float body = sdBox(c, vec3(l * 0.5, 0.035, 0.13));
  float cut = sdBox(c - vec3(0.0, 0.04, 0.0), vec3(l * 0.5 + 0.05, 0.04, 0.1));
  vec2 r = M(max(body, -cut), STEEL);
  r = U(r, sheet(rotY(c - vec3(0.05, 0.0, 0.0), PI * 0.5 + 0.1), m));
  return r;
}

vec2 bin(vec3 p, float m) {
  vec3 q = p - vec3(0.0, 0.2, 0.0);
  vec2 c = M(max(sdRoundBox(q, vec3(0.27, 0.2, 0.27), 0.015), -sdBox(q - vec3(0.0, 0.05, 0.0), vec3(0.24, 0.2, 0.24))), WHITE);
  c = seam(c, q.y + 0.12);
  c = paint(c, 0.14 - q.y, m);
  vec3 fx = faceX(q, 0.27);
  c = U(c, screw(fx - vec3(0.17, 0.0, -0.03), 0.018));
  c = U(c, screw(fx - vec3(-0.17, 0.0, -0.03), 0.018));
  vec3 fz = faceZ(q, 0.27);
  c = U(c, screw(fz - vec3(0.17, 0.0, -0.03), 0.018));
  c = U(c, screw(fz - vec3(-0.17, 0.0, -0.03), 0.018));
  for (int i = 0; i < 6; i++) {
    float fi = float(i);
    c = U(c, sheet(rotY(q - vec3(0.02 * sin(fi * 1.7), -0.02 + 0.025 * fi, 0.02 * cos(fi * 2.3)), 0.35 * sin(fi * 2.9)), m));
  }
  c = U(c, sheet(rotX(rotY(q - vec3(-0.02, 0.17, -0.02), 0.5), 0.12), m));
  return c;
}

vec2 sorter(vec3 p) {
  vec3 l = mirXZ(p) - vec3(0.24, 0.32, 0.24);
  vec2 r = M(sdBox(l, vec3(0.025, 0.32, 0.025)), GRAPHITE);
  r = U(r, M(sdBox(vec3(p.x, p.y - 0.2, abs(p.z) - 0.24), vec3(0.24, 0.018, 0.018)), GRAPHITE));
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.24, p.y - 0.2, p.z), vec3(0.018, 0.018, 0.24)), GRAPHITE));
  vec3 b = p - vec3(0.0, 0.84, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.32, 0.2, 0.32), 0.025), WHITE);
  body = seam(body, b.y + 0.08);
  body = seam(body, b.y - 0.15);
  vec3 fx = faceX(p, 0.32);
  body = vents(body, fx - vec3(-0.14, 0.0, 0.9), 4.0, 0.055, 0.1, 0.022, GRAPHITE);
  vec3 fz = faceZ(p, 0.32);
  body = inset(body, fz - vec3(-0.06, 0.0, 0.9), vec2(0.16, 0.05), 0.02, 0.02, GRAPHITE);
  r = U(r, body);
  r = U(r, led(fx - vec3(0.07, 0.0, 0.9), 0.026, SUN));
  r = U(r, led(fx - vec3(0.15, 0.0, 0.9), 0.026, LILAC));
  r = U(r, led(fx - vec3(0.23, 0.0, 0.9), 0.026, CORAL));
  r = U(r, screw(fz - vec3(0.2, 0.0, 0.9), 0.018));
  r = U(r, M(sdBox(fz - vec3(-0.06, 0.0, 0.9), vec3(0.14, 0.004, 0.006)), LIME + SOLID));
  vec3 h = p - vec3(0.0, 1.06, 0.0);
  r = U(r, M(sdCylinder(h - vec3(0.0, 0.04, 0.0), 0.12, 0.05), STEEL + ROUND));
  vec3 cq = h - vec3(0.0, 0.32, 0.0);
  float outer = sdCone(cq, 0.22, 0.12, 0.42);
  float inner = sdCone(cq - vec3(0.0, 0.03, 0.0), 0.22, 0.09, 0.39);
  r = U(r, M(max(outer, -inner), WHITE + SOLID));
  r = U(r, M(sdTorus(cq - vec3(0.0, 0.22, 0.0), 0.42, 0.025), BLUE + SOLID));
  r = U(r, sheet(rotZ(rotY(cq - vec3(0.06, 0.04, 0.02), 0.6), 0.45), CORAL));
  r = U(r, sheet(rotX(rotY(cq - vec3(-0.1, 0.07, -0.06), -0.3), -0.5), SUN));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.85, 0.0), vec3(0.5, 0.85, 0.5), r.x)) r = U(r, sorter(p));
  vec3 b1 = p - vec3(0.0, 0.0, 1.3);
  vec3 b2 = p - vec3(0.95, 0.0, 0.95);
  vec3 b3 = p - vec3(1.3, 0.0, 0.0);
  if (near(b1 - vec3(0.0, 0.22, 0.0), vec3(0.32, 0.25, 0.32), r.x)) r = U(r, bin(b1, SUN));
  if (near(b2 - vec3(0.0, 0.22, 0.0), vec3(0.32, 0.25, 0.32), r.x)) r = U(r, bin(b2, LILAC));
  if (near(b3 - vec3(0.0, 0.22, 0.0), vec3(0.32, 0.25, 0.32), r.x)) r = U(r, bin(b3, CORAL));
  if (near(p - vec3(0.6, 0.6, 0.6), vec3(0.9, 0.2, 0.9), r.x)) {
    r = U(r, chute(p, vec3(0.0, 0.7, 0.3), vec3(0.0, 0.5, 1.2), SUN));
    r = U(r, chute(p, vec3(0.22, 0.7, 0.22), vec3(0.88, 0.5, 0.88), LILAC));
    r = U(r, chute(p, vec3(0.3, 0.7, 0.0), vec3(1.2, 0.5, 0.0), CORAL));
  }
  if (near(p - vec3(0.0, 1.9, 0.0), vec3(0.45, 0.4, 0.45), r.x)) {
    r = U(r, sheet(rotZ(rotX(rotY(p - vec3(0.1, 1.7, 0.12), 0.4), 0.35), -0.25), LILAC));
    r = U(r, sheet(rotX(rotZ(rotY(p - vec3(-0.18, 1.92, -0.14), -0.7), 0.25), 0.2), WHITE));
    r = U(r, sheet(rotZ(rotX(rotY(p - vec3(0.14, 2.08, -0.22), 1.2), -0.3), 0.2), SUN));
    r = U(r, sheet(rotX(rotY(p - vec3(-0.12, 1.8, 0.24), 2.0), 0.45), CORAL));
  }
  return r;
}
