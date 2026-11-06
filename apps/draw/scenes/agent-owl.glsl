#pragma frame {"label":"Owl"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 b = oval(p, V(0., .9, 0.), V(.62, .9, .5), PALE);
  b = dab(b, m, vec2(.25, 1.22), .26, TONE);
  b = eyes(b, p, .25, 1.22, .2);
  b = line(b, p, vec2(-.05, 1.22), vec2(.05, 1.22), .022, TONE);
  for (int i = 0; i < 2; i++) {
    vec2 at = vec2(0., .68 - float(i) * .17);
    b = smile(b, vec3(abs(p.x) - .12, p.yz), at + vec2(0., .06), .1, .022, MID);
    b = smile(b, vec3(abs(abs(p.x) - .24), p.yz), at - vec2(0., .02), .1, .022, MID);
  }
  vec2 r = b;
  r = U(r, M(sdCone(rotX(p - V(0., 1.02, .52), 1.5708), .1, .09, .01), TONE));
  r = U(r, M(sdCone(rotZ(m - V(.36, 1.76, -.04), -.2), .2, .13, .012), TONE));
  r = U(r, oval(m, V(.62, .78, -.02), V(.08, .4, .2), TONE));
  r = U(r, box(m, V(.2, .05, .2), V(.13, .05, .13), .04, MID));
  return r;
}
