#pragma frame {"set":["business"],"label":"Server building","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.84,0), V(.46,.84,.36), .02, PALE));
  for (int i = 0; i < 4; i++) { r = U(r, box(p, V(0,.32 + float(i) * .3,.36), V(.38,.11,.012), .01, i == 2 ? PALE : MID)); r = U(r, ball(p, V(.28,.32 + float(i) * .3,.37), .025, DARK)); }
  for (int i = 0; i < 3; i++) { r = U(r, box(p, V(-.1,.32 + float(i) * .0,.37), V(.14,.012,.012), .008, DARK)); }
  return r;
}
