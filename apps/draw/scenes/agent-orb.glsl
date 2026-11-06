#pragma frame {"label":"Ringed robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = ball(p, V(0., 1., 0.), .66, PALE);
  h = oblong(h, p, vec2(0., 1.1), vec2(.44, .19), .19, DARK);
  h = dab(h, m, vec2(.2, 1.1), .12, EYE);
  vec2 r = h;
  r = U(r, M(sdTorus(rotX(p - V(0., .86, 0.), .22), .82, .055), TONE));
  r = U(r, stick(m, V(.2, 1.56, 0.), V(.3, 1.76, 0.), .03, TONE));
  r = U(r, ball(m, V(.3, 1.8, 0.), .07, PALE));
  r = U(r, cone(p, V(0., .28, 0.), .08, .05, .16, MID));
  return r;
}
