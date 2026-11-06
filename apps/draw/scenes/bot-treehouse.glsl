#pragma frame {"set":["personal","family"],"label":"Treehouse","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, stick(p, V(0,0,0), V(0,.7,0), .18, TONE));
  r = U(r, stick(p, V(.08,.46,0), V(.5,.76,0), .05, TONE));
  r = U(r, stick(p, V(-.08,.46,0), V(-.5,.76,0), .05, TONE));
  r = U(r, M(smin(smin(sdEllipsoid(p - V(0,1.5,-.2), V(.84,.48,.46)), sdSphere(m - V(.62,1.2,-.2), .36), .12), sdSphere(p - V(0,1.84,-.2), .34), .1), MID));
  r = U(r, box(p, V(0,1.02,.1), V(.4,.24,.32), .01, PALE));
  r = U(r, pitch(p, 1.26, .5, .28, .4, TONE));
  r = U(r, box(p, V(0,.77,.14), V(.6,.025,.42), .01, TONE));
  return r;
}
