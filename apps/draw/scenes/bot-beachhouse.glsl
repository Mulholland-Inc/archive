#pragma frame {"set":["personal"],"label":"Beach house","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.72,0), V(.56,.32,.42), .01, PALE));
  r = U(r, M(sdPrism(p - V(0,1.04,0), .7, .32, .5), TONE));
  r = U(r, box(p, V(0,.42,.14), V(.7,.025,.42), .01, TONE));
  r = U(r, stick(mirZ(m), V(.5,0,.42), V(.5,.42,.42), .05, PALE));
  for (int i = 0; i < 3; i++) { r = U(r, box(p, V(0,.3 - float(i) * .12,.6 + float(i) * .08), V(.12,.012,.04), .006, MID)); }
  return r;
}
