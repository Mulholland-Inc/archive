#pragma frame {"set":["business"],"label":"Butcher","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .68, .58, .42));
  r = U(r, awning(p, .46, .68, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec2 q = rotZ(p - V(0,1.18,0), -.5).xy;
  r = U(r, emblem(p, max(sdRect(q - vec2(-.08,.22), vec2(.44,.28), .04), -sdCircle(q - vec2(-.38,.3), .07)), PALE));
  r = U(r, emblem(p, sdRect(q - vec2(.0,-.26), vec2(.07,.22), .04), TONE));
  return r;
}
