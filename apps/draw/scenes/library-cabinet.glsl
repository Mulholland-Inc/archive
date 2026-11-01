#pragma frame {"size":[1000,900],"center":[0.1,0.98,0.2],"zoom":250,"bounds":[[-1.35,-0.05,-0.6],[1.6,2.3,0.7]]}

vec2 handle(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.05, 0.0), vec3(0.12, 0.016, 0.022), 0.012), STEEL);
  vec3 s = vec3(abs(q.x) - 0.1, q.y - 0.025, q.z);
  r = U(r, M(sdBox(s, vec3(0.014, 0.025, 0.014)), STEEL));
  return r;
}

vec2 label(vec3 q) {
  return M(extrude(max(sdRect(q.xz, vec2(0.09, 0.04), 0.008), -sdRect(q.xz, vec2(0.072, 0.024), 0.004)), q.y - 0.006, 0.006), STEEL);
}

vec2 archiveBox(vec3 p, float m) {
  vec2 r = M(sdBox(p, vec3(0.21, 0.17, 0.25)), m);
  r = U(r, M(sdBox(p - vec3(0.0, 0.14, 0.0), vec3(0.22, 0.045, 0.26)), m));
  vec3 f = faceZ(p, 0.25);
  r = inset(r, f - vec3(0.0, 0.0, -0.03), vec2(0.11, 0.06), 0.01, 0.008, WHITE);
  r = inset(r, f - vec3(0.0, 0.0, 0.135), vec2(0.07, 0.016), 0.016, 0.02, GRAPHITE);
  return r;
}

vec2 binder(vec3 p, float m) {
  vec2 r = M(sdRoundBox(p, vec3(0.045, 0.22, 0.24), 0.01), m);
  vec3 f = faceZ(p, 0.24);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.08), vec2(0.026, 0.05), 0.006), WHITE);
  r = U(r, M(sdTorus(rotX(p - vec3(0.0, -0.11, 0.245), PI * 0.5), 0.022, 0.007), STEEL + ROUND));
  return r;
}

vec2 album(vec3 p, float m) {
  vec2 r = M(sdRoundBox(p, vec3(0.2, 0.04, 0.23), 0.012), m);
  vec3 t = faceY(p, 0.04);
  r = decal(r, t, sdRect(t.xz - vec2(0.02, 0.0), vec2(0.11, 0.13), 0.01), WHITE);
  r = decal(r, t, sdRect(t.xz - vec2(0.02, 0.0), vec2(0.08, 0.1), 0.006), SKY);
  return r;
}

vec2 record(vec3 p) {
  vec2 r = M(sdBox(p, vec3(0.2, 0.2, 0.01)), LILAC);
  vec3 f = faceZ(p, 0.01);
  r = decal(r, f, sdCircle(f.xz, 0.11), WHITE);
  r = decal(r, f, sdCircle(f.xz, 0.035), LILAC);
  vec3 d = rotX(p - vec3(0.1, 0.02, -0.024), PI * 0.5);
  r = U(r, M(sdCylinder(d, 0.19, 0.008), GRAPHITE + ROUND));
  return r;
}

vec2 tape(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.14, 0.09, 0.022), 0.01), GRAPHITE);
  vec3 f = faceZ(p, 0.022);
  r = decal(r, f, sdRect(f.xz - vec2(0.0, 0.035), vec2(0.11, 0.03), 0.006), WHITE);
  r = inset(r, f - vec3(0.0, 0.0, -0.025), vec2(0.07, 0.025), 0.02, 0.01, STEEL);
  vec3 h = vec3(abs(f.x) - 0.045, f.y, f.z + 0.025);
  r = inset(r, h, vec2(0.016, 0.016), 0.016, 0.014, WHITE);
  return r;
}

vec2 can(vec3 p, float m) {
  vec2 r = M(sdCylinder(p, 0.13, 0.025), m + ROUND);
  return r;
}

