#pragma frame {"set":["business"],"label":"Bakery","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .7, .6, .42));
  r = U(r, awning(p, .48, .7, .42));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .32, .42));
  r = U(r, M(max(sdEllipsoid(p - V(0,.9,0), V(.74,.58,.34)), .72 - p.y), PALE));
  for (int i = 0; i < 3; i++) { r = U(r, stick(p, V(-.4 + float(i) * .3,1.08,.26), V(-.26 + float(i) * .3,1.32,.2), .03, TONE)); }
  return r;
}
