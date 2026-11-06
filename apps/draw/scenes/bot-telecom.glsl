#pragma frame {"set":["business"],"label":"Telecom mast","group":"energy"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.2,0), V(.4,.2,.34), .01, PALE));
  r = U(r, box(p, V(0,.44,0), V(.44,.03,.38), .01, TONE));
  r = U(r, door(p, 0., .08, .26, .34));
  vec3 c = mirXZ(p);
  r = U(r, stick(c, V(.24,.44,.2), V(.04,1.82,.04), .035, TONE));
  for (int i = 0; i < 4; i++) { float y = .68 + float(i) * .27, w = .24 * (1. - (y - .44) / 1.5); r = U(r, box(p, V(0,y,w), V(w,.02,.02), .008, TONE)); r = U(r, box(p, V(w,y,0), V(.02,.02,w), .008, TONE)); }
  r = U(r, canZ(p, V(-.34,1.26,0), .24, .05, PALE));
  r = U(r, canZ(p, V(.3,1.0,0), .2, .05, PALE));
  r = U(r, ball(p, V(0,1.86,0), .07, PALE));
  vec2 q = p.xy - vec2(0,1.86);
  r = U(r, emblem(p, max(max(abs(length(q) - .3) - .04, -abs(q.x) * .6 + q.y * -1. + .0), -q.y), PALE));
  r = U(r, emblem(p, max(abs(length(q) - .5) - .045, -q.y), PALE));
  return r;
}
