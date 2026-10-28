#pragma frame {"size":[800,640],"center":[0.0,0.3,0.0],"zoom":205,"bounds":[[-1.7,-0.1,-1.7],[1.7,1.2,1.7]]}

#define TR 0.85

vec2 row(vec3 p, float tag) {
  vec2 c = M(sdRoundBox(p, vec3(0.2, 0.025, 0.14), 0.015), WHITE);
  c = paint(c, p.x + 0.13, tag);
  vec3 q = faceY(p, 0.025);
  c = decal(c, q, sdRect(q.xz - vec2(0.04, -0.06), vec2(0.1, 0.014), 0.014), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.02, 0.0), vec2(0.08, 0.014), 0.014), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.04, 0.06), vec2(0.1, 0.014), 0.014), STEEL);
  return c;
}

vec2 rows(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.03, 0.0), vec3(0.27, 0.03, 0.2), 0.012), WHITE);
  r = U(r, screw(faceY(mirXZ(p), 0.06) - vec3(0.23, 0.0, 0.16), 0.016));
  r = U(r, row(p - vec3(0.0, 0.085, 0.0), LIME));
  r = U(r, row(p - vec3(0.015, 0.14, 0.0), CORAL));
  r = U(r, row(p - vec3(-0.01, 0.195, 0.0), LILAC));
  r = U(r, row(p - vec3(0.01, 0.25, 0.0), SUN));
  r = U(r, row(p - vec3(0.0, 0.305, 0.0), LIME));
  return r;
}

vec2 oven(vec3 p) {
  vec3 o = p - vec3(0.0, 0.36, 0.0);
  vec2 r = M(sdRoundBox(o, vec3(0.27, 0.26, 0.32), 0.02), WHITE);
  r = seam(r, o.y - 0.16);
  vec3 t = p - vec3(0.0, 0.24, 0.0);
  r = Sc(r, M(sdBox(t, vec3(0.17, 0.14, 0.5)), GRAPHITE));
  vec3 fx = faceX(p, 0.27);
  vec3 w = fx - vec3(0.02, 0.0, 0.33);
  r = inset(r, w, vec2(0.2, 0.13), 0.03, 0.025, SUN);
  if (sdRect(w.xz, vec2(0.2, 0.13), 0.03) < 0.0) r = seam(r, w.x);
  r = U(r, M(extrude(max(sdRect(w.xz, vec2(0.24, 0.17), 0.05), -sdRect(w.xz, vec2(0.2, 0.13), 0.03)), w.y - 0.01, 0.01), STEEL));
  vec3 fz = faceZ(p, 0.32);
  r = vents(r, fz - vec3(-0.08, 0.0, 0.5), 4.0, 0.06, 0.07, 0.022, GRAPHITE);
  r = U(r, led(fz - vec3(0.15, 0.0, 0.5), 0.026, CORAL));
  r = U(r, screw(fz - vec3(0.22, 0.0, 0.5), 0.016));
  r = U(r, M(extrude(max(sdRect(fz.xz - vec2(0.0, 0.24), vec2(0.22, 0.17), 0.03), -sdRect(fz.xz - vec2(0.0, 0.24), vec2(0.17, 0.14), 0.0)), fz.y - 0.012, 0.012), STEEL));
  vec3 tp = faceY(p, 0.62);
  r = U(r, button(tp - vec3(0.1, 0.03, 0.18), 0.05, 0.03, GRAPHITE));
  r = U(r, button(tp - vec3(-0.06, 0.03, 0.18), 0.05, 0.03, GRAPHITE));
  r = U(r, M(sdBox(tp - vec3(0.1, 0.075, 0.18), vec3(0.008, 0.016, 0.04)), WHITE));
  r = U(r, M(sdBox(tp - vec3(-0.06, 0.075, 0.18), vec3(0.04, 0.016, 0.008)), WHITE));
  vec3 ch = tp - vec3(-0.1, 0.0, -0.15);
  r = U(r, M(sdCylinder(ch - vec3(0.0, 0.12, 0.0), 0.065, 0.12), STEEL + ROUND));
  r = U(r, M(sdCylinder(ch - vec3(0.0, 0.25, 0.0), 0.085, 0.022), STEEL + ROUND));
  r = Sc(r, M(sdCylinder(ch - vec3(0.0, 0.28, 0.0), 0.05, 0.05), GRAPHITE + ROUND));
  return r;
}

