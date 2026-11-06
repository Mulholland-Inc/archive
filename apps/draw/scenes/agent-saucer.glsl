#pragma frame {"label":"Saucer robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., .68, 0.), .64, PALE);
  h = oblong(h, p, vec2(0., .94), vec2(.42, .2), .15, DARK);
  h = dab(h, m, vec2(.19, .94), .115, EYE);
  vec2 r = h;
  r = U(r, M(sdEllipsoid(p - V(0., .42, 0.), V(1.04, .22, 1.04)), TONE));
  vec3 q = polarY(p, 8.);
  r = U(r, M(sdSphere(q - V(.92, .42, 0.), .055), PALE + 0.));
  r = U(r, cone(p, V(0., .14, 0.), .08, .2, .3, MID));
  return r;
}
