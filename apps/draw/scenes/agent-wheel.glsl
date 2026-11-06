#pragma frame {"label":"Wheel robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., 1.08, 0.), V(.78, .4, .44), .16, SHELL);
  h = oblong(h, p, vec2(0., 1.08), vec2(.6, .22), .08, DARK);
  h = oblong(h, m, vec2(.27, 1.08), vec2(.1, .12), .03, EYE);
  vec2 r = h;
  r = U(r, stick(m, V(.42, 1.46, 0.), V(.56, 1.68, 0.), .035, TONE));
  r = U(r, can(p, V(0., .64, 0.), .12, .06, MID));
  r = U(r, stick(m, V(.15, .62, 0.), V(.15, .28, 0.), .05, TONE));
  r = U(r, canX(p, V(0., .3, 0.), .3, .09, MID));
  r = U(r, canX(p, V(0., .3, 0.), .12, .19, PALE));
  return r;
}
