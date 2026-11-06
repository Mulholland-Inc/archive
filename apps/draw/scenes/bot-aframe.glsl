#pragma frame {"set":["personal"],"label":"A-frame cabin","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(sdPrism(p, .78, 1.5, .6), PALE));
  r = U(r, M(sdPrism(p - V(0,0,.58), .42, .84, .03), MID));
  r = U(r, box(p, V(0,.2,.62), V(.11,.2,.015), .01, DARK));
  r = U(r, box(p, V(.3,1.1,-.2), V(.07,.26,.07), .01, TONE));
  return r;
}
