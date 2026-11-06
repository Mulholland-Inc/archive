#pragma frame {"set":["personal"],"label":"Apartment block","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.92,0), V(.54,.92,.4), .01, PALE));
  r = U(r, box(p, V(0,1.88,0), V(.58,.04,.44), .01, TONE));
  for (int y = 0; y < 3; y++) { r = U(r, box(p, V(-.28,.84 + float(y) * .34,.48), V(.17,.04,.1), .01, TONE)); r = U(r, box(p, V(.28,.84 + float(y) * .34,.48), V(.17,.04,.1), .01, TONE)); r = U(r, box(p, V(-.28,.96 + float(y) * .34,.4), V(.11,.1,.012), .01, MID)); r = U(r, box(p, V(.28,.96 + float(y) * .34,.4), V(.11,.1,.012), .01, MID)); }
  r = U(r, door(p, 0., .11, .32, .4));
  return r;
}
