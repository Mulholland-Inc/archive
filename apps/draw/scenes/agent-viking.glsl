#pragma frame {"label":"Viking"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., .9, 0.), .64, PALE);
  h = eyes(h, p, .25, .8, .15);
  vec2 r = h;
  vec3 q = p - V(0., 1.04, 0.);
  r = U(r, M(max(sdEllipsoid(q, V(.7, .66, .68)), .08 - q.y), MID));
  r = U(r, M(sdRoundCylinder(p - V(0., 1.14, 0.), .7, .06, .04), TONE));
  r = U(r, ball(p, V(0., .66, .62), .09, MID));
  float beard = smin(sdEllipsoid(p - V(0., .3, .14), V(.56, .36, .36)), sdEllipsoid(p - V(0., .06, .14), V(.24, .18, .2)), .2);
  r = U(r, M(max(beard, p.y - .62 + .2 * abs(p.x)), TONE));
  r = U(r, M(sdCone(rotZ(m - V(.78, 1.46, 0.), -.5), .3, .15, .012), PALE));
  return r;
}
