#pragma frame {"set":["family"],"label":"Palace","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.4,0), V(.84,.4,.36), .01, PALE));
  r = U(r, box(p, V(0,.84,0), V(.3,.04,.3), .01, TONE));
  r = U(r, can(p, V(0,1.0,0), .24, .14, PALE));
  r = U(r, M(smin(sdSphere(p - V(0,1.32,0), .3), sdCone(p - V(0,1.68,0), .12, .05, .01), .1), MID));
  for (int i = 0; i < 2; i++) { r = U(r, can(m, V(.76,.5 + float(i) * .0,.3), .1, .5, PALE)); }
  r = U(r, M(smin(sdSphere(mirZ(m) - V(.76,1.06,.3), .13), sdCone(mirZ(m) - V(.76,1.24,.3), .08, .03, .01), .06), MID));
  r = U(r, M(max(sdCylinder((p - V(0,.26,.36)).xzy, .16, .03), 0.), DARK));
  r = U(r, box(p, V(0,.12,.36), V(.16,.12,.03), .01, DARK));
  return r;
}
