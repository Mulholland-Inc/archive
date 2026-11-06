#pragma frame {"set":["business"],"label":"Fishmonger","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .58, .42));
  r = U(r, awning(p, .46, .68, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec2 q = p.xy - vec2(0,1.12);
  float tail = max(max(dot(vec2(abs(q.y), q.x - .84), normalize(vec2(.8,.6))) - .0, .56 - q.x), q.x - .84);
  r = U(r, emblem(p, min(length(q * vec2(.72,1.3)) - .37, tail), PALE));
  r = U(r, ball(p, V(-.3,1.18,.1), .05, DARK));
  return r;
}
