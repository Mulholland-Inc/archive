#pragma frame {"set":["business"],"label":"Auto garage","group":"auto"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.42,0), V(.7,.42,.44), .01, PALE));
  r = U(r, box(p, V(0,.28,.44), V(.56,.28,.012), .01, MID));
  for (int i = 0; i < 3; i++) { r = U(r, box(p, V(0,.1 + float(i) * .15,.45), V(.56,.01,.012), .006, DARK)); }
  r = U(r, box(mirX(p), V(0.38,0.24,.44), V(.16,.13,.012), .01, MID));
  r = U(r, box(p, V(0,.88,0), V(.74,.04,.48), .01, TONE));
  vec2 q = rotZ(p - V(0,1.38,0), -.75).xy;
  vec2 h = q - vec2(0,.42);
  r = U(r, emblem(p, min(sdRect(q - vec2(0,-.14), vec2(.1,.44), .05), max(sdCircle(h, .26), -sdRect(h - vec2(0,.16), vec2(.1,.2), .01))), PALE));
  return r;
}
