#pragma frame {"size":[800,640],"center":[0.0,0.74,0.0],"zoom":380,"bounds":[[-1.0,-0.1,-1.0],[1.0,1.6,1.0]]}

vec2 keycard(vec3 p, float body) {
  vec3 c = p - vec3(0.0, -0.34, 0.0);
  vec2 r = M(sdRoundBox(c, vec3(0.26, 0.42, 0.016), 0.045), body);
  vec3 f = faceZ(c, 0.016);
  float accent = body == WHITE ? STEEL : WHITE;
  r = Sc(r, M(max(sdRect(p.xy, vec2(0.075, 0.022), 0.022), abs(p.z) - 0.1), GRAPHITE));
  r = U(r, M(extrude(abs(sdRect(f.xz - vec2(0.0, 0.34), vec2(0.075, 0.022), 0.022)) - 0.014, f.y - 0.005, 0.005), STEEL));
  vec3 ch = f - vec3(-0.15, 0.0, 0.1);
  vec2 chip = plate(ch - vec3(0.0, 0.004, 0.0), vec2(0.075, 0.06), 0.015, 0.004, SUN);
  chip = seam(chip, ch.x);
  chip = seam(chip, abs(ch.z) - 0.02);
  r = U(r, chip);
  r = decal(r, f, sdRect(f.xz - vec2(0.1, 0.13), vec2(0.1, 0.022), 0.022), accent);
  r = decal(r, f, sdRect(f.xz - vec2(0.06, 0.06), vec2(0.06, 0.022), 0.022), accent);
  r = decal(r, f, sdRect(f.xz - vec2(-0.1, -0.12), vec2(0.12, 0.022), 0.022), accent);
  r = decal(r, f, sdRect(f.xz - vec2(-0.04, -0.19), vec2(0.18, 0.022), 0.022), accent);
  r = decal(r, f, sdRect(f.xz - vec2(0.14, -0.29), vec2(0.07, 0.07), 0.02), body == WHITE ? LILAC : WHITE);
  return r;
}

vec2 map(vec3 p) {
  vec3 rc = p - vec3(0.0, 1.12, 0.0);
  vec2 r = M(sdTorus(rc, 0.3, 0.02), STEEL + SOLID);
  vec3 hk = rotY(rc - vec3(-0.24, 0.0, -0.24), PI * 0.25);
  r = U(r, M(sdTorus(rotX(hk - vec3(0.0, 0.07, 0.0), PI * 0.5), 0.065, 0.016), GRAPHITE + SOLID));
  vec3 cl = hk - vec3(0.0, 0.2, 0.0);
  vec2 clip = M(sdRoundBox(cl, vec3(0.04, 0.07, 0.03), 0.015), LILAC);
  clip = seam(clip, cl.y + 0.02);
  r = U(r, clip);
  r = U(r, M(sdTorus(rotX(cl - vec3(0.0, 0.1, 0.0), PI * 0.5), 0.035, 0.012), GRAPHITE + SOLID));
  vec3 a = rotX(rotZ(p - vec3(0.3, 1.12, 0.0), 0.3), -0.06);
  if (near(a - vec3(0.0, -0.34, 0.0), vec3(0.3, 0.48, 0.06), r.x)) r = U(r, keycard(a, LIME));
  vec3 b = rotY(rotX(p - vec3(0.0, 1.12, 0.3), -0.3), PI * 0.5);
  b = rotX(b, 0.04);
  if (near(b - vec3(0.0, -0.34, 0.0), vec3(0.3, 0.48, 0.06), r.x)) r = U(r, keycard(b, WHITE));
  return r;
}
