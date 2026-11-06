#pragma frame {"label":"Alarm clock robot"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 h = M(sdRoundCylinder(rotX(p - V(0., .86, 0.), 1.5708), .7, .3, .12), PALE);
  h = eyes(h, p, .28, .92, .21);
  h = smile(h, p, vec2(0., .68), .13, .018, TONE);
  vec2 r = h;
  vec3 b = rotZ(m - V(.48, 1.6, 0.), -.5);
  r = U(r, M(max(sdSphere(b, .28), -b.y - .04), TONE));
  r = U(r, stick(p, V(0., 1.55, 0.), V(0., 1.8, 0.), .03, MID));
  r = U(r, M(sdTorus(p - V(0., 1.82, 0.), .07, .03), MID));
  r = U(r, stick(m, V(.4, .28, .1), V(.54, .05, .14), .055, MID));
  return r;
}
