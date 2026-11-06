#pragma frame {"set":["business"],"label":"Museum","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.44,0), V(.72,.44,.42), .01, PALE));
  r = U(r, M(sdEllipsoid(p - V(0,.88,0), V(.46,.46,.34)), MID));
  for (int i = 0; i < 3; i++) { r = U(r, stick(p, V(-.3 + float(i) * .3,.86,.2), V(-.3 + float(i) * .3,1.3 - abs(float(i) - 1.) * .12,.0), .012, TONE)); }
  r = U(r, box(p, V(0,.16,.42), V(.18,.16,.015), .01, DARK));
  for (int i = 0; i < 2; i++) { r = U(r, can(m, V(.42 + float(i) * .2,.44,.46), .05, .42, MID)); }
  return r;
}
