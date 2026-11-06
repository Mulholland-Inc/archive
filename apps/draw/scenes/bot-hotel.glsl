#pragma frame {"set":["business"],"label":"Hotel","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.66,0), V(.56,.66,.38), .01, PALE));
  r = U(r, box(p, V(0,1.34,0), V(.6,.04,.42), .01, TONE));
  for (int y = 0; y < 2; y++) for (int x = 0; x < 4; x++) { r = U(r, box(p, V(-.39 + float(x) * .26,.82 + float(y) * .24,.38), V(.08,.075,.012), .01, MID)); }
  r = U(r, box(mirX(p), V(0.38,0.24,.38), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .26, .38));
  vec2 q = p.xy - vec2(0,1.4);
  r = U(r, emblem(p, min(min(sdRect(q - vec2(0,.16), vec2(.6,.11), .05), sdRect(q - vec2(-.56,.3), vec2(.05,.3), .03)), sdRect(q - vec2(.56,.14), vec2(.05,.16), .03)), PALE));
  r = U(r, oval(p, V(-.32,1.72,0), V(.16,.08,.08), MID));
  return r;
}
