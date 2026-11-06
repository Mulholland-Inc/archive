#pragma frame {"set":["business"],"label":"Port","group":"transport"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.1,0), V(.4,.1,.4), .01, DARK));
  r = U(r, stick(mirZ(m), V(.34,.2,.3), V(.14,1.04,.1), .05, PALE));
  r = U(r, box(p, V(0,1.1,0), V(.24,.12,.24), .02, PALE));
  r = U(r, box(p, V(.1,1.26,0), V(.86,.05,.08), .01, PALE));
  r = U(r, stick(p, V(0,1.3,0), V(0,1.68,0), .04, TONE));
  r = U(r, stick(p, V(0,1.66,0), V(.78,1.3,0), .015, TONE));
  r = U(r, stick(p, V(.7,1.2,0), V(.7,.78,0), .012, TONE));
  r = U(r, box(p, V(.7,.66,0), V(.24,.12,.1), .01, TONE));
  return r;
}
