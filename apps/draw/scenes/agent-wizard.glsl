#pragma frame {"label":"Wizard"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., .72, 0.), .56, PALE);
  h = eyes(h, p, .22, .76, .15);
  vec2 r = h;
  r = U(r, ball(p, V(0., .58, .54), .08, MID));
  float beard = smin(sdEllipsoid(p - V(0., .36, .14), V(.46, .4, .32)), sdEllipsoid(p - V(0., .1, .16), V(.2, .2, .18)), .15);
  r = U(r, M(max(beard, p.y - .5 + .1 * abs(p.x)), MID));
  r = U(r, M(sdRoundCylinder(p - V(0., 1.2, 0.), .62, .05, .04), TONE));
  vec3 q = p - V(0., 1.66, 0.);
  vec2 c = M(sdCone(rotZ(q, q.y * .25), vec2(.48, .42).x, .42, .03), TONE);
  c = dab(c, p, vec2(.14, 1.56), .08, EYE);
  c = dab(c, p, vec2(-.12, 1.4), .05, EYE);
  r = U(r, c);
  return r;
}
