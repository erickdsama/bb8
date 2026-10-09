// Parte 1 · prueba 2: motores con el L298N y sus encoders.
// Monitor serial a 115200, "Nueva línea". Ruedas en el aire la primera vez.
//
//   a / z   motor izquierdo adelante / atrás        k / m   motor derecho adelante / atrás
//   b       los dos adelante                        espacio freno
//   + / -   sube / baja el PWM (empieza en 128 = 50 %)
//   r       pone los contadores en 0 (para medir pulsos por vuelta: gira la rueda
//           10 vueltas a mano y divide E entre 10 → TICKS_PER_REV en bb8/config.py)
//
// Debe pasar: "adelante" hace que el robot avance y su encoder cuente hacia ARRIBA.
// Si no, apunta qué invertir en el firmware (INVERTIR_MOTOR_* o INVERTIR_ENC_*).

const uint8_t ENA = 5, IN1 = 7, IN2 = 8, ENB = 6, IN3 = 11, IN4 = 12;
static const int8_t QDEC[16] = {0, -1, 1, 0, 1, 0, 0, -1, -1, 0, 0, 1, 0, 1, -1, 0};
volatile long encIzq = 0, encDer = 0;
volatile uint8_t prevIzq = 0, prevDer = 0;
int pwm = 128;

static inline uint8_t leerIzq() { return (((PIND >> 2) & 1) << 1) | ((PIND >> 4) & 1); }
static inline uint8_t leerDer() { return (((PIND >> 3) & 1) << 1) | ((PINB >> 2) & 1); }
void isrIzq() { uint8_t s = leerIzq(); encIzq += QDEC[(prevIzq << 2) | s]; prevIzq = s; }
void isrDer() { uint8_t s = leerDer(); encDer += QDEC[(prevDer << 2) | s]; prevDer = s; }
ISR(PCINT2_vect) { isrIzq(); }
ISR(PCINT0_vect) { isrDer(); }

void motor(uint8_t en, uint8_t a, uint8_t b, int v) {
  if (v > 0) { digitalWrite(a, HIGH); digitalWrite(b, LOW); analogWrite(en, v); }
  else if (v < 0) { digitalWrite(a, LOW); digitalWrite(b, HIGH); analogWrite(en, -v); }
  else { digitalWrite(a, HIGH); digitalWrite(b, HIGH); analogWrite(en, 255); }   // freno
}

void setup() {
  Serial.begin(115200);
  pinMode(ENA, OUTPUT); pinMode(IN1, OUTPUT); pinMode(IN2, OUTPUT);
  pinMode(ENB, OUTPUT); pinMode(IN3, OUTPUT); pinMode(IN4, OUTPUT);
  motor(ENA, IN1, IN2, 0); motor(ENB, IN3, IN4, 0);
  pinMode(2, INPUT_PULLUP); pinMode(4, INPUT_PULLUP); pinMode(3, INPUT_PULLUP); pinMode(10, INPUT_PULLUP);
  prevIzq = leerIzq(); prevDer = leerDer();
  attachInterrupt(digitalPinToInterrupt(2), isrIzq, CHANGE);
  attachInterrupt(digitalPinToInterrupt(3), isrDer, CHANGE);
  PCMSK2 |= _BV(PCINT20); PCMSK0 |= _BV(PCINT2); PCICR |= _BV(PCIE2) | _BV(PCIE0);
  Serial.println(F("a/z izq, k/m der, b ambos, espacio freno, +/- PWM, r contadores a 0"));
}

void loop() {
  if (Serial.available()) {
    char c = Serial.read();
    switch (c) {
      case 'a': motor(ENA, IN1, IN2, pwm); break;
      case 'z': motor(ENA, IN1, IN2, -pwm); break;
      case 'k': motor(ENB, IN3, IN4, pwm); break;
      case 'm': motor(ENB, IN3, IN4, -pwm); break;
      case 'b': motor(ENA, IN1, IN2, pwm); motor(ENB, IN3, IN4, pwm); break;
      case ' ': motor(ENA, IN1, IN2, 0); motor(ENB, IN3, IN4, 0); break;
      case '+': pwm = min(255, pwm + 16); break;
      case '-': pwm = max(0, pwm - 16); break;
      case 'r': noInterrupts(); encIzq = encDer = 0; interrupts(); break;
      default: return;
    }
    Serial.print(F("orden ")); Serial.print(c); Serial.print(F("  PWM ")); Serial.println(pwm);
  }
  static unsigned long t = 0;
  static long prevI = 0, prevD = 0;
  if (millis() - t >= 500) {
    t = millis();
    noInterrupts(); long i = encIzq, d = encDer; interrupts();
    Serial.print(F("E izq=")); Serial.print(i); Serial.print(F(" der=")); Serial.print(d);
    Serial.print(F("   pulsos/10 ms izq=")); Serial.print((i - prevI) / 50.0, 1);
    Serial.print(F(" der=")); Serial.println((d - prevD) / 50.0, 1);
    prevI = i; prevD = d;
  }
}
