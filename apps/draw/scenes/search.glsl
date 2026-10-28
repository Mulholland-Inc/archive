#pragma frame {"size":[800,640],"center":[0.2,0.5,0.25],"zoom":290,"bounds":[[-1.0,-0.05,-0.9],[1.6,1.8,1.6]]}

#define H 0.03
#define STEP 0.076

vec2 record(vec3 p, float i, float m) {
  vec2 r = M(sdRoundBox(p, vec3(0.55, H, 0.36), 0.01), m);
  vec3 f = faceZ(p, 0.36);
  float ink = m == WHITE ? STEEL : WHITE;
  float tag = mod(i, 3.0) == 0.0 ? CORAL : mod(i, 3.0) == 1.0 ? BLUE : SUN;
  if (m != WHITE) tag = GRAPHITE;
  r = decal(r, f, sdRect(f.xz - vec2(-0.42, 0.0), vec2(0.07, 0.016), 0.008), tag);
  r = decal(r, f, sdRect(f.xz - vec2(-0.08, 0.0), vec2(0.2, 0.01), 0.01), ink);
  r = decal(r, f, sdRect(f.xz - vec2(0.3, 0.0), vec2(0.12, 0.01), 0.01), ink);
  vec3 t = faceY(p, H);
  if (m == WHITE) {
    r = decal(r, t, sdRect(t.xz - vec2(-0.36, -0.2), vec2(0.1, 0.035), 0.035), tag);
    r = decal(r, t, sdRect(t.xz - vec2(-0.05, -0.06), vec2(0.4, 0.02), 0.02), STEEL);
    r = decal(r, t, sdRect(t.xz - vec2(-0.13, 0.04), vec2(0.32, 0.02), 0.02), STEEL);
    r = decal(r, t, sdRect(t.xz - vec2(-0.2, 0.14), vec2(0.25, 0.02), 0.02), STEEL);
    r = decal(r, t, sdCircle(t.xz - vec2(0.38, -0.2), 0.05), STEEL);
  }
  vec3 x = faceX(p, 0.55);
  r = decal(r, x, sdRect(x.xz - vec2(0.0, 0.0), vec2(0.2, 0.01), 0.01), ink);
  return r;
}

vec2 hit(vec3 p) {
  vec2 r = record(p, 9.0, LIME);
  vec3 t = faceY(p, H);
  r = decal(r, t, sdRect(t.xz - vec2(-0.25, 0.14), vec2(0.16, 0.03), 0.03), GRAPHITE);
  r = decal(r, t, sdRect(t.xz - vec2(0.0, 0.24), vec2(0.4, 0.02), 0.02), WHITE);
  return r;
}

vec2 lens(vec3 q) {
  vec2 r = M(sdTorus(q, 0.28, 0.034), WHITE + SOLID);
  r = U(r, M(max(sdTorus(q, 0.28, 0.046), abs(q.y) - 0.018), GRAPHITE + SOLID));
  vec3 g = q - vec3(0.0, 0.01, 0.0);
  vec3 h = q - vec3(0.32, 0.0, 0.0);
  r = U(r, M(sdCylinder(rotZ(h - vec3(0.04, 0.0, 0.0), PI * 0.5), 0.05, 0.04), STEEL + ROUND));
  r = U(r, M(sdCylinder(rotZ(h - vec3(0.1, 0.0, 0.0), PI * 0.5), 0.065, 0.02), STEEL + ROUND));
  r = U(r, M(sdRoundCylinder(rotZ(h - vec3(0.36, 0.0, 0.0), PI * 0.5), 0.07, 0.24, 0.05), GRAPHITE + ROUND));
  r = U(r, M(sdTorus(rotZ(h - vec3(0.25, 0.0, 0.0), PI * 0.5), 0.07, 0.012), STEEL + SOLID));
  return r;
}

vec2 base(vec3 p) {
  vec3 b = p - vec3(0.0, 0.05, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.68, 0.05, 0.46), 0.03), STEEL);
  r = seam(r, b.y);
  vec3 fz = faceZ(p, 0.46);
  r = vents(r, fz - vec3(0.35, 0.0, 0.05), 5.0, 0.06, 0.03, 0.026, GRAPHITE);
  vec3 c = mirXZ(faceY(p, 0.1)) - vec3(0.62, 0.0, 0.4);
  r = U(r, screw(c, 0.025));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = base(p);
  float y0 = 0.1 + H;
  for (int i = 0; i < 8; i++) {
    float fi = float(i);
    vec3 q = p - vec3(0.0, y0 + fi * STEP, 0.0);
    if (i == 4) {
      r = U(r, hit(q - vec3(0.04, 0.0, 0.34)));
    } else {
      float j = mod(fi * 7.0, 3.0);
      r = U(r, record(q - vec3((j - 1.0) * 0.012, 0.0, (1.0 - j) * 0.01), fi, WHITE));
    }
  }
  vec3 c = p - vec3(0.26, 0.68, 0.74);
  vec3 lq = rotY(rotX(rotY(c, PI * 0.25), 0.75), -0.6);
  r = U(r, lens(lq));
  return r;
}
