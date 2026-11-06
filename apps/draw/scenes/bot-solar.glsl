#pragma frame {"set":["business"],"label":"Solar farm","group":"energy"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.05,0), V(.86,.05,.62), .01, TONE));
  r = U(r, box(p, V(0,.22,-.42), V(.22,.14,.14), .01, PALE));
  for (int i = 0; i < 3; i++) for (int k = 0; k < 2; k++) {
  vec3 q = rotX(p - V(-.58 + float(i) * .58,.56 - float(k) * .3,-.1 + float(k) * .54), .55);
  r = U(r, box(q, V(0,0,0), V(.25,.02,.2), .01, PALE));
  r = U(r, box(q, V(0,.02,0), V(.012,.012,.2), .006, DARK));
  r = U(r, box(q, V(0,.02,0), V(.25,.012,.012), .006, DARK));
  r = U(r, stick(p, V(-.58 + float(i) * .58,.1,-.1 + float(k) * .54), V(-.58 + float(i) * .58,.5 - float(k) * .3,-.1 + float(k) * .54), .018, TONE));
  }
  return r;
}
