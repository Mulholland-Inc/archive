#pragma frame {"set":["business"],"label":"Construction yard","group":"trade"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.04,0), V(.36,.04,.36), .01, DARK));
  r = U(r, box(p, V(0,.74,0), V(.1,.7,.1), .01, PALE));
  r = U(r, box(p, V(.08,1.52,0), V(.74,.05,.08), .02, PALE));
  r = U(r, box(p, V(-.48,1.4,0), V(.14,.1,.1), .02, DARK));
  r = U(r, box(p, V(.02,1.6,.0), V(.14,.14,.12), .03, PALE));
  r = U(r, stick(p, V(.66,1.5,0), V(.66,.9,0), .01, DARK));
  r = U(r, box(p, V(.66,.82,0), V(.1,.08,.08), .02, TONE));
  return r;
}
