#pragma frame {"set":["business"],"label":"Dental office","group":"health"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .62, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec2 q = p.xy - vec2(0,1.3);
  r = U(r, emblem(p, smin(sdRect(q - vec2(0,.14), vec2(.4,.28), .2), min(sdSeg(q, vec2(-.21,-.05), vec2(-.17,-.46)) - .13, sdSeg(q, vec2(.21,-.05), vec2(.17,-.46)) - .13), .06), PALE));
  return r;
}
