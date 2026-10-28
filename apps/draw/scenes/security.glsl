#pragma frame {"size":[800,640],"center":[0.0,0.53,0.0],"zoom":218,"bounds":[[-1.2,-0.1,-1.2],[1.2,1.8,1.2]]}

const float BY = 0.67;

vec2 door(vec2 r, vec3 f) {
  vec3 d = f - vec3(0.14, 0.0, BY - 0.04);
  float rr = length(d.xz);
  r = Sc(r, M(max(rr - 0.36, -(d.y + 0.05)), GRAPHITE));
  vec2 ring = M(extrude(max(rr - 0.425, 0.36 - rr), d.y, 0.02), STEEL + ROUND);
  r = U(r, ring);
  vec3 sp = polarY(d, 10.0) - vec3(0.393, 0.02, 0.0);
  r = U(r, screw(sp, 0.014));
  vec3 dd = d;
  vec2 slab = M(sdCylinder(dd - vec3(0.0, 0.015, 0.0), 0.335, 0.06), WHITE + ROUND);
  slab = seam(slab, -(length(dd.xz) - 0.265));
  r = U(r, slab);
  vec3 tk = polarY(d, 8.0) - vec3(0.175, 0.08, 0.0);
  r = U(r, M(sdBox(tk, vec3(0.02, 0.006, 0.006)), GRAPHITE));
  vec3 dl = d - vec3(0.0, 0.1, 0.0);
  vec2 dial = M(sdRoundCylinder(dl, 0.12, 0.035, 0.012), STEEL + ROUND);
  dial = U(dial, M(sdRoundCylinder(dl - vec3(0.0, 0.04, 0.0), 0.065, 0.025, 0.01), GRAPHITE + ROUND));
  dial = U(dial, M(sdBox(dl - vec3(0.0, 0.07, 0.04), vec3(0.012, 0.012, 0.03)), WHITE));
  r = U(r, dial);
    vec3 h = d - vec3(-0.235, 0.0, 0.0);
  vec3 hp = vec3(h.x, h.y - 0.1, abs(h.z) - 0.12);
  vec2 handle = M(sdCylinder(hp, 0.018, 0.06), GRAPHITE + SOLID);
  handle = U(handle, M(sdCapsule(h, vec3(0.0, 0.15, -0.16), vec3(0.0, 0.15, 0.16), 0.03), LIME + SOLID));
  r = U(r, handle);
  vec3 bq = d;
  float ba = atan(bq.z, -bq.x);
  float k = clamp(round(ba / 0.5), -1.0, 1.0) * 0.5;
  vec2 dir = vec2(-cos(k), sin(k));
  vec3 bl = vec3(dot(bq.xz, dir), bq.y - 0.03, dot(bq.xz, vec2(-dir.y, dir.x)));
  r = U(r, M(sdCylinder(vec3(bl.z, bl.x - 0.36, bl.y), 0.028, 0.07), STEEL + SOLID));
  r = U(r, M(sdRoundCylinder(vec3(bl.z, bl.x - 0.43, bl.y), 0.042, 0.014, 0.006), GRAPHITE + ROUND));
  vec3 hg = vec3(d.x - 0.385, d.y - 0.02, abs(d.z) - 0.2);
  r = U(r, M(sdCylinder(hg.xzy, 0.035, 0.07), GRAPHITE + ROUND));
  return r;
}

vec2 keypad(vec3 f) {
  vec3 k = f - vec3(-0.53, 0.0, BY + 0.02);
  vec2 r = M(extrude(sdRect(k.xz, vec2(0.09, 0.16), 0.02), k.y, 0.015), STEEL);
  vec3 b = vec3(repC(k.x, 0.06, 3.0), k.y - 0.015, repC(k.z + 0.03, 0.06, 4.0));
  float ix = repI(k.x, 0.06, 3.0), iz = repI(k.z + 0.03, 0.06, 4.0);
  float m = (ix == 1.0 && iz == 0.0) ? LIME : WHITE;
  r = U(r, M(sdRoundBox(b, vec3(0.022, 0.01, 0.019), 0.006), m));
  r = U(r, led(k - vec3(0.05, 0.015, 0.135), 0.016, LIME));
  r = U(r, led(k - vec3(0.0, 0.015, 0.135), 0.016, STEEL));
  return r;
}

