#pragma frame {"size":[1200,960],"center":[0.0,0.69,0.0],"zoom":135,"trim":false,"bounds":[[-4.0,-0.1,-4.0],[4.0,3.2,4.0]]}

vec2 platform(vec3 p) {
  vec2 r = M(sdRoundBox(p - vec3(0.0, 0.05, 0.0), vec3(1.9, 0.05, 1.9), 0.02), STEEL);
  vec2 deck = M(sdRoundBox(p - vec3(0.0, 0.15, 0.0), vec3(1.88, 0.05, 1.88), 0.015), WHITE);
  vec2 g = abs(mod(p.xz, 0.76) - 0.38);
  deck = groove(deck, min(g.x, g.y), 0.012, 0.03);
  r = U(r, deck);
  vec2 apron = M(max(sdRoundBox(p - vec3(0.0, 0.06, 0.0), vec3(2.4, 0.06, 2.4), 0.02), -sdBox(p, vec3(1.89, 1.0, 1.89))), PAPER);
  apron = paint(apron, p.y - 0.04, STEEL);
  return U(r, apron);
}

vec2 envelope(vec3 p, float m) {
  vec2 e = M(sdBox(p, vec3(0.2, 0.016, 0.14)), m);
  return seam(e, p.z + 0.14 - (0.2 - abs(p.x)) * 0.6);
}

vec2 parcel(vec3 p, float s) {
  vec2 b = M(sdRoundBox(p - vec3(0.0, s, 0.0), vec3(s), 0.015), PAPER);
  return seam(b, s * 0.18 - abs(p.x));
}

vec2 stack(vec3 p, vec2 h, float n, float m) {
  vec2 s = M(sdBox(p - vec3(0.0, n * 0.5, 0.0), vec3(h.x, n * 0.5, h.y)), m);
  return stripes(s, p.y, 0.03);
}

vec2 belt(vec3 q, float L) {
  vec3 b = q - vec3(L * 0.5, 0.3, 0.0);
  vec2 r = M(sdRoundBox(b, vec3(L * 0.5, 0.06, 0.26), 0.02), STEEL);
  r = U(r, M(sdBox(b - vec3(0.0, 0.065, 0.0), vec3(L * 0.5 - 0.03, 0.014, 0.2)), LIME));
  vec3 l = vec3(repC(q.x - L * 0.5, 0.7, floor(L / 0.7)), q.y - 0.1, abs(q.z) - 0.18);
  return U(r, M(sdBox(l, vec3(0.035, 0.2, 0.035)), STEEL));
}

vec2 drawerOut(vec3 d, vec2 h, float ext) {
  vec3 b = d - vec3(0.0, ext * 0.5 - 0.1, 0.0);
  float box = sdBox(b, vec3(h.x * 0.92, ext * 0.5 + 0.1, h.y * 0.85));
  float cut = sdBox(b - vec3(0.0, -0.03, 0.08), vec3(h.x * 0.8, ext * 0.5 + 0.05, h.y * 0.85));
  vec2 r = M(max(box, -cut), WHITE);
  vec3 e = d - vec3(0.0, ext, 0.0);
  r = U(r, M(extrude(sdRect(e.xz, h, 0.02), e.y - 0.015, 0.015), WHITE));
  r = U(r, M(sdBox(e - vec3(0.0, 0.03, h.y * 0.4), vec3(h.x * 0.4, 0.016, 0.025)), STEEL));
  vec3 s = b - vec3(0.0, -0.02, 0.08 - h.y * 0.85);
  vec2 st = M(sdBox(s - vec3(0.0, 0.0, 0.13), vec3(h.x * 0.68, ext * 0.42, 0.13)), WHITE);
  return U(r, stripes(st, s.z, 0.032));
}

vec2 drawers(vec2 r, vec3 f, vec2 n, vec2 pitch, vec2 open) {
  vec2 id = vec2(repI(f.x, pitch.x, n.x), repI(f.z, pitch.y, n.y));
  vec3 d = vec3(repC(f.x, pitch.x, n.x), f.y, repC(f.z, pitch.y, n.y));
  vec2 h = pitch * 0.5 - 0.035;
  if (id == open) return Sc(r, M(max(sdRect(d.xz, h, 0.0), -(d.y + 0.35)), GRAPHITE));
  r = U(r, plate(d, h, 0.02, 0.014, WHITE));
  return U(r, M(sdBox(d - vec3(0.0, 0.03, h.y * 0.4), vec3(h.x * 0.4, 0.016, 0.025)), STEEL));
}

vec3 cellAt(vec2 n, vec2 pitch, vec2 id) { return vec3((id.x - 0.5 * (n.x - 1.0)) * pitch.x, 0.0, (id.y - 0.5 * (n.y - 1.0)) * pitch.y); }

