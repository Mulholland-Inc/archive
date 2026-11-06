#pragma frame {"label":"Light bulb robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = M(smin(sdSphere(p - V(0., 1.3, 0.), .68), sdCylinder(p - V(0., .66, 0.), .24, .2), .22), PALE);
  h = eyes(h, p, .28, 1.36, .21);
  h = line(h, m, vec2(0., 1.06), vec2(.1, 1.1), .018, TONE);
  vec2 r = h;
  for (int i = 0; i < 3; i++) r = U(r, M(sdRoundCylinder(p - V(0., .34 + float(i) * .1, 0.), .32, .045, .04), MID));
  r = U(r, cone(p, V(0., .25, 0.), .05, .14, .26, TONE));
  r = U(r, stick(m, V(.13, .22, 0.), V(.22, .06, 0.), .05, MID));
  r = U(r, stick(m, V(.3, .46, 0.), V(.54, .36, .06), .05, MID));
  return r;
}
