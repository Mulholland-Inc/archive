#pragma frame {"set":["family"],"label":"Manor","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.44,0), V(.84,.44,.36), .01, PALE));
  r = U(r, pitch(p, .88, .92, .3, .42, TONE));
  r = U(r, box(p, V(0,.5,.3), V(.26,.5,.14), .01, PALE));
  r = U(r, M(sdPrism(p - V(0,1.0,.3), .34, .3, .17), TONE));
  r = U(r, box(mirZ(m), V(.62,1.14,.1), V(.07,.22,.07), .01, DARK));
  r = U(r, box(p, V(0,.16,.44), V(.09,.16,.015), .01, DARK));
  for (int i = 0; i < 2; i++) { r = U(r, box(m, V(.56,.24 + float(i) * .38,.36), V(.09,.11,.012), .01, MID)); }
  return r;
}