vec2 storage(vec3 g) {
  vec3 c = g - vec3(-1.2, 0.62, 0.75);
  vec2 r = M(sdBox(c, vec3(0.7, 0.62, 0.75)), WHITE);
  r = U(r, M(sdRoundBox(g - vec3(-1.2, 1.28, 0.75), vec3(0.74, 0.04, 0.79), 0.015), WHITE));
  vec2 n = vec2(3.0, 3.0), pz = vec2(0.44, 0.38), px = vec2(0.48, 0.38);
  vec3 fz = faceZ(c, 0.75);
  r = drawers(r, fz, n, pz, vec2(1.0, 1.0));
  r = U(r, drawerOut(fz - cellAt(n, pz, vec2(1.0, 1.0)), pz * 0.5 - 0.035, 0.36));
  vec3 fx = faceX(c, 0.7);
  r = drawers(r, fx, n, px, vec2(2.0, 1.0));
  r = U(r, drawerOut(fx - cellAt(n, px, vec2(2.0, 1.0)), px * 0.5 - 0.035, 0.3));
  vec3 k = g - vec3(-1.65, 1.32, 0.35);
  r = U(r, M(sdCylinder(k - vec3(0.0, 0.4, 0.0), 0.11, 0.4), WHITE + ROUND));
  r = U(r, M(sdCylinder(k - vec3(0.0, 0.7, 0.0), 0.125, 0.05), LIME + ROUND));
  r = U(r, M(sdCylinder(k - vec3(0.0, 0.82, 0.0), 0.14, 0.02), STEEL + ROUND));
  return r;
}

vec2 tooth(vec3 q) {
  float d = max(sdBox(q - vec3(0.25, 0.2, 0.0), vec3(0.25, 0.2, 0.78)), (0.5 * q.y - 0.4 * q.x) * 1.56173762);
  return paint(M(d, WHITE), 0.488 - q.x, LILAC);
}

vec2 hall(vec3 g) {
  vec3 c = g - vec3(-0.35, 0.7, -0.8);
  vec2 r = M(sdBox(c, vec3(1.55, 0.7, 0.8)), WHITE);
  r = paint(r, g.y - 0.1, STEEL);
  r = U(r, M(sdRoundBox(g - vec3(-1.1, 1.43, -0.8), vec3(0.83, 0.03, 0.83), 0.012), WHITE));
  vec3 fz = faceZ(c, 0.8);
  r = Sc(r, M(max(sdRect(fz.xz - vec2(0.0, 0.47), vec2(1.45, 0.08), 0.0), -(fz.y + 0.04)), LILAC));
  vec3 dz = fz - vec3(0.35, 0.0, -0.32);
  r = Sc(r, M(max(sdRect(dz.xz, vec2(0.3, 0.3), 0.0), -(dz.y + 0.6)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(dz.xz, vec2(0.36, 0.36), 0.03), -sdRect(dz.xz, vec2(0.3, 0.3), 0.0)), dz.y - 0.02, 0.02), STEEL));
  vec3 fx = faceX(c, 1.55);
  vec3 s = fx - vec3(0.0, 0.0, 0.06);
  r = U(r, M(extrude(sdRect(s.xz, vec2(0.7, 0.44), 0.04), s.y - 0.025, 0.025), STEEL));
  r = Sc(r, M(max(sdRect(s.xz, vec2(0.62, 0.36), 0.02), -(s.y + 0.0)), GRAPHITE));
  vec2 id = vec2(repI(s.x, 0.25, 5.0), repI(s.z, 0.24, 3.0));
  vec3 q = vec3(repC(s.x, 0.25, 5.0), s.y + 0.01, repC(s.z, 0.24, 3.0));
  float lit = mod(id.x * 2.0 + id.y * 3.0, 4.0);
  float m = lit < 0.5 ? LIME : lit < 2.5 ? SKY : LILAC;
  r = U(r, plate(q, vec2(0.1, 0.09), 0.015, 0.012, m));
  for (int i = 0; i < 3; i++) r = U(r, tooth(g - vec3(-0.3 + 0.5 * float(i), 1.4, -0.8)));
  return r;
}

vec2 silo(vec3 p) {
  vec2 r = M(sdRoundCylinder(p - vec3(0.0, 0.04, 0.0), 0.62, 0.04, 0.015), STEEL + ROUND);
  r = U(r, M(sdCylinder(p - vec3(0.0, 0.53, 0.0), 0.48, 0.44), GRAPHITE + ROUND));
  r = U(r, M(sdRoundCylinder(vec3(p.x, repC(p.y - 0.55, 0.5, 2.0), p.z), 0.56, 0.21, 0.05), WHITE + ROUND));
  r = U(r, M(sdRoundCylinder(p - vec3(0.0, 1.04, 0.0), 0.24, 0.04, 0.015), STEEL + ROUND));
  return U(r, led(p - vec3(0.0, 1.08, 0.0), 0.08, LIME));
}

