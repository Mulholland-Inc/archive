#pragma frame {"set":["business"],"label":"Theatre","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .72, .6, .42));
  r = U(r, door(p, 0., .13, .3, .42));
  r = U(r, box(mirX(p), V(.46,.24,.42), V(.14,.13,.012), .01, MID));
  vec2 q = p.xy - vec2(0,1.2);
  r = U(r, emblem(p, sdRect(q, vec2(.68,.5), .03), DARK));
  r = U(r, emblem(p - V(0,0,.03), max(sdRect(q, vec2(.68,.5), .03), -(length(q - vec2(0,-.6)) - .56)), PALE));
  r = U(r, emblem(p - V(0,0,.06), sdRect(q - vec2(0,.42), vec2(.72,.13), .05), TONE));
  r = U(r, stick(p, V(0,1.2,.14), V(0,1.5,.14), .012, DARK));
  return r;
}
