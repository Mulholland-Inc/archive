#pragma frame {"label":"Monster"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = box(p, V(0., .8, 0.), V(.74, .7, .54), .34, PALE);
  h = eyes(h, p, .28, 1.08, .2);
  h = oblong(h, p, vec2(0., .58), vec2(.46, .17), .15, DARK);
  h = oblong(h, m, vec2(.2, .69), vec2(.07, .06), .01, EYE);
  h = oblong(h, m, vec2(.08, .46), vec2(.05, .05), .01, EYE);
  vec2 r = h;
  r = U(r, M(sdCone(rotZ(m - V(.5, 1.6, 0.), -.45), .24, .13, .012), TONE));
  r = U(r, oval(m, V(.8, .62, .04), V(.14, .24, .17), MID));
  r = U(r, oval(m, V(.3, .06, .14), V(.2, .07, .2), TONE));
  return r;
}