vec2 shelves(vec3 p) {
  vec3 b = p - vec3(-0.68, 0.95, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.58, 0.95, 0.34), 0.02), WHITE);
  r = Sc(r, M(sdBox(b - vec3(0.0, -0.585, 0.06), vec3(0.53, 0.265, 0.34)), PAPER));
  r = Sc(r, M(sdBox(b - vec3(0.0, 0.01, 0.06), vec3(0.53, 0.27, 0.34)), PAPER));
  r = Sc(r, M(sdBox(b - vec3(0.0, 0.61, 0.06), vec3(0.53, 0.27, 0.34)), PAPER));
  r = U(r, archiveBox(p - vec3(-0.95, 0.27, 0.03), SUN));
  r = U(r, archiveBox(p - vec3(-0.45, 0.27, 0.03), STEEL));
  r = U(r, binder(p - vec3(-1.13, 0.91, 0.04), BLUE));
  r = U(r, binder(p - vec3(-1.03, 0.91, 0.04), WHITE));
  r = U(r, binder(p - vec3(-0.93, 0.91, 0.04), LILAC));
  r = U(r, binder(rotZ(p - vec3(-0.79, 0.7, 0.04), -0.32) - vec3(0.0, 0.22, 0.0), GRAPHITE));
  r = U(r, album(rotY(p - vec3(-0.4, 0.725, 0.05), 0.08), CORAL));
  r = U(r, album(rotY(p - vec3(-0.41, 0.805, 0.05), -0.1), LIME));
  r = U(r, record(rotX(p - vec3(-0.98, 1.47, 0.0), -0.2) / 0.85) * vec2(0.85, 1.0));
  r = U(r, tape(rotY(p - vec3(-0.66, 1.38, 0.18), 0.25)));
  r = U(r, can(p - vec3(-0.47, 1.315, 0.1), STEEL));
  r = U(r, can(p - vec3(-0.46, 1.37, 0.11), SUN));
  return r;
}

vec2 card(vec3 p, float m, float tab) {
  vec2 c = M(sdBox(p, vec3(0.28, 0.11, 0.005)), m);
  if (tab > -1.0) c = U(c, M(sdRoundBox(p - vec3(tab, 0.13, 0.0), vec3(0.06, 0.03, 0.005), 0.008), m));
  return c;
}

vec2 drawer(vec3 p) {
  vec3 b = p - vec3(0.0, 0.0, -0.1);
  vec2 tray = M(max(sdBox(b, vec3(0.34, 0.15, 0.46)), -sdBox(b - vec3(0.0, 0.05, 0.0), vec3(0.31, 0.17, 0.43))), WHITE);
  vec3 fp = p - vec3(0.0, 0.0, 0.38);
  vec2 face = M(sdRoundBox(fp, vec3(0.38, 0.18, 0.025), 0.012), WHITE);
  vec3 fq = faceZ(fp, 0.025);
  face = inset(face, fq, vec2(0.33, 0.13), 0.02, 0.012, PAPER);
  vec2 r = U(tray, face);
  r = U(r, handle(fq - vec3(0.0, 0.0, -0.04)));
  r = U(r, label(fq - vec3(0.0, 0.0, 0.08)));
  vec3 cq = p - vec3(0.0, -0.02, -0.05);
  cq.z = repC(cq.z, 0.034, 16.0);
  if (abs(p.z + 0.05) < 0.3) r = U(r, card(cq, WHITE, -2.0));
  r = U(r, card(p - vec3(0.0, 0.0, 0.25), BLUE, 0.17));
  r = U(r, card(p - vec3(0.0, 0.0, 0.08), LILAC, -0.02));
  r = U(r, card(p - vec3(0.0, 0.0, -0.13), SUN, -0.19));
  r = U(r, card(p - vec3(0.0, 0.0, -0.33), BLUE, 0.1));
  return r;
}

