#pragma frame {"set":["business"],"label":"Art gallery","group":"culture"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.32,0), V(.7,.32,.42), .01, PALE));
  r = U(r, box(p, V(0,.68,0), V(.74,.04,.46), .01, TONE));
  r = U(r, box(mirX(p), V(0.38,0.24,.42), V(.16,.13,.012), .01, MID));
  r = U(r, door(p, 0., .1, .3, .42));
  r = U(r, box(p, V(0,1.22,0), V(.62,.5,.06), .02, TONE));
  r = U(r, box(p, V(0,1.22,.06), V(.5,.38,.012), .01, PALE));
  vec2 q = p.xy - vec2(0,1.22);
  r = U(r, M(extrude(max(max(dot(vec2(abs(q.x + .1), q.y + .34), normalize(vec2(.6,.34))) - .22, -q.y - .36), abs(q.x) - .48), p.z - .07, .012), DARK));
  r = U(r, canZ(p, V(.24,1.42,.06), .08, .02, MID));
  return r;
}
