// Parte 1 · prueba 4: MPU6050 sin librerías (los mismos registros que el firmware).
// Monitor serial a 115200, o el Serial Plotter para verlo en gráfica.
// Inclina la protoboard: pitch y roll cambian; gírala sobre la mesa: el rumbo
// debe SUBIR al girar a la derecha (si baja, SIGNO_GIRO_Z = 1 en el firmware).
#include <Wire.h>

const uint8_t MPU = 0x68;
float pitch = 0, roll = 0, rumbo = 0, sesgo = 0;
const int SIGNO_GIRO_Z = -1;

void escribir(uint8_t r, uint8_t v) { Wire.beginTransmission(MPU); Wire.write(r); Wire.write(v); Wire.endTransmission(); }

bool leer(int16_t *v) {
  Wire.beginTransmission(MPU); Wire.write(0x3B);
  if (Wire.endTransmission(false) != 0 || Wire.requestFrom(MPU, (uint8_t)14) != 14) return false;
  for (uint8_t i = 0; i < 7; i++) v[i] = (Wire.read() << 8) | Wire.read();
  return true;
}

void setup() {
  Serial.begin(115200);
  Wire.begin(); Wire.setClock(400000); Wire.setWireTimeout(3000, true);
  escribir(0x6B, 0x01); escribir(0x1A, 0x03); escribir(0x1B, 0x08); escribir(0x1C, 0x08);
  delay(50);
  int16_t v[7]; long s = 0; int n = 0;
  Serial.println(F("Calibrando giroscopio: no lo muevas 1 s..."));
  for (int i = 0; i < 200; i++) { if (leer(v)) { s += v[6]; n++; } delay(5); }
  if (n == 0) { Serial.println(F("No responde el MPU6050 en 0x68. Corre p1_i2c.")); while (1) {} }
  sesgo = (float)s / n;
}

void loop() {
  static unsigned long previo = micros(), imp = 0;
  unsigned long t = micros();
  if (t - previo < 10000) return;
  float dt = (t - previo) * 1e-6; previo = t;
  int16_t v[7];
  if (!leer(v)) { Serial.println(F("lectura fallida")); return; }
  float ax = v[0], ay = v[1], az = v[2];
  pitch = 0.98 * (pitch + v[5] / 65.5 * dt) + 0.02 * atan2(-ax, sqrt(ay * ay + az * az)) * RAD_TO_DEG;
  roll = 0.98 * (roll + v[4] / 65.5 * dt) + 0.02 * atan2(ay, az) * RAD_TO_DEG;
  float gz = (v[6] - sesgo) / 65.5;
  if (fabs(gz) > 0.3) rumbo += SIGNO_GIRO_Z * gz * dt;
  if (millis() - imp >= 100) {
    imp = millis();
    Serial.print(F("pitch:")); Serial.print(pitch, 1);
    Serial.print(F(" roll:")); Serial.print(roll, 1);
    Serial.print(F(" inclinacion:")); Serial.print(max(fabs(pitch), fabs(roll)), 1);
    Serial.print(F(" rumbo:")); Serial.println(rumbo, 1);
  }
}
