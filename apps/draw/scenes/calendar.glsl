#pragma frame {"size":[800,640],"center":[0.05,0.62,0.05],"zoom":235,"bounds":[[-1.3,-0.1,-0.9],[1.3,1.6,0.9]]}

float bar(vec2 p, float x0, float y, float len, float w) {
  return sdRect(p - vec2(x0 + len * 0.5, y), vec2(len * 0.5, w), w);
}

vec2 dayTile(vec3 q, float m, float h) {
  vec2 t = M(sdRoundBox(q - vec3(0.0, h, 0.0), vec3(0.092, h, 0.082), 0.018), m);
  vec3 s = faceY(q, h * 2.0);
  float dot = sdCircle(s.xz - vec2(-0.045, 0.038), 0.018);
  t = decal(t, s, dot, m == WHITE ? STEEL : WHITE);
  return t;
}

vec2 face(vec2 r, vec3 f) {
  vec3 g = f - vec3(0.0, 0.0, -0.13);
  float col = repI(g.x, 0.232, 7.0);
  float row = repI(g.z, 0.21, 4.0);
  float gi = col + 7.0 * (3.0 - row);
  vec3 c = vec3(repC(g.x, 0.232, 7.0), g.y, repC(g.z, 0.21, 4.0));
  bool hot = gi == 9.0 || gi == 17.0 || gi == 26.0;
  if (!hot) r = U(r, dayTile(c - vec3(0.0, -0.01, 0.0), WHITE, 0.012));
  r = U(r, dayTile(g - vec3(-0.232, -0.01, 0.105), LIME, 0.04));
  r = U(r, dayTile(g - vec3(0.0, -0.01, -0.105), LILAC, 0.04));
  r = U(r, dayTile(g - vec3(0.464, -0.01, -0.315), CORAL, 0.04));
  vec3 w = vec3(repC(f.x, 0.232, 7.0), f.y, f.z - 0.36);
  r = decal(r, w, sdRect(w.xz, vec2(0.05, 0.016), 0.016), repI(f.x, 0.232, 7.0) > 4.5 ? CORAL : STEEL);
  vec3 hb = vec3(repC(f.x, 0.29, 6.0), f.y, f.z - 0.47);
  r = decal(r, hb, sdCircle(hb.xz, 0.032), GRAPHITE);
  return r;
}

vec2 rings(vec3 p) {
  vec3 q = vec3(repC(p.x, 0.29, 6.0), p.y - 0.56, p.z);
  return M(sdTorus(rotZ(q, PI * 0.5), 0.15, 0.017), STEEL + SOLID);
}

vec2 pin(vec3 p) {
  vec2 r = M(sdCylinder(p - vec3(0.0, 0.03, 0.0), 0.008, 0.03), STEEL + SOLID);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.07, 0.0), 0.018, 0.02), GRAPHITE + SOLID));
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.1, 0.0), 0.04, 0.018, 0.009), GRAPHITE + SOLID));
  return r;
}

vec2 body(vec3 p) {
  vec2 r = M(sdRoundBox(p, vec3(0.9, 0.6, 0.12), 0.04), WHITE);
  if (p.z < 0.02) r = stripes(paint(r, -1.0, PAPER), p.z, 0.022);
  vec3 f = faceZ(p, 0.12);
  r = face(r, f);
  r = U(r, rings(p));
  r = U(r, pin(f - vec3(0.035, 0.06, -0.205)));
  return r;
}

vec2 pen(vec3 p) {
  vec2 r = M(sdCylinder(rotZ(p, PI * 0.5), 0.035, 0.32), CORAL + SOLID);
  r = U(r, M(sdCone(rotZ(p - vec3(0.37, 0.0, 0.0), PI * 0.5), 0.05, 0.035, 0.008), WHITE + SOLID));
  r = U(r, M(sdCylinder(rotZ(p + vec3(0.335, 0.0, 0.0), PI * 0.5), 0.037, 0.03), GRAPHITE + SOLID));
  r = U(r, M(sdBox(p - vec3(-0.18, 0.04, 0.0), vec3(0.12, 0.008, 0.014)), STEEL));
  return r;
}

vec2 map(vec3 p) {
  vec3 fb = p - vec3(0.0, 0.07, 0.05);
  vec2 r = M(sdRoundBox(fb, vec3(1.02, 0.07, 0.5), 0.03), STEEL);
  r = seam(r, fb.y + 0.02);
  vec3 tz = faceZ(fb, 0.5);
  r = vents(r, tz - vec3(0.62, 0.0, -0.005), 5.0, 0.06, 0.04, 0.024, GRAPHITE);
  vec3 cs = mirXZ(faceY(fb, 0.07));
  r = U(r, screw(vec3(cs.x - 0.94, cs.y, cs.z - 0.42), 0.022));
  r = Sc(r, M(sdBox(fb - vec3(0.0, 0.07, -0.07), vec3(0.94, 0.04, 0.15)), GRAPHITE));
  vec3 b = rotX(p - vec3(0.0, 0.72, -0.14), -0.22);
  if (near(b, vec3(1.0, 0.8, 0.3), r.x)) r = U(r, body(b));
  r = U(r, pen(rotY(p - vec3(0.3, 0.175, 0.36), 0.12)));
  vec3 n = rotY(p - vec3(-0.6, 0.14, 0.33), -0.2);
  vec2 note = M(sdBox(n - vec3(0.0, 0.03, 0.0), vec3(0.14, 0.03, 0.1)), LIME);
  r = U(r, stripes(note, n.y - 0.02, 0.02));
  return r;
}
