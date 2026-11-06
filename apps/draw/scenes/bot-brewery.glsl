#pragma frame {"set":["business"],"label":"Brewery","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .6, .62, .4));
  r = U(r, box(mirX(p), V(0.38,0.24,.4), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .32, .4));
  r = U(r, can(p, V(-.84,.5,0), .22, .5, MID));
  r = U(r, M(max(sdSphere(p - V(-.84,1.0,0), .22), 1. - p.y), MID));
  r = U(r, can(p, V(-.08,1.14,0), .36, .42, PALE));
  r = U(r, ringZ(p, V(.36,1.14,0), .17, .05, PALE));
  r = U(r, oval(p, V(-.2,1.58,0), V(.2,.12,.2), MID));
  r = U(r, oval(p, V(.08,1.6,0), V(.22,.14,.2), MID));
  for (int i = 0; i < 2; i++) { r = U(r, box(p, V(-.2 + float(i) * .24,1.12,.35), V(.025,.28,.02), .015, TONE)); }
  return r;
}
