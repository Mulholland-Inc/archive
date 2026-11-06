#pragma frame {"label":"Stacked robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = M(sdRoundCylinder(p - V(0., 1.22, 0.), .68, .4, .14), PALE);
  h = oblong(h, p, vec2(0., 1.22), vec2(.5, .19), .1, DARK);
  h = dab(h, m, vec2(.24, 1.22), .11, EYE);
  vec2 r = h;
  r = U(r, can(p, V(0., 1.68, 0.), .1, .06, TONE));
  r = U(r, M(sdRoundCylinder(p - V(0., .66, 0.), .46, .17, .08), TONE));
  r = U(r, M(sdRoundCylinder(p - V(0., .28, 0.), .54, .2, .08), PALE));
  r = U(r, stick(m, V(.46, .66, 0.), V(.7, .56, 0.), .055, MID));
  r = U(r, box(m, V(.26, .05, .05), V(.16, .05, .18), .04, MID));
  return r;
}
