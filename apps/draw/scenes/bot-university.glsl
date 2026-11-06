#pragma frame {"set":["business"],"label":"University","group":"civic"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.46,0), V(.76,.46,.42), .01, PALE));
  for (int i = 0; i < 2; i++) { r = U(r, can(m, V(.3 + float(i) * .28,.46,.46), .06, .44, MID)); }
  r = U(r, door(p, 0., .09, .3, .42));
  r = U(r, box(p, V(0,.96,0), V(.8,.04,.46), .01, TONE));
  r = U(r, can(p, V(0,1.2,0), .34, .2, TONE));
  r = U(r, M(sdRoundBox(rotY(p - V(0,1.44,0), .785), V(.58,.03,.58), .01), PALE));
  r = U(r, stick(p, V(0,1.48,0), V(.5,1.36,.0), .015, DARK));
  r = U(r, stick(p, V(.5,1.36,0), V(.5,1.1,0), .015, DARK));
  r = U(r, ball(p, V(.5,1.06,0), .05, DARK));
  return r;
}
