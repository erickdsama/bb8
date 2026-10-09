// Parte 1 · prueba 1: ¿están vivos los sensores I2C?
// Monitor serial a 115200. Debe listar 0x68 (MPU6050) y 0x29 (VL53L0X).
// Si no aparece nada: SDA en A4, SCL en A5, VCC a 5 V (o 3.3 V según el módulo) y GND común.
#include <Wire.h>

void setup() {
  Serial.begin(115200);
  Wire.begin();
  Wire.setWireTimeout(3000, true);
}

void loop() {
  Serial.println(F("Buscando en el bus I2C..."));
  uint8_t n = 0;
  for (uint8_t dir = 1; dir < 127; dir++) {
    Wire.beginTransmission(dir);
    if (Wire.endTransmission() == 0) {
      Serial.print(F("  0x")); Serial.print(dir, HEX);
      if (dir == 0x68) Serial.print(F("  MPU6050"));
      else if (dir == 0x69) Serial.print(F("  MPU6050 con AD0 en alto (el firmware espera 0x68)"));
      else if (dir == 0x29) Serial.print(F("  VL53L0X"));
      Serial.println();
      n++;
    }
  }
  Serial.print(n); Serial.println(F(" dispositivo(s). Repito en 3 s.\n"));
  delay(3000);
}
