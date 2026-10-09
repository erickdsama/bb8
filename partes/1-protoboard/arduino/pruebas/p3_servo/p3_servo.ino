// Parte 1 · prueba 3: servo MG996R del poste en D9.
// Aliméntalo del buck a 5 V (nunca del pin 5V del Arduino) con GND común.
// Barre −90…+90 (servo 0…180°) y luego acepta un ángulo por el monitor serial
// (115200, "Nueva línea"), por ejemplo: 30  o  -45
#include <Servo.h>

Servo servo;

void ir(int g) {
  g = constrain(g, -90, 90);
  servo.writeMicroseconds(1500 + g * 1000L / 90);   // igual que el firmware
  Serial.print(F("cabeza ")); Serial.print(g); Serial.println(F("°"));
}

void setup() {
  Serial.begin(115200);
  servo.attach(9, 500, 2500);
  for (int g = -90; g <= 90; g += 15) { ir(g); delay(400); }
  for (int g = 90; g >= -90; g -= 15) { ir(g); delay(400); }
  ir(0);
  Serial.println(F("Escribe un ángulo entre -90 y 90. Si los extremos zumban, el servo no llega: usa ±80."));
}

void loop() {
  if (Serial.available()) {
    int g = Serial.parseInt();
    while (Serial.available()) Serial.read();
    ir(g);
  }
}
