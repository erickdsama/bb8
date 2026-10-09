// Parte 1 · prueba 5: sensor de distancia VL53L0X (librería "VL53L0X" de Pololu).
// Monitor serial a 115200. Acerca la mano: por debajo de 250 mm el firmware frenaría.
// Alcance útil ~1.2 m en interiores; "fuera de rango" es normal mirando al vacío.
#include <Wire.h>
#include <VL53L0X.h>

VL53L0X tof;

void setup() {
  Serial.begin(115200);
  Wire.begin(); Wire.setWireTimeout(3000, true);
  tof.setTimeout(500);
  if (!tof.init()) { Serial.println(F("No responde el VL53L0X en 0x29. Corre p1_i2c.")); while (1) {} }
  tof.setMeasurementTimingBudget(33000);
  tof.startContinuous();
}

void loop() {
  uint16_t mm = tof.readRangeContinuousMillimeters();
  if (tof.timeoutOccurred()) Serial.println(F("timeout"));
  else if (mm >= 2000) Serial.println(F("fuera de rango"));
  else { Serial.print(mm); Serial.println(mm < 250 ? F(" mm  ← FRENARÍA") : F(" mm")); }
  delay(100);
}
