#pragma frame {"set":["personal"],"label":"Houseboat","group":"home"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, M(sdRoundBox(p - V(0,.18,0), V(.46,.18,.9), .12), TONE));
  r = U(r, box(p, V(0,.6,-.08), V(.36,.26,.56), .01, PALE));
  r = U(r, pitch(p, .86, .46, .26, .64, TONE));
  r = U(r, stick(p, V(0,.36,.84), V(0,.74,.84), .018, DARK));
  r = U(r, box(p, V(0,.68,.84), V(.1,.05,.01), .01, TONE));
  return r;
}
