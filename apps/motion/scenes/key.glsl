#pragma frame {"size":[1920,1080],"center":[0.0,0.2,0.0],"zoom":420,"bounds":[[-2,-1,-2],[2,5,2]],"t":3}

vec2 map(vec3 p) {
  float x = clamp(T / 0.95, 0.0, 1.0);
  float e = (1.0 - x) * (1.0 - x);
  float y = 3.2 * abs(cos(x * 7.85)) * e;
  float spin = 0.5 * T + 5.0 * e;
  vec3 q = rotY(p - vec3(0.0, y, 0.0), spin - 0.6);
  q = rotX(q, 0.5 * e * sin(x * 24.0));
  return fKey(q, WHITE);
}
