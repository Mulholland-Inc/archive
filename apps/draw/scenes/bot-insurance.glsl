#pragma frame {"set":["business"],"label":"Insurance office","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, M(max(sdEllipsoid(p - V(0,1.18,0), V(.74,.5,.2)), 1.18 - p.y), PALE));
  vec3 u = p - V(0,1.18,0);
  r = U(r, M(max(max(sdEllipsoid(u, V(.745,.505,.205)), -u.y), abs(abs(u.x) - .26) - .012), TONE));
  r = U(r, stick(p, V(0,.66,0), V(0,1.8,0), .03, TONE));
  return r;
}
