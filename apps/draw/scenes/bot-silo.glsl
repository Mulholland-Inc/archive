#pragma frame {"set":["business"],"label":"Grain silo","group":"trade"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, can(p, V(-.1,.84,0), .42, .84, PALE));
  r = U(r, M(max(sdSphere(p - V(-.1,1.68,0), .42), 1.68 - p.y), TONE));
  r = U(r, can(p, V(.52,.42,0), .24, .42, MID));
  r = U(r, M(max(sdSphere(p - V(.52,.84,0), .24), .84 - p.y), TONE));
  r = U(r, stick(p, V(.1,1.52,0), V(.42,.96,0), .03, TONE));
  return r;
}
