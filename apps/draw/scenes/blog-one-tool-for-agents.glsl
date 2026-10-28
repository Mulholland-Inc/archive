#pragma frame {"size":[1400,760],"center":[0.0,0.9,0.0],"zoom":235,"bounds":[[-3.0,-0.1,-3.0],[3.0,2.4,3.0]]}

vec2 card(vec3 p, float m) {
  vec2 c = M(sdRoundBox(p, vec3(0.12, 0.008, 0.085), 0.005), WHITE);
  vec3 q = faceY(p, 0.008);
  c = decal(c, q, sdRect(q.xz - vec2(-0.055, 0.0), vec2(0.035, 0.055), 0.01), m);
  c = decal(c, q, sdRect(q.xz - vec2(0.05, -0.035), vec2(0.04, 0.009), 0.009), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.04, 0.0), vec2(0.03, 0.009), 0.009), STEEL);
  return decal(c, q, sdRect(q.xz - vec2(0.05, 0.035), vec2(0.04, 0.009), 0.009), STEEL);
}

vec2 gadget(vec3 q, float k) {
  vec2 r = M(sdCylinder(vec3(q.x, q.z - 0.03, q.y - 0.07), 0.009, 0.03), GRAPHITE + SOLID);
  if (k == 0.0) {
    r = U(r, M(sdCapsule(q, vec3(0.0, 0.06, 0.05), vec3(0.0, -0.04, 0.05), 0.012), STEEL + SOLID));
    r = U(r, M(sdEllipsoid(q - vec3(0.0, -0.08, 0.05), vec3(0.03, 0.045, 0.012)), WHITE + SOLID));
  } else if (k == 1.0) {
    r = U(r, M(sdRoundBox(q - vec3(0.0, -0.02, 0.05), vec3(0.04, 0.07, 0.016), 0.012), WHITE));
    r = U(r, button(vec3(q.x, q.z - 0.068, q.y + 0.0), 0.016, 0.006, SUN));
  } else if (k == 2.0) {
    r = U(r, M(sdCapsule(q, vec3(0.0, 0.06, 0.05), vec3(0.0, -0.02, 0.05), 0.01), STEEL + SOLID));
    r = U(r, M(sdTorus(vec3(q.x, q.z - 0.05, q.y + 0.06), 0.035, 0.009), STEEL + SOLID));
  } else if (k == 3.0) {
    r = U(r, M(sdCapsule(q, vec3(0.0, 0.06, 0.05), vec3(0.0, -0.01, 0.05), 0.012), WHITE + SOLID));
    r = U(r, M(sdRoundBox(q - vec3(0.0, -0.07, 0.05), vec3(0.035, 0.05, 0.006), 0.005), STEEL));
  } else {
    r = U(r, M(sdCapsule(q, vec3(0.0, 0.06, 0.05), vec3(0.0, 0.0, 0.05), 0.01), STEEL + SOLID));
    r = U(r, M(sdEllipsoid(q - vec3(0.0, -0.07, 0.05), vec3(0.035, 0.065, 0.035)), SUN + SOLID));
  }
  return r;
}

vec2 pegboard(vec3 p) {
  vec3 b = p - vec3(0.0, 0.85, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.85, 0.5, 0.025), 0.012), WHITE);
  vec3 f = faceZ(b, 0.025);
  r = grille(r, f, vec2(17.0, 10.0), 0.095, 0.011, GRAPHITE);
  r = U(r, M(sdBox(vec3(abs(p.x) - 0.7, p.y - 0.2, p.z + 0.06), vec3(0.03, 0.2, 0.03)), STEEL));
  r = U(r, M(sdRoundBox(vec3(abs(p.x) - 0.7, p.y - 0.02, p.z), vec3(0.06, 0.02, 0.24), 0.01), STEEL));
  r = U(r, M(sdBox(vec3(p.x, p.y - 1.38, p.z + 0.01), vec3(0.88, 0.03, 0.04)), STEEL));
  vec3 t = vec3(repC(b.x, 0.19, 8.0), repC(b.y, 0.3, 3.0), b.z - 0.025);
  float ci = repI(b.x, 0.19, 8.0);
  float ri = repI(b.y, 0.3, 3.0);
  r = U(r, gadget(t, mod(ci + ri * 3.0, 5.0)));
  return r;
}

vec2 clipboard(vec3 p) {
  vec3 b = p - vec3(0.0, 0.03, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.3, 0.03, 0.22), 0.012), STEEL);
  vec3 s = rotX(p - vec3(0.0, 0.06, -0.14), 1.05) - vec3(0.0, 0.0, -0.32);
  vec2 bd = M(sdRoundBox(s, vec3(0.26, 0.018, 0.33), 0.02), STEEL);
  r = U(r, bd);
  vec3 sh = s - vec3(0.0, 0.026, 0.02);
  vec2 pa = M(sdBox(sh, vec3(0.22, 0.006, 0.28)), WHITE);
  vec3 q = faceY(sh, 0.006);
  for (int i = 0; i < 6; i++) {
    float fi = float(i);
    float z = -0.2 + fi * 0.075;
    float w = 0.12 + 0.05 * sin(fi * 2.3);
    float ind = mod(fi, 3.0) == 0.0 ? 0.0 : 0.04;
    pa = decal(pa, q, sdRect(q.xz - vec2(-0.08 + ind + w * 0.5, z), vec2(w * 0.5, 0.012), 0.012), GRAPHITE);
  }
  r = U(r, pa);
  vec3 c = s - vec3(0.0, 0.04, -0.33);
  r = U(r, M(sdRoundBox(c, vec3(0.1, 0.025, 0.04), 0.012), LIME));
  r = U(r, M(sdCylinder(vec3(c.y, c.x, c.z) - vec3(0.03, 0.0, 0.0), 0.012, 0.08), GRAPHITE + SOLID));
  return r;
}

