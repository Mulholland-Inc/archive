#pragma frame {"set":["business","family"],"label":"Church","group":"civic"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  r = U(r, box(p, V(0,.42,-.1), V(.46,.42,.56), .01, PALE));
  r = U(r, pitch(p, .84, .56, .36, .62, TONE));
  r = U(r, box(p, V(0,.84,.34), V(.2,.26,.2), .01, PALE));
  r = U(r, M(sdPrism(p - V(0,1.1,.34), .26, .42, .22), TONE));
  vec2 q = p.xy - vec2(0,1.84);
  r = U(r, emblem(p - V(0,0,.34), min(sdRect(q, vec2(.04,.34), .01), sdRect(q - vec2(0,.08), vec2(.2,.04), .01)), PALE));
  r = U(r, box(p, V(0,.2,.47), V(.1,.2,.02), .01, DARK));
  return r;
}
