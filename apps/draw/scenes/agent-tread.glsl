#pragma frame {"label":"Tracked robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., 1.04, 0.), V(.72, .44, .42), .13, SHELL);
  h = oblong(h, p, vec2(0., 1.04), vec2(.56, .2), .1, DARK);
  h = dab(h, m, vec2(.27, 1.04), .115, EYE);
  vec2 r = h;
  r = U(r, box(m, V(.4, 1.5, 0.), V(.05, .05, .05), .02, TONE));
  r = U(r, can(p, V(0., .54, 0.), .2, .07, MID));
  r = U(r, M(extrude(sdRect(p.zy - vec2(0., .24), vec2(.56, .24), .22), m.x - .46, .18) - .02, TONE));
  r = U(r, canX(m, V(.67, .24, .3), .1, .02, PALE));
  r = U(r, canX(m, V(.67, .24, -.3), .1, .02, PALE));
  r = U(r, box(p, V(0., .3, 0.), V(.3, .1, .3), .03, MID));
  return r;
}
