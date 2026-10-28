#pragma frame {"size":[800,640],"center":[0.3,0.25,0.4],"zoom":188,"bounds":[[-1.2,-0.1,-1.2],[2.4,1.0,2.4]]}

float cable(vec3 p, vec3 a, vec3 b, vec3 c, vec3 d, float r) {
  float k = 1e9;
  vec3 prev = a;
  for (int i = 1; i <= 10; i++) {
    float t = float(i) / 10.0;
    vec3 ab = mix(a, b, t), bc = mix(b, c, t), cd = mix(c, d, t);
    vec3 q = mix(mix(ab, bc, t), mix(bc, cd, t), t);
    k = min(k, sdCapsule(p, prev, q, r));
    prev = q;
  }
  return k;
}

vec2 row(vec3 p, float tag) {
  vec2 c = M(sdRoundBox(p, vec3(0.3, 0.03, 0.2), 0.02), WHITE);
  vec3 q = faceY(p, 0.03);
  c = paint(c, p.x + 0.2, tag);
  c = decal(c, q, sdRect(q.xz - vec2(0.06, -0.08), vec2(0.15, 0.018), 0.018), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.02, 0.0), vec2(0.11, 0.018), 0.018), STEEL);
  c = decal(c, q, sdRect(q.xz - vec2(0.05, 0.08), vec2(0.14, 0.018), 0.018), STEEL);
  return c;
}

vec2 stack(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.04, 0.0), vec3(0.4, 0.04, 0.3), 0.02), WHITE);
  r = seam(r, p.y - 0.05);
  r = U(r, screw(faceY(mirXZ(p), 0.08) - vec3(0.36, 0.0, 0.0), 0.022));
  r = U(r, row(p - vec3(0.0, 0.115, 0.0), LIME));
  r = U(r, row(p - vec3(0.02, 0.185, 0.0), CORAL));
  r = U(r, row(p - vec3(-0.01, 0.255, 0.0), LILAC));
  r = U(r, row(p - vec3(0.01, 0.325, 0.0), SUN));
  r = U(r, row(p - vec3(-0.01, 0.395, 0.0), CORAL));
  r = U(r, row(p - vec3(0.01, 0.465, 0.0), LILAC));
  return r;
}

vec2 chip(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.04, 0.0), vec3(0.88, 0.04, 0.88), 0.03), WHITE);
  r = seam(r, p.y - 0.045);
  vec3 bt = faceY(p, 0.08);
  vec3 bc = mirXZ(bt);
  r = U(r, screw(bc - vec3(0.8, 0.0, 0.8), 0.028));
  vec3 c = p - vec3(0.0, 0.27, 0.0);
  vec2 body = M(sdChamferBox(c, vec3(0.62, 0.17, 0.62), 0.04), WHITE);
  body = seam(body, c.y + 0.07);
  vec3 fx = faceX(p, 0.62);
  body = vents(body, fx - vec3(-0.3, 0.0, 0.3), 5.0, 0.065, 0.1, 0.024, GRAPHITE);
  vec3 fz = faceZ(p, 0.62);
  body = vents(body, fz - vec3(0.3, 0.0, 0.3), 5.0, 0.065, 0.1, 0.024, GRAPHITE);
  r = U(r, body);
  r = U(r, led(fx - vec3(0.32, 0.0, 0.3), 0.035, LIME));
  r = U(r, screw(fz - vec3(-0.34, 0.0, 0.3), 0.022));
  vec3 px = vec3(abs(p.x) - 0.62, p.y, repC(p.z, 0.13, 9.0));
  r = U(r, M(sdBox(px - vec3(0.04, 0.14, 0.0), vec3(0.04, 0.012, 0.025)), STEEL));
  r = U(r, M(sdBox(px - vec3(0.07, 0.11, 0.0), vec3(0.012, 0.03, 0.025)), STEEL));
  vec3 pz = vec3(repC(p.x, 0.13, 9.0), p.y, abs(p.z) - 0.62);
  r = U(r, M(sdBox(pz - vec3(0.0, 0.14, 0.04), vec3(0.025, 0.012, 0.04)), STEEL));
  r = U(r, M(sdBox(pz - vec3(0.0, 0.11, 0.07), vec3(0.025, 0.03, 0.012)), STEEL));
  vec3 hs = p - vec3(0.0, 0.46, -0.3);
  vec2 sink = M(sdBox(hs, vec3(0.56, 0.025, 0.26)), STEEL);
  r = U(r, sink);
  vec3 fin = vec3(repC(p.x, 0.1, 11.0), p.y - 0.57, p.z + 0.3);
  r = U(r, M(sdBox(fin, vec3(0.016, 0.09, 0.25)), WHITE));
  vec3 t = faceY(p, 0.44) - vec3(0.0, 0.0, 0.31);
  r = U(r, M(extrude(max(sdRect(t.xz, vec2(0.52, 0.25), 0.04), -sdRect(t.xz, vec2(0.4, 0.15), 0.02)), t.y - 0.015, 0.015), STEEL));
  r = inset(r, t, vec2(0.42, 0.17), 0.02, 0.08, GRAPHITE);
  vec3 k = t - vec3(0.0, -0.045, 0.0);
  vec2 core = M(sdChamferBox(k, vec3(0.34, 0.035, 0.12), 0.015), LILAC);
  vec3 kq = faceY(k, 0.035);
  vec3 pad = vec3(repC(kq.x, 0.15, 4.0), kq.y - 0.015, repC(kq.z, 0.12, 2.0));
  core = U(core, M(sdChamferBox(pad, vec3(0.05, 0.015, 0.04), 0.008), LILAC));
  r = U(r, core);
  r = U(r, screw(mirXZ(faceY(p, 0.47) - vec3(0.0, 0.0, 0.31)) - vec3(0.47, 0.0, 0.21), 0.02));
  return r;
}

