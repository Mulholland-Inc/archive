#pragma frame {"set":["business"],"label":"Diner","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.3,0), V(.82,.3,.42), .12, PALE));
  r = U(r, box(p, V(0,.36,.42), V(.7,.13,.012), .05, MID));
  r = U(r, box(p, V(0,.1,.42), V(.82,.03,.012), .012, TONE));
  r = U(r, door(p, .44, .08, .3, .43));
  r = U(r, box(p, V(0,.64,0), V(.86,.03,.46), .02, TONE));
  r = U(r, M(max(sdEllipsoid(p - V(0,1.16,0), V(.54,.42,.46)), 1.16 - p.y), PALE));
  r = U(r, can(p, V(0,1.08,0), .58, .03, TONE));
  r = U(r, can(p, V(0,.98,0), .55, .06, DARK));
  r = U(r, can(p, V(0,.84,0), .52, .08, PALE));
  return r;
}
