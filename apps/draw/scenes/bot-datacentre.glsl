#pragma frame {"set":["business"],"label":"Data centre","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.42,0), V(.84,.42,.56), .01, PALE));
  r = U(r, box(p, V(0,.88,0), V(.88,.04,.6), .01, TONE));
  for (int i = 0; i < 4; i++) { r = U(r, box(p, V(-.54 + float(i) * .36,.42,.56), V(.13,.34,.012), .008, i == 1 ? PALE : MID)); r = U(r, ball(p, V(-.54 + float(i) * .36,.68,.57), .03, DARK)); }
  r = U(r, can(p, V(-.42,1.02,0), .26, .1, MID));
  r = U(r, can(p, V(.42,1.02,0), .26, .1, MID));
  r = U(r, can(p, V(-.42,1.13,0), .17, .012, DARK));
  r = U(r, can(p, V(.42,1.13,0), .17, .012, DARK));
  return r;
}
