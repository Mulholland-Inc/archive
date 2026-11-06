#pragma frame {"set":["business"],"label":"Car wash","group":"auto"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(max(max(max(length(p.xy) - .86, .58 - length(p.xy)), abs(p.z) - .44), -p.y), PALE));
  vec2 q = p.xy - vec2(0,.24);
  r = U(r, emblem(p - V(0,0,.1), min(sdRect(q, vec2(.42,.12), .08), max(sdRect(q - vec2(-.02,.14), vec2(.24,.12), .1), -q.y)), MID));
  r = U(r, canZ(p - V(0,0,.1), V(-.24,.1,.08), .08, .03, DARK));
  r = U(r, canZ(p - V(0,0,.1), V(.24,.1,.08), .08, .03, DARK));
  r = U(r, ball(p, V(-.3,1.08,0), .17, PALE));
  r = U(r, ball(p, V(.08,1.14,0), .22, MID));
  r = U(r, ball(p, V(.44,1.04,0), .14, PALE));
  r = U(r, ball(p, V(-.08,1.42,0), .11, MID));
  r = U(r, ball(p, V(.28,1.42,0), .09, PALE));
  return r;
}
