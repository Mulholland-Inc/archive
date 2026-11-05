#pragma frame {"set":["business"],"label":"Medical clinic","group":"health"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .72, .56, .44));
  r = U(r, box(mirX(p), V(0.38,0.24,.44), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .13, .32, .44));
  vec2 q = p.xy - vec2(0,1.16);
  r = U(r, emblem(p, min(sdRect(q, vec2(.46,.16), .03), sdRect(q, vec2(.16,.46), .03)), PALE));
  return r;
}