vec2 model(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.1, 0.0), 0.24, 0.1, 0.02), WHITE + ROUND);
  r = seam(r, p.y - 0.15);
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.21, 0.0), 0.19, 0.015, 0.008), STEEL + ROUND));
  vec3 c = p - vec3(0.0, 0.28, 0.0);
  r = U(r, M(sdChamferBox(c, vec3(0.13, 0.045, 0.13), 0.015), LILAC));
  vec3 cq = faceY(c, 0.045);
  r = U(r, M(sdChamferBox(cq - vec3(0.0, 0.012, 0.0), vec3(0.07, 0.012, 0.07), 0.006), LILAC));
  vec3 px = vec3(abs(c.x) - 0.15, c.y + 0.01, repC(c.z, 0.06, 4.0));
  r = U(r, M(sdBox(px, vec3(0.022, 0.008, 0.012)), STEEL));
  vec3 pz = vec3(repC(c.x, 0.06, 4.0), c.y + 0.01, abs(c.z) - 0.15);
  r = U(r, M(sdBox(pz, vec3(0.012, 0.008, 0.022)), STEEL));
  vec3 f = faceX(rotY(p, -PI * 0.25), 0.24);
  r = U(r, led(f - vec3(0.0, 0.0, 0.1), 0.026, LIME));
  return r;
}

vec2 tray(vec3 p) {
  vec3 q = p - vec3(0.0, 0.07, 0.0);
  vec2 r = M(max(sdRoundBox(q, vec3(0.22, 0.07, 0.27), 0.015), -sdBox(q - vec3(0.0, 0.05, 0.0), vec3(0.19, 0.07, 0.24))), WHITE);
  r = seam(r, q.y + 0.03);
  r = U(r, screw(faceX(q, 0.22) - vec3(0.17, 0.0, 0.0), 0.016));
  r = U(r, screw(faceX(q, 0.22) - vec3(-0.17, 0.0, 0.0), 0.016));
  for (int i = 0; i < 3; i++) {
    float fi = float(i);
    vec3 c = rotY(q - vec3(0.0, -0.025 + 0.035 * fi, 0.0), 0.08 * sin(fi * 2.3));
    vec2 k = M(sdRoundBox(c, vec3(0.16, 0.014, 0.21), 0.01), WHITE);
    vec3 kq = faceY(c, 0.014);
    k = decal(k, kq, sdRect(kq.xz - vec2(0.0, -0.12), vec2(0.1, 0.035), 0.03), LIME);
    k = decal(k, kq, sdRect(kq.xz - vec2(-0.02, 0.0), vec2(0.09, 0.014), 0.014), STEEL);
    k = decal(k, kq, sdRect(kq.xz - vec2(0.0, 0.07), vec2(0.11, 0.014), 0.014), STEEL);
    r = U(r, k);
  }
  return r;
}

vec2 cart(vec3 q, float load) {
  vec2 r = M(sdRoundBox(q - vec3(0.0, 0.07, 0.0), vec3(0.11, 0.035, 0.1), 0.012), WHITE);
  r = seam(r, q.y - 0.07);
  r = inset(r, faceY(q, 0.105), vec2(0.085, 0.075), 0.012, 0.02, GRAPHITE);
  vec3 w = vec3(abs(q.x) - 0.07, q.y - 0.045, abs(q.z) - 0.1);
  r = U(r, M(sdCylinder(rotX(w, PI * 0.5), 0.03, 0.015), GRAPHITE + SOLID));
  vec3 l = q - vec3(0.0, 0.105, 0.0);
  if (load < 0.5) {
    r = U(r, M(sdRoundBox(l - vec3(0.0, 0.02, 0.0), vec3(0.09, 0.02, 0.08), 0.01), WHITE));
    r = U(r, M(sdRoundBox(l - vec3(0.0, 0.06, 0.0), vec3(0.09, 0.02, 0.08), 0.01), WHITE));
    r = paint(r, max(-l.y, l.x + 0.05), CORAL);
  } else if (load < 1.5) {
    r = U(r, M(sdChamferBox(l - vec3(0.0, 0.035, 0.0), vec3(0.075, 0.035, 0.075), 0.012), LILAC));
  } else if (load < 2.5) {
    r = U(r, M(sdRoundBox(l - vec3(0.0, 0.015, 0.0), vec3(0.09, 0.015, 0.08), 0.008), WHITE));
    r = paint(r, max(-l.y, l.x + 0.04), LIME);
  }
  return r;
}

