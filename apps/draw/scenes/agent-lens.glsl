#pragma frame {"label":"One-eyed robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = M(sdChamferBox(p - V(0., 1., 0.), V(.64, .6, .5), .14), SHELL);
  h = dab(h, p, vec2(0., 1.), .42, MID);
  h = dab(h, p, vec2(0., 1.), .33, DARK);
  h = dab(h, p, vec2(0., 1.), .2, EYE);
  h = dab(h, p, vec2(.1, 1.1), .05, EYE);
  vec2 r = h;
  r = U(r, box(m, V(.68, 1., 0.), V(.06, .2, .2), .03, TONE));
  r = U(r, box(p, V(0., 1.66, 0.), V(.24, .06, .16), .03, TONE));
  r = U(r, stick(m, V(.28, .4, 0.), V(.28, .16, 0.), .07, MID));
  r = U(r, box(m, V(.28, .07, .05), V(.16, .07, .2), .05, TONE));
  return r;
}
