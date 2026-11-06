#pragma frame {"size":[800,640],"center":[0.05,0.6,0.1],"zoom":250,"bounds":[[-1.0,-0.1,-1.0],[1.0,1.4,1.4]]}

vec2 handle(vec3 q) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.05, 0.0), vec3(0.12, 0.016, 0.022), 0.012), STEEL);
  vec3 s = vec3(abs(q.x) - 0.1, q.y - 0.025, q.z);
  r = U(r, M(sdBox(s, vec3(0.014, 0.025, 0.014)), STEEL));
  return r;
}

vec2 label(vec3 q) {
  return M(extrude(max(sdRect(q.xz, vec2(0.09, 0.04), 0.008), -sdRect(q.xz, vec2(0.072, 0.024), 0.004)), q.y - 0.006, 0.006), STEEL);
}

vec2 cabinet(vec3 p) {
  vec3 b = p - vec3(0.0, 0.62, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(0.42, 0.56, 0.38), 0.02), WHITE);
  r = seam(r, -(abs(b.y) - 0.52));
  vec3 f = faceZ(p, 0.38);
  for (int i = 0; i < 2; i++) {
    float y = i == 0 ? 0.37 : 0.86;
    r = U(r, M(sdRoundBox(p - vec3(0.0, y, 0.39), vec3(0.38, 0.21, 0.02), 0.012), WHITE));
    vec3 d = faceZ(p, 0.41) - vec3(0.0, 0.0, y);
    r = inset(r, d, vec2(0.33, 0.16), 0.02, 0.012, PAPER);
    r = U(r, handle(d - vec3(-0.1, 0.0, -0.06)));
    r = U(r, label(d - vec3(-0.1, 0.0, 0.08)));
  }
  r = U(r, M(sdBox(p - vec3(0.0, 0.03, 0.0), vec3(0.38, 0.03, 0.34)), GRAPHITE));
  vec3 fx = faceX(p, 0.42);
  r = inset(r, fx - vec3(0.0, 0.0, 0.62), vec2(0.28, 0.42), 0.03, 0.012, WHITE);
  r = U(r, screw(vec3(abs(fx.x) - 0.22, fx.y, abs(fx.z - 0.62) - 0.36), 0.022));
  vec3 t = faceY(p, 1.18);
  r = inset(r, t, vec2(0.34, 0.3), 0.03, 0.01, WHITE);
  return r;
}

vec3 lockFrame(vec3 p) {
  vec3 l = faceZ(p, 0.41) - vec3(0.2, 0.0, 0.88);
  return vec3(l.x, l.z, l.y);
}

vec2 lock(vec3 p) {
  vec3 k = lockFrame(p);
  vec2 r = M(sdRoundCylinder(rotX(k, PI * 0.5) - vec3(0.0, 0.012, 0.0), 0.075, 0.014, 0.006), STEEL + ROUND);
  r = U(r, M(sdCylinder(rotX(k, PI * 0.5) - vec3(0.0, 0.026, 0.0), 0.05, 0.004), GRAPHITE + ROUND));
  vec3 q = k;
  r = U(r, M(sdCapsule(q, vec3(0.0, 0.0, 0.0), vec3(0.0, 0.0, 0.2), 0.022), STEEL + SOLID));
  r = U(r, M(sdRoundCylinder(rotX(q - vec3(0.0, 0.0, 0.2), PI * 0.5), 0.04, 0.014, 0.006), STEEL + ROUND));
  vec3 bq = q - vec3(0.0, 0.0, 0.34);
  float c = sdCircle(bq.yz, 0.12);
  vec2 bow = M(extrude(abs(c) - 0.038, bq.x, 0.024), LIME);
  bow = seam(bow, abs(c) - 0.016);
  r = U(r, bow);
  return r;
}

vec2 tag(vec3 p) {
  vec3 k = lockFrame(p);
  vec3 rg = k - vec3(0.0, -0.15, 0.34);
  vec2 r = M(sdTorus(rotX(rg, PI * 0.5), 0.045, 0.011), GRAPHITE + SOLID);
  vec3 tq = rotZ(k - vec3(0.0, -0.36, 0.34), 0.1);
  float d2 = sdRect(tq.xy, vec2(0.085, 0.14), 0.03);
  vec2 tg = M(extrude(d2, tq.z, 0.012), SUN);
  tg = Sc(tg, M(max(sdCircle(tq.xy - vec2(0.0, 0.095), 0.02), abs(tq.z) - 0.05), GRAPHITE));
  vec3 tf = vec3(tq.x, tq.z - 0.012, tq.y);
  tg = decal(tg, tf, sdRect(tf.xz - vec2(0.0, 0.0), vec2(0.05, 0.012), 0.012), WHITE);
  tg = decal(tg, tf, sdRect(tf.xz - vec2(-0.015, -0.05), vec2(0.035, 0.012), 0.012), WHITE);
  r = U(r, tg);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.6, 0.0), vec3(0.46, 0.62, 0.45), r.x)) r = U(r, cabinet(p));
  if (near(p - vec3(0.2, 0.88, 0.7), vec3(0.25, 0.25, 0.3), r.x)) r = U(r, lock(p));
  if (near(p - vec3(0.2, 0.58, 0.75), vec3(0.15, 0.25, 0.1), r.x)) r = U(r, tag(p));
  return r;
}
