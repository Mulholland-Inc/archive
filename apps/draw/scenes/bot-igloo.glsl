#pragma frame {"set":["personal"],"label":"Igloo","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(max(sdSphere(p - V(0,.0,-.04), .84), -p.y), PALE));
  r = U(r, M(max(max(sdSphere(p - V(0,0,-.04), .845), -p.y), abs(mod(p.y, .28) - .14) - .008), MID));
  r = U(r, M(max(sdRoundBox(p - V(0,.1,.82), V(.3,.26,.2), .2), -p.y), PALE));
  r = U(r, M(max(sdRoundBox(p - V(0,.08,1.0), V(.16,.17,.03), .12), -p.y), DARK));
  return r;
}
