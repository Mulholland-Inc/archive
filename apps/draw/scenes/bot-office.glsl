#pragma frame {"set":["business"],"label":"Office","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.76,0), V(.52,.76,.44), .01, PALE));
  for (int y = 0; y < 3; y++) for (int x = 0; x < 3; x++) { r = U(r, box(p, V(-.3 + float(x) * .3,.92 + float(y) * .2,.44), V(.1,.06,.012), .008, TONE)); }
  r = U(r, box(p, V(0,.16,.44), V(.14,.16,.012), .01, DARK));
  r = U(r, box(p, V(0,1.54,0), V(.3,.03,.26), .01, MID));
  return r;
}
