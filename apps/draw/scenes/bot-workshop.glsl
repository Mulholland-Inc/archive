#pragma frame {"set":["business"],"label":"Workshop","group":"trade"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.34,0), V(.72,.34,.42), .01, PALE));
  r = U(r, M(sdPrism(p - V(0,.68,0), .84, .22, .48), TONE));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, box(p, V(0,.26,.42), V(.22,.26,.012), .01, DARK));
  vec3 g = polarY((p - V(0,1.24,0)).xzy, 8.);
  r = U(r, canZ(p, V(0,1.24,0), .32, .07, PALE));
  r = U(r, M(sdRoundBox(g - V(.38,0,0), V(.12,.07,.09), .02), PALE));
  r = U(r, canZ(p, V(0,1.24,.05), .12, .03, DARK));
  return r;
}