vec2 vault(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.06, 0.0), vec3(0.74, 0.06, 0.86), 0.02), WHITE);
  r = seam(r, p.y - 0.09);
    vec3 b = p - vec3(0.0, BY, 0.0);
  vec2 body = M(sdRoundBox(b, vec3(0.6, 0.55, 0.74), 0.05), WHITE);
  body = seam(body, -(b.y + 0.47));
  body = seam(body, b.y - 0.47);
  vec3 fz = faceZ(p, 0.74);
  body = vents(body, fz - vec3(0.0, 0.0, BY + 0.28), 7.0, 0.06, 0.16, 0.026, GRAPHITE);
  body = inset(body, fz - vec3(0.0, 0.0, BY - 0.2), vec2(0.38, 0.16), 0.03, 0.015, WHITE);
  vec3 fx = faceX(p, 0.6);
  body = door(body, fx);
  r = U(r, body);
  r = U(r, keypad(fx));
  vec3 rz = vec3(abs(fz.x) - 0.52, fz.y, repC(fz.z - BY, 0.17, 6.0));
  r = U(r, screw(rz, 0.016));
  vec3 pl = fz - vec3(0.0, 0.0, BY - 0.2);
  r = U(r, M(extrude(sdRect(pl.xz, vec2(0.3, 0.025), 0.025), pl.y + 0.008, 0.01), STEEL));
  r = U(r, M(extrude(sdRect(pl.xz - vec2(-0.08, -0.08), vec2(0.22, 0.025), 0.025), pl.y + 0.008, 0.01), STEEL));
  return r;
}

vec2 slabs(vec3 p) {
  vec3 q = p - vec3(0.0, 1.22, 0.0);
  vec3 a = rotY(q - vec3(0.02, 0.035, -0.02), 0.06);
  vec2 r = M(sdRoundBox(a, vec3(0.44, 0.035, 0.34), 0.012), WHITE);
  r = seam(r, a.y);
  vec3 b = rotY(q - vec3(-0.04, 0.105, 0.03), -0.1);
  vec2 s = M(sdRoundBox(b, vec3(0.4, 0.035, 0.3), 0.012), SUN);
  s = seam(s, b.y);
  r = U(r, s);
  vec3 c = rotY(q - vec3(0.03, 0.175, 0.0), 0.12);
  vec2 t = M(sdRoundBox(c, vec3(0.36, 0.035, 0.27), 0.012), LIME);
  t = seam(t, c.y);
  vec3 ct = faceY(c, 0.035);
  t = inset(t, ct, vec2(0.25, 0.16), 0.03, 0.01, LIME);
  r = U(r, t);
  r = U(r, screw(vec3(abs(ct.x) - 0.3, ct.y, abs(ct.z) - 0.21), 0.016));
  return r;
}

vec2 camera(vec3 p) {
  vec3 q = p - vec3(0.4, 1.22, -0.6);
  vec2 r = M(sdRoundCylinder(q - vec3(0.0, 0.02, 0.0), 0.07, 0.02, 0.008), GRAPHITE + ROUND);
  r = U(r, M(sdCylinder(q - vec3(0.0, 0.14, 0.0), 0.018, 0.12), GRAPHITE + SOLID));
  vec3 c = rotX(rotY(q - vec3(0.02, 0.3, 0.02), PI * 0.25 - 0.6), 0.25);
  vec2 body = M(sdRoundBox(c, vec3(0.08, 0.06, 0.15), 0.02), WHITE);
  body = seam(body, c.z + 0.07);
  r = U(r, body);
  vec3 lq = c - vec3(0.0, 0.0, 0.15);
  r = U(r, M(sdRoundCylinder(rotX(lq, PI * 0.5) - vec3(0.0, -0.015, 0.0), 0.045, 0.02, 0.006), GRAPHITE + ROUND));
  r = U(r, M(sdCylinder(rotX(lq, PI * 0.5) - vec3(0.0, -0.035, 0.0), 0.025, 0.006), BLUE + ROUND));
  r = U(r, M(sdRoundBox(c - vec3(0.0, 0.072, 0.09), vec3(0.09, 0.01, 0.1), 0.005), STEEL));
  r = U(r, led(faceY(c, 0.06) - vec3(0.0, 0.0, -0.08), 0.015, LIME));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.62, 0.0), vec3(0.8, 0.64, 0.8), r.x)) r = U(r, vault(p));
  if (near(p - vec3(0.0, 1.35, 0.0), vec3(0.5, 0.15, 0.45), r.x)) r = U(r, slabs(p));
  if (near(p - vec3(0.4, 1.45, -0.6), vec3(0.25, 0.25, 0.25), r.x)) r = U(r, camera(p));
  return r;
}
