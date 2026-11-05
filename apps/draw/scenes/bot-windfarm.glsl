#pragma frame {"set":["business"],"label":"Wind farm","group":"energy"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, cone(p, V(0,.7,0), .7, .1, .05, PALE));
  r = U(r, canZ(p, V(0,1.42,.02), .1, .16, PALE));
  vec3 b = p - V(0,1.42,.2);
  for (int i = 0; i < 3; i++) { vec3 q = rotZ(b, float(i) * 2.094 + .3); r = U(r, M(sdEllipsoid(q - V(.0,.42,0), V(.075,.44,.02)), MID)); }
  r = U(r, ball(b, V(0,0,.02), .07, TONE));
  r = U(r, box(p, V(0,.03,0), V(.34,.03,.34), .01, TONE));
  return r;
}
