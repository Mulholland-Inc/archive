#pragma frame {"set":["business"],"label":"Accounting office","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .6, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .32, .42));
  r = U(r, box(p, V(0,1.2,0), V(.44,.52,.08), .05, PALE));
  r = U(r, box(p, V(0,1.5,.08), V(.34,.13,.015), .02, DARK));
  for (int y = 0; y < 2; y++) for (int x = 0; x < 3; x++) { r = U(r, box(p, V(-.26 + float(x) * .26,.96 + float(y) * .2,.08), V(.09,.07,.02), .02, TONE)); }
  return r;
}
