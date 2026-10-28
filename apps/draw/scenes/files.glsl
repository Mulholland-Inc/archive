#pragma frame {"size":[800,640],"center":[0.0,0.92,0.31],"zoom":235,"bounds":[[-1.0,-0.05,-0.8],[1.0,2.4,1.6]]}

vec2 handle(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.05, 0.0), vec3(0.13, 0.016, 0.022), 0.012), STEEL);
  vec3 s = vec3(abs(q.x) - 0.11, q.y - 0.025, q.z);
  r = U(r, M(sdBox(s, vec3(0.014, 0.025, 0.014)), STEEL));
  return r;
}

vec2 label(vec3 q) {
  vec2 r = M(extrude(max(sdRect(q.xz, vec2(0.1, 0.045), 0.008), -sdRect(q.xz, vec2(0.08, 0.028), 0.004)), q.y - 0.006, 0.006), STEEL);
  return r;
}

vec2 front(vec3 p, vec3 c) {
  vec3 q = faceZ(p, c.z) - vec3(0.0, 0.0, c.y);
  vec2 r = M(1e9, 0.0);
  r = U(r, handle(q - vec3(0.0, 0.0, -0.04)));
  r = U(r, label(q - vec3(0.0, 0.0, 0.1)));
  return r;
}

vec2 folder(vec3 p, float m, float tab) {
  p.y -= 0.08;
  vec2 f = M(sdBox(p, vec3(0.32, 0.15, 0.012)), m);
  f = U(f, M(sdRoundBox(p - vec3(tab, 0.17, 0.0), vec3(0.07, 0.035, 0.012), 0.01), m));
  return f;
}

vec2 drawer(vec3 p) {
  vec3 b = p - vec3(0.0, 0.0, -0.1);
  vec2 tray = M(max(sdBox(b, vec3(0.37, 0.18, 0.5)), -sdBox(b - vec3(0.0, 0.05, 0.0), vec3(0.34, 0.2, 0.47))), WHITE);
  vec3 fp = p - vec3(0.0, 0.0, 0.42);
  vec2 face = M(sdRoundBox(fp, vec3(0.41, 0.21, 0.025), 0.012), WHITE);
  vec3 fq = faceZ(fp, 0.025);
  face = inset(face, fq, vec2(0.36, 0.16), 0.02, 0.012, PAPER);
  vec2 r = U(tray, face);
  r = U(r, handle(fq - vec3(0.0, 0.0, -0.04)));
  r = U(r, label(fq - vec3(0.0, 0.0, 0.1)));
  r = U(r, folder(p - vec3(0.0, 0.0, 0.33), CORAL, 0.18));
  r = U(r, folder(p - vec3(0.0, 0.0, 0.25), WHITE, -0.06));
  r = U(r, folder(p - vec3(0.0, 0.0, 0.17), MINT, 0.12));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.01), BLUE, -0.2));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.09), WHITE, 0.2));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.17), LILAC, 0.0));
  r = U(r, folder(p - vec3(0.0, 0.0, -0.25), PINK, -0.15));
  return r;
}