vec2 cartAt(vec3 p, float a, float load) {
  vec2 cs = vec2(cos(a), sin(a));
  vec3 d = p - vec3(cs.x * TR, 0.135, cs.y * TR);
  vec3 q = vec3(dot(d.xz, vec2(-cs.y, cs.x)), d.y, dot(d.xz, cs));
  vec2 c = cart(q / 1.25, load);
  return vec2(c.x * 1.25, c.y);
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.09, 0.0), vec3(1.55, 0.1, 1.55), r.x)) {
    vec2 b = M(sdRoundCylinder(p - vec3(0.0, 0.06, 0.0), 1.5, 0.06, 0.03), WHITE + ROUND);
    b = seam(b, p.y - 0.07);
    r = U(r, b);
    r = U(r, screw(faceY(polarY(p, 16.0), 0.12) - vec3(1.42, 0.0, 0.0), 0.022));
    vec3 s = polarY(rotY(p, 0.065), 48.0);
    r = U(r, M(sdBox(s - vec3(TR, 0.13, 0.0), vec3(0.1, 0.012, 0.018)), GRAPHITE));
    r = groove(r, length(p.xz) - 1.32, 0.006, 0.01);
    vec3 tr = vec3(p.x, p.y - 0.155, p.z);
    r = U(r, M(min(sdTorus(tr, TR - 0.065, 0.014), sdTorus(tr, TR + 0.065, 0.014)), STEEL + SOLID));
  }
  if (near(p - vec3(0.0, 0.3, 0.0), vec3(0.28, 0.3, 0.28), r.x)) {
    vec2 h = M(sdRoundCylinder(p - vec3(0.0, 0.27, 0.0), 0.24, 0.15, 0.02), WHITE + ROUND);
    h = seam(h, p.y - 0.3);
    vec3 hf = faceX(rotY(p, -PI * 0.25), 0.24);
    h = vents(h, hf - vec3(-0.06, 0.0, 0.21), 3.0, 0.05, 0.08, 0.02, GRAPHITE);
    r = U(r, h);
    r = U(r, led(hf - vec3(0.08, 0.0, 0.21), 0.026, LIME));
    r = U(r, M(sdRoundCylinder(p - vec3(0.0, 0.44, 0.0), 0.15, 0.025, 0.012), BLUE + ROUND));
    r = U(r, M(sdCylinder(p - vec3(0.0, 0.52, 0.0), 0.012, 0.07), GRAPHITE + ROUND));
    r = U(r, M(sdSphere(p - vec3(0.0, 0.6, 0.0), 0.03), LIME + SOLID));
  }
  if (near(p - vec3(-TR, 0.5, 0.0), vec3(0.35, 0.5, 0.4), r.x)) r = U(r, oven(p - vec3(-TR, 0.1, 0.0)));
  if (near(p - vec3(0.0, 0.3, 1.2), vec3(0.3, 0.25, 0.25), r.x)) r = U(r, rows(p - vec3(0.0, 0.12, 1.2)));
  if (near(p - vec3(0.0, 0.3, -1.18), vec3(0.3, 0.3, 0.3), r.x)) r = U(r, model(p - vec3(0.0, 0.12, -1.18)));
  if (near(p - vec3(1.2, 0.2, 0.0), vec3(0.25, 0.15, 0.3), r.x)) r = U(r, tray(p - vec3(1.2, 0.12, 0.0)));
  if (sdTorus(p - vec3(0.0, 0.25, 0.0), TR, 0.3) < r.x) {
    r = U(r, cartAt(p, PI * 0.66, 0.0));
    r = U(r, cartAt(p, PI * 1.34, 1.0));
    r = U(r, cartAt(p, PI * 1.73, 2.0));
    r = U(r, cartAt(p, PI * 0.27, 3.0));
  }
  return r;
}
