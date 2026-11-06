#pragma frame {"size":[720,1080],"center":[0.0,0.75,0.0],"zoom":250,"bounds":[[-3,-0.5,-3],[3,4,3]],"t":1}

vec2 map(vec3 p) {
  vec3 g = p - vec3(0.0, -0.1, 0.0);
  vec2 r = M(sdRoundBox(g, vec3(1.25, 0.1, 1.25), 0.02), WHITE);
  if (g.y > 0.07 && abs(p.x) < 1.2 && abs(p.z) < 1.2) {
    vec2 c = vec2(floor((p.x + 1.2) / 0.4), floor((p.z + 1.2) / 0.4));
    float w = mod(T * 5.0 - (c.x + c.y) * 0.6, 4.0);
    r = paint(r, -1.0, w < 1.0 ? LIME : w < 1.6 ? LILAC : WHITE);
    r = stripes(r, p.x + 1.2, 0.4);
    r = stripes(r, p.z + 1.2, 0.4);
  }
  float step_ = abs(sin(T * 7.0));
  vec3 b = p - vec3(0.5 * sin(T * 2.2), 0.04 * step_, 0.5 * cos(T * 2.2));
  r = U(r, fBot(rotY(b, -2.2 * T - 1.6)));
  return r;
}
