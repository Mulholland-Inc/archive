#pragma frame {"size":[800,640],"center":[0.1,0.55,0.05],"zoom":270,"bounds":[[-1.2,-0.1,-1.2],[1.2,1.6,1.2]]}

vec2 cube(vec3 p, float s, float m) {
  return M(sdRoundBox(p, vec3(s), s * 0.16), m);
}

vec2 box(vec3 p) {
  vec3 b = p - vec3(0.0, 0.25, 0.0);
  float outer = sdRoundBox(b, vec3(0.55, 0.25, 0.44), 0.035);
  float inner = sdBox(b - vec3(0.0, 0.2, 0.0), vec3(0.49, 0.25, 0.38));
  vec2 r = M(max(outer, -inner), WHITE);
  r = seam(r, b.y - 0.13);
  r = seam(r, b.y + 0.17);
  r = paint(r, b.y + 0.17, STEEL);
  vec3 fz = faceZ(p, 0.44);
  r = vents(r, fz - vec3(-0.3, 0.0, 0.2), 4.0, 0.05, 0.12, 0.02, GRAPHITE);
  r = vents(r, fz - vec3(0.3, 0.0, 0.2), 4.0, 0.05, 0.12, 0.02, GRAPHITE);
  vec3 sc = vec3(abs(fz.x) - 0.48, fz.y, fz.z - 0.43);
  r = U(r, screw(sc, 0.018));
  vec3 fx = faceX(p, 0.55);
  r = U(r, screw(vec3(abs(fx.x) - 0.37, fx.y, fx.z - 0.43), 0.018));
  vec3 h = p - vec3(0.6, 0.3, 0.0);
  vec2 handle = M(min(min(pipe(h, vec3(0.0, 0.0, -0.18), vec3(0.0, 0.0, 0.18), 0.022), pipe(h, vec3(-0.05, 0.0, -0.18), vec3(0.0, 0.0, -0.18), 0.022)), pipe(h, vec3(-0.05, 0.0, 0.18), vec3(0.0, 0.0, 0.18), 0.022)), STEEL + SOLID);
  r = U(r, handle);
  vec3 l = fz - vec3(0.0, 0.0, 0.36);
  r = U(r, plate(l, vec2(0.09, 0.1), 0.02, 0.012, STEEL));
  r = U(r, screw(vec3(abs(l.x) - 0.055, l.y - 0.012, l.z + 0.06), 0.012));
  vec3 st = vec3(l.x, l.y - 0.05, l.z - 0.02);
  r = U(r, M(sdTorus(rotX(st, PI * 0.5), 0.038, 0.012), GRAPHITE + SOLID));
  r = U(r, M(sdBox(p - vec3(0.0, 0.17, 0.0), vec3(0.49, 0.02, 0.38)), PAPER));
  return r;
}

vec2 lid(vec3 p) {
  vec3 b = p - vec3(0.0, 0.0, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.57, 0.06, 0.46), 0.035), WHITE);
  r = seam(r, b.y + 0.015);
  r = U(r, M(sdBox(b - vec3(0.0, -0.08, 0.0), vec3(0.485, 0.03, 0.375)), STEEL));
  vec3 t = faceY(p, 0.06);
  r = inset(r, t, vec2(0.42, 0.33), 0.04, 0.015, STEEL);
  r = U(r, M(extrude(sdRect(t.xz, vec2(0.16, 0.035), 0.035), t.y - 0.03, 0.03), LILAC + ROUND));
  vec3 sc = mirXZ(t) - vec3(0.5, 0.0, 0.39);
  r = U(r, screw(sc, 0.018));
  vec3 tg = p - vec3(0.0, -0.12, 0.475);
  vec2 tongue = M(sdRoundBox(tg, vec3(0.06, 0.13, 0.012), 0.008), STEEL);
  tongue = Sc(tongue, M(sdBox(tg - vec3(0.0, -0.07, 0.0), vec3(0.03, 0.022, 0.05)), GRAPHITE));
  r = U(r, tongue);
  r = U(r, screw(faceZ(p, 0.487) - vec3(0.0, 0.0, -0.03), 0.012));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.03, 0.25, 0.0), vec3(0.65, 0.3, 0.5), r.x)) r = U(r, box(p));
  vec3 lq = rotX(rotZ(p - vec3(0.0, 1.08, -0.04), 0.07), -0.06);
  if (near(lq, vec3(0.62, 0.35, 0.55), r.x)) r = U(r, lid(lq));
  if (near(p - vec3(0.0, 0.5, 0.0), vec3(0.5, 0.3, 0.4), r.x)) {
    float s = 0.085;
    r = U(r, cube(p - vec3(-0.36, 0.28, -0.26), s, WHITE));
    r = U(r, cube(rotY(p - vec3(-0.16, 0.28, -0.25), 0.15), s, LILAC));
    r = U(r, cube(p - vec3(0.06, 0.28, -0.27), s, WHITE));
    r = U(r, cube(rotY(p - vec3(0.3, 0.28, -0.24), -0.1), s, WHITE));
    r = U(r, cube(p - vec3(-0.34, 0.28, -0.02), s, LILAC));
    r = U(r, cube(rotY(p - vec3(-0.12, 0.28, 0.0), -0.2), s, WHITE));
    r = U(r, cube(p - vec3(0.12, 0.28, 0.02), s, WHITE));
    r = U(r, cube(rotY(p - vec3(0.36, 0.28, -0.01), 0.25), s, LILAC));
    r = U(r, cube(rotY(p - vec3(-0.35, 0.28, 0.24), 0.1), s, WHITE));
    r = U(r, cube(p - vec3(-0.13, 0.28, 0.25), s, WHITE));
    r = U(r, cube(rotY(p - vec3(-0.34, 0.455, -0.03), 0.4), s, WHITE));
    r = U(r, cube(rotY(p - vec3(0.06, 0.455, -0.26), -0.3), s, LILAC));
  }
  r = U(r, cube(rotZ(rotX(rotY(p - vec3(0.24, 0.66, 0.22), 0.5), 0.35), 0.3), 0.085, LILAC));
  r = U(r, cube(rotY(p - vec3(0.9, 0.085, 0.12), 0.3), 0.085, WHITE));
  r = U(r, cube(rotY(p - vec3(0.86, 0.085, 0.42), -0.2), 0.085, LILAC));
  r = U(r, cube(rotY(p - vec3(0.9, 0.255, 0.14), 0.7), 0.085, LIME));
  return r;
}