vec2 arm(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.05, 0.0), 0.26, 0.05, 0.015), WHITE + ROUND);
  r = U(r, screw(faceY(polarY(p, 6.0), 0.1) - vec3(0.2, 0.0, 0.0), 0.018));
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.2, 0.0), 0.15, 0.1), STEEL + ROUND));
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.32, 0.0), 0.18, 0.035, 0.01), GRAPHITE + ROUND));
  vec3 a = vec3(0.0, 0.45, 0.0), b = vec3(0.1, 1.2, 0.12), c = vec3(0.65, 1.0, -0.55);
  r = U(r, M(sdRoundBox(p - vec3(0.0, 0.42, 0.0), vec3(0.12, 0.06, 0.12), 0.03), WHITE));
  r = U(r, M(sdCapsule(p, a, b, 0.07), WHITE + SOLID));
  r = U(r, M(sdCapsule(p, b, c, 0.058), WHITE + SOLID));
  r = U(r, M(sdSphere(p - a - vec3(0.0, 0.04, 0.0), 0.1), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(p - b, 0.09), GRAPHITE + SOLID));
  r = U(r, M(sdSphere(p - c, 0.075), GRAPHITE + SOLID));
  r = U(r, M(sdCylinder(p - c - vec3(0.0, -0.1, 0.0), 0.05, 0.04), STEEL + ROUND));
  vec3 g = p - c - vec3(0.0, -0.17, 0.0);
  r = U(r, M(sdBox(g, vec3(0.1, 0.02, 0.05)), GRAPHITE));
  r = U(r, M(sdBox(vec3(abs(g.x) - 0.09, g.y + 0.07, g.z), vec3(0.012, 0.07, 0.04)), GRAPHITE));
  r = U(r, card(rotZ(g - vec3(0.0, -0.13, 0.0), 0.1), LIME));
  r = U(r, led(faceZ(p - vec3(0.0, 0.42, 0.0), 0.12), 0.022, LIME));
  return r;
}

vec2 tray(vec3 p, float w) {
  vec3 q = p - vec3(0.0, 0.09, 0.0);
  vec2 c = M(max(sdRoundBox(q, vec3(w, 0.09, 0.24), 0.012), -sdBox(q - vec3(0.0, 0.05, 0.0), vec3(w - 0.025, 0.09, 0.215))), WHITE);
  c = seam(c, q.y + 0.05);
  vec3 f = faceZ(q, 0.24);
  c = U(c, M(extrude(sdRect(f.xz - vec2(0.0, 0.0), vec2(0.07, 0.03), 0.006), f.y - 0.004, 0.004), STEEL + SOLID));
  return c;
}

vec2 bin(vec3 p, float k) {
  vec2 r = tray(p, 0.24);
  for (int i = 0; i < 4; i++) {
    float fi = float(i);
    vec3 q = rotY(p - vec3(0.0, 0.05 + fi * 0.022, 0.0), 0.08 * sin(fi * 2.7 + k));
    r = U(r, card(q, k == 1.0 ? LIME : k == 0.0 ? STEEL : SUN));
  }
  return r;
}

vec2 inbox(vec3 p) {
  vec2 r = tray(p, 0.3);
  float ms[6] = float[6](STEEL, SUN, WHITE, LILAC, STEEL, SUN);
  for (int i = 0; i < 6; i++) {
    float fi = float(i);
    vec3 q = p - vec3(0.12 * sin(fi * 2.1), 0.06 + fi * 0.022, 0.06 * cos(fi * 1.7));
    q = rotY(q, 0.5 * sin(fi * 3.1));
    r = U(r, card(q, ms[i]));
  }
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  vec3 pb = (p - vec3(-1.3, 0.0, -1.0)) / 0.8;
  if (near(pb - vec3(0.0, 0.75, 0.0), vec3(0.95, 0.75, 0.3), r.x)) r = U(r, pegboard(pb) * vec2(0.8, 1.0));
  vec3 cb = rotY(p - vec3(-0.95, 0.0, 1.0), 0.0);
  if (near(cb - vec3(0.0, 0.25, 0.0), vec3(0.35, 0.35, 0.35), r.x)) r = U(r, clipboard(cb));
  vec3 ap = p - vec3(0.3, 0.0, 0.25);
  if (near(ap - vec3(0.2, 0.7, -0.1), vec3(0.55, 0.75, 0.45), r.x)) r = U(r, arm(ap));
  vec3 ip = p - vec3(0.25, 0.0, 1.35);
  if (near(ip - vec3(0.0, 0.15, 0.0), vec3(0.35, 0.2, 0.28), r.x)) r = U(r, inbox(ip));
  vec3 b1 = p - vec3(1.25, 0.0, -0.9);
  if (near(b1 - vec3(0.0, 0.12, 0.0), vec3(0.28, 0.16, 0.28), r.x)) r = U(r, bin(b1, 0.0));
  vec3 b2 = p - vec3(0.95, 0.0, -0.25);
  if (near(b2 - vec3(0.0, 0.12, 0.0), vec3(0.28, 0.16, 0.28), r.x)) r = U(r, bin(b2, 1.0));
  vec3 b3 = p - vec3(1.55, 0.0, -1.55);
  if (near(b3 - vec3(0.0, 0.12, 0.0), vec3(0.28, 0.16, 0.28), r.x)) r = U(r, bin(b3, 2.0));
  return r;
}
