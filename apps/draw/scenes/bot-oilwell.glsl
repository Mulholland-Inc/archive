#pragma frame {"set":["business"],"label":"Oil well","group":"energy"}
vec2 map(vec3 p) {
  p = rotY(p, TURN);
  vec3 m = mirX(p);
  vec2 r = M(1e3, PALE);
  vec3 c = mirXZ(p);
  r = U(r, stick(c, V(.54,0,.54), V(.08,1.6,.08), .04, TONE));
  for (int i = 0; i < 4; i++) { float y = .7 + float(i) * .24, w = .54 * (1. - y / 1.74); r = U(r, box(p, V(0,y,w), V(w,.018,.018), .008, TONE)); r = U(r, box(p, V(w,y,0), V(.018,.018,w), .008, TONE)); }
  r = U(r, box(p, V(0,1.66,0), V(.12,.07,.12), .01, DARK));
  r = U(r, box(p, V(0,.34,0), V(.34,.34,.34), .01, PALE));
  return r;
}
