#pragma frame {"label":"Hedgehog"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., .82, 0.), V(.76, .68, .6), TONE);
  h = oblong(h, p, vec2(0., .68), vec2(.58, .44), .4, PALE);
  h = eyes(h, p, .28, .84, .16);
  h = dab(h, p, vec2(0., .66), .07, DARK);
  h = smile(h, p, vec2(0., .58), .08, .014, DARK);
  vec2 r = h;
  for (int i = 0; i < 9; i++) {
    float a = (float(i) - 4.) * .42;
    vec3 q = rotZ(p - V(0., .82, -.06), a) - V(0., .72, 0.);
    r = U(r, M(sdCone(q, .17, .13, .01), TONE));
    q = rotZ(p - V(0., .82, -.34), a + .2) - V(0., .62, 0.);
    r = U(r, M(sdCone(q, .17, .13, .01), TONE));
  }
  r = U(r, oval(m, V(.26, .07, .22), V(.15, .07, .18), MID));
  return r;
}
