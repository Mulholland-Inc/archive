#pragma frame {"set":["business"],"label":"Library","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.5,0), V(.72,.5,.4), .01, PALE));
  for (int i = 0; i < 7; i++) { r = U(r, box(p, V(-.54 + float(i) * .18,.3,.4), V(.055,.22,.015), .01, mod(float(i),3.) == 0. ? TONE : mod(float(i),3.) == 1. ? MID : DARK)); }
  r = U(r, M(sdPrism(p - V(0,1.0,0), .78, .26, .44), TONE));
  return r;
}
