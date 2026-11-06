#pragma frame {"set":["business"],"label":"Recruitment agency","group":"office"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(p, V(0,1.22,0), V(.56,.56,.05), .03, MID));
  vec2 q = p.xy - vec2(0,1.0);
  r = U(r, emblem(p - V(0,0,.05), min(sdCircle(q - vec2(0,.44), .19), max(length((q - vec2(0,.02)) * vec2(.8,1.)) - .32, -q.y - .16)), PALE));
  return r;
}
