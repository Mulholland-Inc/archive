#pragma frame {"set":["business"],"label":"Car dealership","group":"auto"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.42,0), V(.76,.42,.44), .01, PALE));
  r = U(r, box(p, V(0,.42,.44), V(.68,.34,.012), .01, MID));
  r = U(r, box(p, V(0,.88,0), V(.8,.04,.48), .01, TONE));
  r = U(r, box(mirX(p), V(0.38,0.24,.46), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .09, .26, .46));
  vec2 q = p.xy - vec2(0,1.2);
  r = U(r, emblem(p, min(sdRect(q, vec2(.7,.16), .1), max(sdRect(q - vec2(-.04,.2), vec2(.4,.2), .14), -q.y)), PALE));
  r = U(r, canZ(p, V(-.42,1.05,.08), .15, .05, DARK));
  r = U(r, canZ(p, V(.42,1.05,.08), .15, .05, DARK));
  r = U(r, box(p, V(-.14,1.36,.1), V(.14,.09,.012), .03, MID));
  r = U(r, box(p, V(.18,1.36,.1), V(.12,.09,.012), .03, MID));
  return r;
}
