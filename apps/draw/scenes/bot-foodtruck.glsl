#pragma frame {"set":["business"],"label":"Food truck","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(-.14,.6,0), V(.6,.42,.42), .05, PALE));
  r = U(r, box(p, V(.62,.42,0), V(.22,.24,.4), .05, PALE));
  r = U(r, box(p, V(.68,.52,.0), V(.14,.1,.41), .02, MID));
  r = U(r, canZ(mirZ(p), V(-.44,.14,.38), .15, .04, DARK));
  r = U(r, canZ(mirZ(p), V(.56,.14,.38), .15, .04, DARK));
  r = U(r, box(p, V(-.14,.68,.42), V(.42,.2,.012), .02, DARK));
  vec3 q = rotX(p - V(-.14,.96,.56), .5);
  r = U(r, box(q, V(0,0,0), V(.5,.012,.17), .008, TONE));
  r = U(r, box(p, V(-.14,.46,.48), V(.46,.02,.07), .01, TONE));
  r = U(r, box(p, V(-.14,1.08,0), V(.12,.06,.1), .02, TONE));
  return r;
}
