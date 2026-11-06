#pragma frame {"label":"Chick"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., 1.04, 0.), .64, PALE);
  h = eyes(h, p, .27, 1.12, .17);
  h = dab(h, m, vec2(.46, .9), .08, MID);
  vec2 r = h;
  r = U(r, M(sdCone(rotX(p - V(0., .92, .72), 1.5708), .13, .15, .01), TONE));
  for (int i = 0; i < 3; i++) r = U(r, oval(rotZ(p - V(0., 1.72, 0.), (float(i) - 1.) * .5), V(0., .06, 0.), V(.05, .14, .05), TONE));
  float a = atan(p.z, p.x) * 8. / PI;
  float rim = .5 + .11 * (abs(fract(a * .5) - .5) * 4. - 1.);
  r = U(r, M(max(sdEllipsoid(p - V(0., .36, 0.), V(.66, .5, .66)), (p.y - rim) * .6), MID));
  return r;
}
