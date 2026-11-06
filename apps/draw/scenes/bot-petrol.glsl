#pragma frame {"set":["business"],"label":"Petrol station","group":"auto"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(-.42,.2,.0), V(.34,.2,.44), .01, PALE));
  r = U(r, stick(mirZ(m), V(.58,0,.3), V(.58,.92,.3), .05, TONE));
  r = U(r, box(p, V(0,1.0,0), V(.92,.07,.56), .03, PALE));
  r = U(r, box(p, V(0,1.0,.56), V(.84,.03,.012), .01, TONE));
  r = U(r, box(p, V(.32,.32,0), V(.15,.32,.13), .03, PALE));
  r = U(r, box(p, V(.32,.48,.13), V(.1,.08,.012), .01, DARK));
  r = U(r, stick(p, V(.47,.5,0), V(.56,.2,.1), .02, DARK));
  return r;
}