vec2 sink(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.25, 0.0), vec3(0.3, 0.25, 0.32), 0.03), WHITE);
  r = seam(r, p.y - 0.12);
  vec3 fx = faceX(p, 0.3);
  r = inset(r, fx - vec3(0.06, 0.0, 0.32), vec2(0.16, 0.08), 0.02, 0.02, GRAPHITE);
  vec3 tp = faceY(p, 0.5);
  r = inset(r, tp - vec3(0.0, 0.0, 0.0), vec2(0.04, 0.22), 0.02, 0.06, GRAPHITE);
  vec2 o = M(1e9, 0.0);
  o = U(o, led(fx - vec3(-0.2, 0.0, 0.32), 0.03, LIME));
  o = U(o, led(fx - vec3(-0.2, 0.0, 0.22), 0.03, STEEL));
  r = U(r, o);
  r = U(r, screw(vec3(fx.x, fx.y, fx.z) - vec3(0.22, 0.0, 0.08), 0.022));
  r = U(r, screw(vec3(fx.x, fx.y, fx.z) - vec3(-0.22, 0.0, 0.08), 0.022));
  r = vents(r, faceZ(p, 0.32) - vec3(0.0, 0.0, 0.3), 4.0, 0.065, 0.12, 0.024, GRAPHITE);
  vec3 cd = rotZ(p - vec3(0.0, 0.62, 0.0), 0.0);
  vec2 card = M(sdRoundBox(vec3(cd.x, cd.y, cd.z), vec3(0.025, 0.14, 0.2), 0.012), WHITE);
  vec3 cq = faceX(cd, 0.025);
  card = decal(card, cq, sdRect(cq.xz - vec2(0.0, 0.06), vec2(0.13, 0.035), 0.03), LILAC);
  card = decal(card, cq, sdRect(cq.xz - vec2(0.02, -0.03), vec2(0.11, 0.015), 0.015), STEEL);
  r = U(r, card);
  return r;
}

vec2 map(vec3 p) {
  vec2 r = M(1e9, 0.0);
  if (near(p - vec3(0.0, 0.4, 0.0), vec3(1.0, 0.45, 1.0), r.x)) r = U(r, chip(p));
  vec3 sp = p - vec3(-0.15, 0.0, 1.75);
  if (near(sp - vec3(0.0, 0.3, 0.0), vec3(0.5, 0.4, 0.4), r.x)) r = U(r, stack(sp));
  vec3 op = p - vec3(1.75, 0.0, -0.05);
  if (near(op - vec3(0.0, 0.45, 0.0), vec3(0.4, 0.5, 0.4), r.x)) r = U(r, sink(op));
  if (near(p - vec3(0.0, 0.3, 1.2), vec3(0.3, 0.3, 0.35), r.x)) {
    vec2 c = M(cable(p, vec3(0.0, 0.3, 0.72), vec3(0.0, 0.3, 1.05), vec3(-0.15, 0.08, 0.95), vec3(-0.15, 0.08, 1.45), 0.035), GRAPHITE + SOLID);
    c = U(c, M(sdBox(p - vec3(0.0, 0.3, 0.68), vec3(0.08, 0.05, 0.05)), STEEL));
    r = U(r, c);
  }
  if (near(p - vec3(1.1, 0.25, 0.0), vec3(0.4, 0.2, 0.3), r.x)) {
    vec2 c = M(cable(p, vec3(0.72, 0.3, 0.1), vec3(1.05, 0.3, 0.1), vec3(1.0, 0.1, -0.05), vec3(1.45, 0.1, -0.05), 0.035), GRAPHITE + SOLID);
    c = U(c, M(sdBox(p - vec3(0.68, 0.3, 0.1), vec3(0.05, 0.05, 0.08)), STEEL));
    r = U(r, c);
  }
  return r;
}
