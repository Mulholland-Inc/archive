#pragma frame {"size":[380,320],"center":[0.0,0.55,0.0],"zoom":150,"bounds":[[-1.3,-0.1,-1.3],[1.3,1.6,1.3]]}

float flap(vec3 q, float w, float l, float a) {
  q = rotX(q, a);
  return sdBox(q - vec3(0.0, 0.0, -l * 0.5), vec3(w, 0.016, l * 0.5));
}

vec2 map(vec3 p) {
  vec3 b = p - vec3(0.0, 0.34, 0.0);
  vec2 r = Sc(M(sdBox(b, vec3(0.5, 0.34, 0.4)), WHITE), M(sdBox(b - vec3(0.0, 0.1, 0.0), vec3(0.47, 0.34, 0.37)), PAPER));
  float y = p.y - 0.68;
  float f = flap(vec3(p.x, y, p.z - 0.4), 0.5, 0.36, 0.3);
  f = min(f, flap(vec3(p.x, y, -p.z - 0.4), 0.5, 0.36, 1.35));
  f = min(f, flap(vec3(p.z, y, p.x - 0.5), 0.4, 0.3, 1.9));
  f = min(f, flap(vec3(p.z, y, -p.x - 0.5), 0.4, 0.3, 0.2));
  return U(r, M(f, PAPER));
}
