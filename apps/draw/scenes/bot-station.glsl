#pragma frame {"set":["business"],"label":"Train station","group":"transport"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.34,0), V(.86,.34,.36), .01, PALE));
  r = U(r, M(sdPrism(p - V(0,.68,0), .96, .24, .42), TONE));
  r = U(r, door(p, 0., .12, .3, .36));
  r = U(r, box(mirX(p), V(.44,.2,.36), V(.2,.11,.012), .01, MID));
  vec2 q = p.xy - vec2(0,1.3);
  r = U(r, emblem(p, min(sdRect(q - vec2(-.1,0.), vec2(.44,.18), .14), sdRect(q - vec2(.42,.1), vec2(.2,.3), .03)), PALE));
  r = U(r, emblem(p, sdRect(q - vec2(-.38,.26), vec2(.07,.14), .02), TONE));
  r = U(r, emblem(p, sdRect(q - vec2(.42,.4), vec2(.26,.05), .02), TONE));
  r = U(r, canZ(p, V(-.34,1.1,.08), .1, .03, DARK));
  r = U(r, canZ(p, V(.0,1.1,.08), .1, .03, DARK));
  r = U(r, canZ(p, V(.38,1.08,.08), .14, .03, DARK));
  r = U(r, box(p, V(.42,1.44,.1), V(.1,.08,.012), .01, MID));
  return r;
}
