#pragma frame {"set":["business"],"label":"Vet","group":"health"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(sdPrism(p - V(0,.62,0), .78, .3, .5), TONE));
  r = U(r, box(p, V(0,.31,0), V(.62,.31,.42), .01, PALE));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec2 q = p.xy - vec2(0,1.42);
  float toes = min(min(sdCircle(vec2(abs(q.x), q.y) - vec2(.36,.04), .13), sdCircle(vec2(abs(q.x), q.y) - vec2(.14,.28), .13)), 1e3);
  r = U(r, emblem(p, min(toes, length((q - vec2(0,-.22)) * vec2(.85,1.1)) - .27), PALE));
  return r;
}
