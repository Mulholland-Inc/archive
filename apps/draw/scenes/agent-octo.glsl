#pragma frame {"label":"Octopus"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = oval(p, V(0., 1.06, 0.), V(.72, .72, .62), PALE);
  h = eyes(h, p, .3, 1.02, .2);
  h = dab(h, p, vec2(0., .8), .055, TONE);
  h = dab(h, p, vec2(-.3, 1.54), .09, MID);
  h = dab(h, p, vec2(.14, 1.62), .06, MID);
  vec2 r = h;
  vec3 q = polarY(rotY(p, .4), 6.);
  r = U(r, M(sdCapsule(q, V(.3, .5, 0.), V(.66, .2, 0.), .15), TONE));
  r = U(r, M(sdCapsule(q, V(.66, .2, 0.), V(.84, .14, 0.), .09), TONE));
  return r;
}
