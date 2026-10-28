#pragma frame {"size":[800,640],"center":[-0.03,1.07,0.03],"zoom":200,"bounds":[[-2.0,-0.05,-0.6],[2.1,2.4,0.6]]}

#define HX 0.6
#define HZ 0.2
#define GAP 0.42

vec2 line(vec2 o, vec3 q, vec2 c, float w, float m) {
  return decal(o, q, sdRect(q.xz - c, vec2(w, 0.022), 0.022), m);
}

vec2 strip(vec3 p, float kind, float m) {
  vec2 r = M(sdRoundBox(p, vec3(HX, 0.035, HZ), 0.015), m);
  r = seam(r, p.y + 0.01);
  vec3 q = faceY(p, 0.035);
  float ink = m == WHITE ? STEEL : WHITE;
  if (kind == 0.0) {
    r = U(r, M(sdRoundBox(q - vec3(-0.25, 0.025, -0.04), vec3(0.26, 0.025, 0.06), 0.02), GRAPHITE));
    r = line(r, q, vec2(-0.22, 0.1), 0.3, ink);
    r = U(r, M(sdRoundCylinder(q - vec3(0.42, 0.02, 0.0), 0.08, 0.02, 0.01), CORAL + ROUND));
  } else if (kind == 1.0) {
    r = line(r, q, vec2(-0.1, -0.1), 0.42, ink);
    r = line(r, q, vec2(0.0, 0.0), 0.5, ink);
    r = line(r, q, vec2(-0.18, 0.1), 0.34, ink);
  } else if (kind == 2.0) {
    vec2 c = vec2(repC(q.x + 0.1, 0.2, 4.0), repC(q.z, 0.12, 3.0));
    float cell = max(sdRect(c, vec2(0.085, 0.045), 0.01), sdRect(q.xz - vec2(-0.1, 0.0), vec2(0.4, 0.18), 0.0));
    r = Sc(r, M(max(cell, -(q.y + 0.015)), STEEL));
    r = U(r, button(q - vec3(0.45, 0.0, 0.0), 0.06, 0.02, STEEL));
  } else if (kind == 3.0) {
    vec3 iq = q - vec3(-0.32, 0.0, 0.0);
    r = inset(r, iq, vec2(0.22, 0.15), 0.02, 0.012, SKY);
    r = decal(r, iq + vec3(0.0, 0.012, 0.0), max(sdRect(iq.xz, vec2(0.22, 0.15), 0.02), 0.65 * abs(iq.x + 0.05) - 0.04 - iq.z), MINT);
    r = decal(r, iq + vec3(0.0, 0.012, 0.0), sdCircle(iq.xz - vec2(0.12, -0.06), 0.04), SUN);
    r = line(r, q, vec2(0.24, -0.08), 0.24, ink);
    r = line(r, q, vec2(0.2, 0.02), 0.2, ink);
    r = line(r, q, vec2(0.24, 0.12), 0.24, ink);
  } else if (kind == 4.0) {
    r = line(r, q, vec2(-0.12, -0.06), 0.38, ink);
    r = line(r, q, vec2(-0.26, 0.06), 0.26, ink);
    r = U(r, M(sdRoundBox(q - vec3(0.45, 0.02, 0.0), vec3(0.08, 0.02, 0.05), 0.012), STEEL));
  } else {
    r = line(r, q, vec2(-0.1, -0.08), 0.42, ink);
    r = line(r, q, vec2(-0.18, 0.04), 0.34, ink);
    r = U(r, led(q - vec3(0.45, 0.0, 0.0), 0.04, GRAPHITE));
  }
  return r;
}

vec2 ghost(vec3 q) {
  vec2 h = vec2(HX, HZ);
  float wire = extrude(abs(sdRect(q.xz, h, 0.015)) - 0.008, q.y, 0.008);
  float u = abs(q.z) > HZ - 0.03 ? q.x : q.z;
  float dash = abs(mod(u + 0.025, 0.1) - 0.05) - 0.025;
  return M(max(wire, dash), GRAPHITE + SOLID);
}

vec2 jig(vec3 p) {
  vec3 b = p - vec3(0.0, 0.06, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.76, 0.06, 0.36), 0.03), WHITE);
  r = seam(r, b.y + 0.02);
  r = paint(r, b.y + 0.02, STEEL);
  vec3 fz = faceZ(p, 0.36);
  r = vents(r, fz - vec3(-0.35, 0.0, 0.07), 6.0, 0.07, 0.035, 0.028, GRAPHITE);
  r = U(r, led(fz - vec3(0.42, 0.0, 0.07), 0.025, LIME));
  r = U(r, led(fz - vec3(0.52, 0.0, 0.07), 0.025, STEEL));
  vec3 c = mirXZ(p) - vec3(0.68, 0.0, 0.28);
  r = U(r, M(sdCylinder(c - vec3(0.0, 1.18, 0.0), 0.02, 1.06), STEEL + ROUND));
  r = U(r, M(sdRoundCylinder(c - vec3(0.0, 0.14, 0.0), 0.05, 0.025, 0.01), GRAPHITE + ROUND));
  r = U(r, M(sdSphere(c - vec3(0.0, 2.26, 0.0), 0.04), GRAPHITE + SOLID));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = jig(p);
  float y0 = 0.3;
  r = U(r, strip(p - vec3(0.0, y0, 0.0), 5.0, WHITE));
  r = U(r, strip(p - vec3(0.0, y0 + GAP, 0.0), 4.0, WHITE));
  r = U(r, strip(p - vec3(-1.26, y0 + GAP * 2.0, 0.0), 2.0, WHITE));
  r = U(r, ghost(p - vec3(0.0, y0 + GAP * 2.0, 0.0)));
  r = U(r, strip(p - vec3(0.92, y0 + GAP * 2.0, 0.0), 1.0, LIME));
  r = U(r, strip(p - vec3(0.0, y0 + GAP * 3.0, 0.0), 3.0, WHITE));
  r = U(r, strip(p - vec3(0.0, y0 + GAP * 4.0, 0.0), 0.0, WHITE));
  vec3 m = p - vec3(1.66, y0 + GAP * 2.0, 0.0);
  r = U(r, M(sdCapsule(m, vec3(-0.06, 0.0, -0.12), vec3(0.1, 0.0, -0.12), 0.018), GRAPHITE + SOLID));
  r = U(r, M(sdCapsule(m, vec3(0.0, 0.0, 0.0), vec3(0.2, 0.0, 0.0), 0.018), GRAPHITE + SOLID));
  r = U(r, M(sdCapsule(m, vec3(-0.06, 0.0, 0.12), vec3(0.1, 0.0, 0.12), 0.018), GRAPHITE + SOLID));
  return r;
}
