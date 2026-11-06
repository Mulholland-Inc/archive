#pragma frame {"set":["business"],"label":"Nonprofit office","group":"civic"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec2 q = p.xy - vec2(0,1.24);
  vec2 h = vec2(abs(q.x), q.y);
  r = U(r, emblem(p, min(sdCircle(h - vec2(.26,.14), .28), max(dot(h - vec2(0.,-.54), normalize(vec2(.8,-.52))), max(-h.x, h.y - .14))), PALE));
  return r;
}
