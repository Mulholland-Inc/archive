#pragma frame {"set":["business"],"label":"Market","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.28,.0), V(.66,.28,.36), .02, PALE));
  r = U(r, stick(mirZ(m), V(.62,.5,.3), V(.62,1.1,.3), .03, TONE));
  r = U(r, M(max(max(sdBox(p - V(0,1.18,0), V(.74,.14,.48)), dot(p.yz - vec2(1.34,-.4), normalize(vec2(1.,.2))) ), 0.), PALE));
  r = U(r, M(max(max(sdBox(p - V(0,1.18,0), V(.75,.14,.485)), dot(p.yz - vec2(1.34,-.4), normalize(vec2(1.,.2)))), abs(mod(p.x + 1.05, .42) - .21) - .1), TONE));
  r = U(r, ball(p, V(-.3,.62,.2), .12, MID));
  r = U(r, ball(p, V(-.02,.62,.2), .12, MID));
  r = U(r, ball(p, V(.28,.6,.2), .1, TONE));
  return r;
}
