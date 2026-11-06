#pragma frame {"set":["business"],"label":"Jeweller","group":"shop"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, block(p, .66, .56, .42));
  r = U(r, box(p, V(0,.32,.42), V(.2,.05,.012), .01, MID));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec2 q = p.xy - vec2(0,1.28);
  r = U(r, emblem(p, max(max(dot(vec2(abs(q.x), q.y), normalize(vec2(.62,.42))) - .28, -(q.y + .52) * 1. + .0), max(dot(vec2(abs(q.x), -q.y), normalize(vec2(.84,.56))) - .3, q.y - .3)), PALE));
  r = U(r, stick(p, V(-.5,1.46,.08), V(.5,1.46,.08), .012, DARK));
  r = U(r, stick(p, V(-.2,1.46,.08), V(0,.9,.08), .012, DARK));
  r = U(r, stick(p, V(.2,1.46,.08), V(0,.9,.08), .012, DARK));
  return r;
}