vec2 liftedCard(vec3 p) {
  vec2 c = M(sdBox(p, vec3(0.28, 0.17, 0.005)), WHITE);
  vec3 f = faceZ(p, 0.005);
  c = decal(c, f, sdRect(f.xz - vec2(-0.08, 0.1), vec2(0.15, 0.018), 0.018), LIME);
  c = decal(c, f, sdRect(f.xz - vec2(0.0, 0.04), vec2(0.22, 0.012), 0.012), STEEL);
  c = decal(c, f, sdRect(f.xz - vec2(-0.04, -0.01), vec2(0.18, 0.012), 0.012), STEEL);
  c = decal(c, f, sdRect(f.xz - vec2(-0.08, -0.06), vec2(0.14, 0.012), 0.012), STEEL);
  return c;
}

vec2 plant(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.08, 0.0), 0.1, 0.08), CORAL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.155, 0.0), 0.11, 0.02), CORAL + ROUND));
  vec3 l = polarY(p - vec3(0.0, 0.17, 0.0), 5.0);
  float leaf = sdEllipsoid(rotZ(l - vec3(0.1, 0.1, 0.0), -0.7), vec3(0.14, 0.03, 0.05));
  r = U(r, M(min(leaf, sdEllipsoid(p - vec3(0.0, 0.33, 0.0), vec3(0.04, 0.16, 0.04))), MINT + SOLID));
  return r;
}

vec2 cabinet(vec3 p) {
  vec3 b = p - vec3(0.5, 0.68, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.42, 0.6, 0.38), 0.02), WHITE);
  vec3 fz = faceZ(p, 0.38);
  r = Sc(r, M(max(sdRect(fz.xz - vec2(0.5, 1.04), vec2(0.38, 0.18), 0.012), -(fz.y + 0.7)), GRAPHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.5, 0.31, 0.39), vec3(0.38, 0.18, 0.02), 0.012), WHITE));
  r = U(r, M(sdRoundBox(p - vec3(0.5, 0.68, 0.39), vec3(0.38, 0.16, 0.02), 0.012), WHITE));
  vec3 f1 = faceZ(p, 0.41) - vec3(0.5, 0.0, 0.0);
  r = inset(r, f1 - vec3(0.0, 0.0, 0.31), vec2(0.33, 0.13), 0.02, 0.012, PAPER);
  r = inset(r, f1 - vec3(0.0, 0.0, 0.68), vec2(0.33, 0.11), 0.02, 0.012, PAPER);
  r = U(r, handle(f1 - vec3(0.0, 0.0, 0.27)));
  r = U(r, label(f1 - vec3(0.0, 0.0, 0.39)));
  r = U(r, handle(f1 - vec3(0.0, 0.0, 0.64)));
  r = U(r, label(f1 - vec3(0.0, 0.0, 0.76)));
  r = U(r, M(sdBox(p - vec3(0.5, 0.04, 0.0), vec3(0.38, 0.04, 0.34)), GRAPHITE));
  vec3 fx = faceX(p, 0.92);
  r = inset(r, fx - vec3(0.0, 0.0, 0.7), vec2(0.28, 0.46), 0.03, 0.012, WHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.22, fx.y, abs(fx.z - 0.7) - 0.38), 0.022));
  r = seam(r, -(abs(b.y) - 0.56));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = shelves(p);
  vec3 q = p - vec3(0.34, 0.0, 0.0);
  vec3 c = vec3(q.z + 0.5, q.y, q.x);
  r = U(r, cabinet(c));
  r = U(r, drawer(c - vec3(0.5, 1.04, 0.52)));
  r = U(r, liftedCard(rotX(rotY(c - vec3(0.52, 1.3, 0.62), 0.12), -0.12)));
  vec2 pl = plant((c - vec3(0.3, 1.28, -0.16)) / 0.75);
  r = U(r, vec2(pl.x * 0.75, pl.y));
  vec3 s = c - vec3(0.7, 1.3, -0.12);
  r = U(r, M(sdBox(rotY(s, 0.2), vec3(0.16, 0.018, 0.12)), WHITE));
  r = U(r, M(sdBox(rotY(s - vec3(0.0, 0.036, 0.0), -0.15), vec3(0.16, 0.018, 0.12)), BLUE));
  return r;
}
