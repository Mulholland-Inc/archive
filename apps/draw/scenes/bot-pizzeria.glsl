#pragma frame {"set":["business"],"label":"Pizzeria","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .56, .42));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(mirX(p), V(.4,.24,.42), V(.16,.13,.012), .01, MID));
  vec2 q = p.xy - vec2(0,1.22);
  r = U(r, emblem(p, max(dot(vec2(abs(q.x), q.y + .6), normalize(vec2(1.04,-.56))), q.y - .42), PALE));
  r = U(r, emblem(p, sdRect(q - vec2(0,.5), vec2(.62,.1), .08), MID));
  r = U(r, canZ(p, V(-.14,1.34,.11), .1, .02, DARK));
  r = U(r, canZ(p, V(.16,1.42,.11), .09, .02, DARK));
  r = U(r, canZ(p, V(.03,1.02,.11), .08, .02, DARK));
  return r;
}