vec2 cabinet(vec3 p) {
  vec3 b = p - vec3(0.0, 0.8, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.45, 0.72, 0.42), 0.02), WHITE);
  vec3 fz = faceZ(p, 0.42);
  r = Sc(r, M(max(sdRect(fz.xz - vec2(0.0, 1.26), vec2(0.41, 0.21), 0.012), -(fz.y + 1.2)), GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.32, 0.43), vec3(0.41, 0.21, 0.02), 0.012), WHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.79, 0.43), vec3(0.41, 0.21, 0.02), 0.012), WHITE));
  vec3 f1 = faceZ(p, 0.45);
  r = inset(r, f1 - vec3(0.0, 0.0, 0.32), vec2(0.36, 0.16), 0.02, 0.012, PAPER);
  r = inset(r, f1 - vec3(0.0, 0.0, 0.79), vec2(0.36, 0.16), 0.02, 0.012, PAPER);
  r = U(r, handle(f1 - vec3(0.0, 0.0, 0.28)));
  r = U(r, label(f1 - vec3(0.0, 0.0, 0.42)));
  r = U(r, handle(f1 - vec3(0.0, 0.0, 0.75)));
  r = U(r, label(f1 - vec3(0.0, 0.0, 0.89)));
  vec3 kick = p - vec3(0.0, 0.04, 0.0);
  r = U(r, M(sdBox(kick, vec3(0.41, 0.04, 0.37)), GRAPHITE));
  vec3 fx = faceX(p, 0.45);
  r = inset(r, fx - vec3(0.0, 0.0, 0.86), vec2(0.3, 0.56), 0.03, 0.012, WHITE);
  r = vents(r, vec3(fx.x, fx.y, repC(fx.z - 0.42, 0.06, 3.0)), 5.0, 0.07, 0.12, 0.028, GRAPHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.24, fx.y, abs(fx.z - 0.86) - 0.48), 0.022));
  r = seam(r, -(abs(b.y) - 0.68));
  vec3 t = faceY(p, 1.52);
  r = U(r, screw(mirXZ(t) - vec3(0.38, 0.0, 0.35), 0.024));
  return r;
}

vec2 sheet(vec3 p, float m) {
  vec2 s = M(sdBox(p, vec3(0.26, 0.14, 0.004)), m);
  vec3 q = faceZ(p, 0.004);
  s = decal(s, q, sdRect(q.xz - vec2(-0.06, 0.08), vec2(0.13, 0.016), 0.016), STEEL);
  s = decal(s, q, sdRect(q.xz - vec2(0.0, 0.03), vec2(0.19, 0.012), 0.012), STEEL);
  return s;
}

vec2 lifted(vec3 p) {
  vec2 r = M(sdBox(p - vec3(0.0, 0.0, -0.03), vec3(0.32, 0.15, 0.012)), SUN);
  r = U(r, M(sdBox(rotX(p - vec3(0.0, -0.15, 0.0), 0.25) - vec3(0.0, 0.15, 0.012), vec3(0.32, 0.15, 0.012)), SUN));
  r = U(r, M(sdRoundBox(p - vec3(0.18, 0.17, -0.03), vec3(0.07, 0.035, 0.012), 0.01), SUN));
  r = U(r, sheet(rotX(p - vec3(0.0, 0.07, 0.0), 0.04), WHITE));
  r = U(r, sheet(rotZ(rotX(p - vec3(0.03, 0.1, 0.02), 0.12), -0.06), WHITE));
  return r;
}

vec2 plant(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.08, 0.0), 0.1, 0.08), CORAL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.155, 0.0), 0.11, 0.02), CORAL + ROUND));
  vec3 l = polarY(p - vec3(0.0, 0.17, 0.0), 5.0);
  float leaf = sdEllipsoid(rotZ(l - vec3(0.1, 0.1, 0.0), -0.7), vec3(0.14, 0.03, 0.05));
  r = U(r, M(min(leaf, sdEllipsoid(p - vec3(0.0, 0.33, 0.0), vec3(0.04, 0.16, 0.04))), MINT + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = cabinet(p);
  r = U(r, drawer(p - vec3(0.0, 1.26, 0.6)));
  r = U(r, lifted(rotY(rotZ(p - vec3(0.05, 1.84, 0.69), 0.1), 0.15)));
  vec2 pl = plant((p - vec3(-0.22, 1.54, -0.14)) / 0.8);
  r = U(r, vec2(pl.x * 0.8, pl.y));
  vec3 s = p - vec3(0.2, 1.56, -0.12);
  r = U(r, M(sdBox(rotY(s, 0.2), vec3(0.17, 0.018, 0.12)), WHITE));
  r = U(r, M(sdBox(rotY(s - vec3(0.0, 0.036, 0.0), -0.15), vec3(0.17, 0.018, 0.12)), BLUE));
  return r;
}
