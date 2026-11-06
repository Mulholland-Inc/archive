#pragma frame {"set":["business"],"label":"Yoga studio","group":"care"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.32,0), V(.68,.32,.42), .08, PALE));
  r = U(r, box(p, V(0,.68,0), V(.74,.035,.48), .03, TONE));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  vec3 c = p - V(0,.76,0);
  r = U(r, M(sdEllipsoid(c - V(0,.44,0), V(.14,.44,.07)), PALE));
  r = U(r, M(sdEllipsoid(rotZ(mirX(c) - V(.22,.36,0), .5), V(.13,.38,.06)), PALE));
  r = U(r, M(sdEllipsoid(rotZ(mirX(c) - V(.46,.22,0), 1.05), V(.11,.34,.05)), PALE));
  return r;
}
