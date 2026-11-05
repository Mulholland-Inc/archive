#pragma frame {"set":["business"],"label":"Grocery","group":"food"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .76, .62, .44));
  r = U(r, awning(p, .5, .76, .44));
  r = U(r, box(mirX(p), V(0.38,0.24,.44), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .12, .34, .44));
  r = U(r, M(smin(sdSphere(p - V(-.2,1.16,0), .34), sdSphere(p - V(.2,1.16,0), .34), .08), PALE));
  r = U(r, stick(p, V(0,1.42,0), V(.06,1.62,0), .04, TONE));
  r = U(r, M(sdEllipsoid(rotZ(p - V(.26,1.6,0), 1.1), V(.08,.2,.04)), MID));
  return r;
}
