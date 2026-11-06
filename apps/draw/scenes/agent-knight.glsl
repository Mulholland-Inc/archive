#pragma frame {"label":"Knight robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec3 q = p - V(0., 1., 0.);
  vec2 h = M(sdCapsule(q, V(0., -.14, 0.), V(0., .14, 0.), .62), PALE);
  h = oblong(h, p, vec2(0., 1.02), vec2(.46, .12), .03, DARK);
  h = dab(h, m, vec2(.21, 1.02), .08, EYE);
  h = line(h, p, vec2(0., .9), vec2(0., .44), .02, TONE);
  h = dab(h, m, vec2(.2, .7), .03, TONE);
  h = dab(h, m, vec2(.2, .56), .03, TONE);
  vec2 r = h;
  vec3 c = p - V(0., 1.28, 0.);
  r = U(r, M(max(abs(p.x) - .07, length(c.yz) - .62), TONE));
  r = U(r, oval(m, V(.44, .3, 0.), V(.2, .13, .2), MID));
  r = U(r, box(p, V(0., .18, 0.), V(.34, .18, .24), .08, SHELL));
  return r;
}
