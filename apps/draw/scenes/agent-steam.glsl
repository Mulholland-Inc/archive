#pragma frame {"label":"Steam robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., .86, 0.), V(.68, .5, .46), .22, SHELL);
  h = oblong(h, p, vec2(0., .9), vec2(.5, .24), .12, DARK);
  h = dab(h, m, vec2(.23, .9), .115, EYE);
  vec2 r = h;
  r = U(r, can(p, V(.26, 1.48, 0.), .13, .14, TONE));
  r = U(r, cone(p, V(.26, 1.68, 0.), .08, .13, .22, MID));
  r = U(r, ball(p, V(.2, 1.96, 0.), .11, PALE));
  r = U(r, ball(p, V(.04, 2.14, 0.), .15, PALE));
  r = U(r, canX(m, V(.72, .86, 0.), .14, .05, MID));
  r = U(r, box(m, V(.3, .16, .03), V(.15, .16, .2), .06, TONE));
  return r;
}
