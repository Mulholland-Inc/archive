#pragma frame {"set":["business"],"label":"Law firm","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.05,.05), V(.76,.05,.5), .01, TONE));
  r = U(r, box(p, V(0,.42,-.05), V(.58,.34,.32), .01, PALE));
  r = U(r, can(p, V(-.5,.42,.36), .06, .32, PALE));
  r = U(r, can(p, V(-.25,.42,.36), .06, .32, PALE));
  r = U(r, can(p, V(.25,.42,.36), .06, .32, PALE));
  r = U(r, can(p, V(.5,.42,.36), .06, .32, PALE));
  r = U(r, door(p, 0., .09, .34, .28));
  r = U(r, box(p, V(0,.78,.1), V(.66,.04,.4), .01, TONE));
  vec3 q = p - V(0,.82,0);
  r = U(r, box(q, V(0,.5,0), V(.04,.5,.04), .01, PALE));
  r = U(r, box(q, V(0,.92,0), V(.62,.03,.04), .01, PALE));
  r = U(r, stick(mirX(q), V(.56,.92,0), V(.56,.56,0), .012, TONE));
  r = U(r, M(max(sdEllipsoid(mirX(q) - V(.56,.54,0), V(.26,.16,.2)), mirX(q).y - .54), PALE));
  return r;
}
