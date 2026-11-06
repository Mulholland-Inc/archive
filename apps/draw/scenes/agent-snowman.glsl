#pragma frame {"label":"Snowman"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., 1.0, 0.), .58, PALE);
  h = eyes(h, p, .22, 1.08, .16);
  h = dab(h, m, vec2(.12, .78), .025, TONE);
  h = dab(h, p, vec2(0., .74), .025, TONE);
  vec2 b = ball(p, V(0., .34, 0.), .44, PALE);
  b = dab(b, p, vec2(0., .3), .04, TONE);
  b = dab(b, p, vec2(0., .12), .04, TONE);
  vec2 r = U(h, b);
  r = U(r, M(sdCone(rotX(p - V(0., .92, .72), 1.5708), .2, .12, .01), MID));
  r = U(r, M(sdRoundCylinder(p - V(0., 1.48, 0.), .54, .04, .03), TONE));
  r = U(r, M(sdRoundCylinder(p - V(0., 1.62, 0.), .32, .13, .03), TONE));
  r = U(r, M(sdTorus(p - V(0., .56, 0.), .36, .08), MID));
  r = U(r, stick(m, V(.4, .5, 0.), V(.78, .68, 0.), .025, TONE));
  return r;
}
