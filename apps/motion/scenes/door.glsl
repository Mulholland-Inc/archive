#pragma frame {"size":[1000,1080],"center":[0.0,1.25,1.2],"zoom":175,"yaw":40,"pitch":24,"bounds":[[-5,-0.5,-2],[5,6,6]],"t":2}

vec2 map(vec3 p) {
  float w = mix(0.95, 0.24, glide(0.15, 1.9));
  float g = mix(0.55, 0.95, glide(0.2, 1.8));
  vec2 r = M(sdRoundBox(p - vec3(0.0, -0.1, 1.5), vec3(4.0, 0.1, 2.9), 0.02), WHITE);
  r = seam(r, p.y + 0.05);
  vec2 wl = M(sdBox(p - vec3(0.0, 1.8, -0.2), vec3(4.0, 1.8, 0.2)), WHITE);
  float hole = sdRect(vec2(p.x, p.y - 1.3), vec2(w, 1.3), 0.0);
  wl.x = max(wl.x, -max(hole, abs(p.z + 0.2) - 0.3));
  r = U(r, wl);
  r = U(r, M(sdBox(p - vec3(0.0, 1.8, -0.5), vec3(4.0, 1.8, 0.1)), GRAPHITE));
  float bump = abs(sin(T * 5.0)) * 0.34;
  vec3 d = p - vec3(0.0, 0.0, 1.05 + g + bump);
  r = U(r, fDrum(d / g) * vec2(g, 1.0));
  vec3 b = p - vec3(0.2, 0.0, 1.5 + 2.0 * g + bump);
  r = U(r, fBot(rotY(b, PI)));
  return r;
}