vec2 intake(vec3 g) {
  vec3 c = g - vec3(0.8, 0.5, 0.65);
  vec2 r = M(sdBox(c, vec3(0.5, 0.5, 0.65)), WHITE);
  r = paint(r, g.y - 0.1, STEEL);
  r = U(r, M(sdRoundBox(g - vec3(0.8, 1.03, 0.65), vec3(0.56, 0.04, 0.71), 0.02), WHITE));
  vec3 fx = faceX(c, 0.5) - vec3(0.0, 0.0, -0.1);
  r = Sc(r, M(max(sdRect(fx.xz, vec2(0.36, 0.26), 0.0), -(fx.y + 0.6)), GRAPHITE));
  r = U(r, M(extrude(max(sdRect(fx.xz, vec2(0.43, 0.33), 0.04), -sdRect(fx.xz, vec2(0.36, 0.26), 0.0)), fx.y - 0.025, 0.025), STEEL));
  vec3 aw = rotZ(g - vec3(1.42, 0.82, 0.55), -0.35);
  r = U(r, M(sdBox(aw, vec3(0.16, 0.025, 0.5)), STEEL));
  vec3 fz = faceZ(c, 0.65);
  r = U(r, plate(fz - vec3(0.0, 0.0, -0.05), vec2(0.3, 0.32), 0.02, 0.014, WHITE));
  r = U(r, M(sdBox(fz - vec3(0.2, 0.03, -0.05), vec3(0.025, 0.016, 0.08)), STEEL));
  vec3 v = g - vec3(0.65, 1.07, 0.5);
  vec2 vent = M(sdRoundBox(v - vec3(0.0, 0.16, 0.0), vec3(0.22, 0.16, 0.22), 0.02), WHITE);
  vent = vents(vent, faceX(v - vec3(0.0, 0.16, 0.0), 0.22), 4.0, 0.09, 0.16, 0.04, GRAPHITE);
  r = U(r, vent);
  r = U(r, M(sdRoundBox(v - vec3(0.0, 0.35, 0.0), vec3(0.27, 0.03, 0.27), 0.015), STEEL));
  return r;
}

vec2 inbound(vec3 g) {
  vec3 q = g - vec3(1.15, 0.0, 0.55);
  vec2 r = belt(q, 1.2);
  vec3 t = q - vec3(0.0, 0.377, 0.0);
  r = U(r, envelope(rotY(t - vec3(0.38, 0.016, 0.0), 0.15), WHITE));
  r = U(r, parcel(rotY(t - vec3(0.8, 0.0, 0.0), -0.2), 0.12));
  r = U(r, envelope(rotY(t - vec3(1.1, 0.016, 0.02), -0.1), WHITE));
  return r;
}

vec2 outbound(vec3 g) {
  vec3 q = g - vec3(0.0, 0.0, 0.05);
  vec2 r = belt(vec3(q.z, q.y, -q.x), 2.1);
  vec3 t = q - vec3(0.0, 0.377, 0.0);
  r = U(r, stack(t - vec3(0.0, 0.0, 0.95), vec2(0.15, 0.11), 0.19, WHITE));
  r = U(r, stack(t - vec3(0.0, 0.0, 1.65), vec2(0.15, 0.11), 0.13, SKY));
  return r;
}

vec2 map(vec3 p) {
  vec2 r = platform(p);
  vec3 g = p - vec3(0.0, 0.2, 0.0);
  if (near(g - vec3(-0.35, 1.0, -0.8), vec3(1.6, 1.0, 0.85), r.x)) r = U(r, hall(g));
  if (near(g - vec3(-1.1, 2.0, -0.8), vec3(0.65, 0.6, 0.65), r.x)) r = U(r, silo(g - vec3(-1.1, 1.46, -0.8)));
  if (near(g - vec3(-1.15, 1.0, 0.8), vec3(0.8, 1.0, 0.85), r.x)) r = U(r, storage(g));
  if (near(g - vec3(0.85, 0.75, 0.65), vec3(0.65, 0.75, 0.75), r.x)) r = U(r, intake(g));
  if (near(g - vec3(1.75, 0.35, 0.55), vec3(0.65, 0.3, 0.3), r.x)) r = U(r, inbound(g));
  if (near(g - vec3(0.0, 0.35, 1.1), vec3(0.3, 0.4, 1.1), r.x)) r = U(r, outbound(g));
  return r;
}
