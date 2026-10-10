// BB-8 · firmware del Arduino Uno (partes 2, 3 y 6)
//
// Implementa PROTOCOLO.md, sección 1: M, S, H, D, ?, Z, W, I y P (apagado).
// Reflejos cada 10 ms que no esperan a la Pi: rampa, PID con encoders,
// freno por inclinación > 35°, por obstáculo < 250 mm y watchdog de 500 ms.
//
// Librerías: Wire y Servo (vienen con el IDE) y "VL53L0X" de Pololu
// (Gestor de librerías → buscar "VL53L0X Pololu") si USAR_TOF_LOCAL = 1.
//
// Pines (un L298N para los dos motores, jumpers ENA/ENB quitados):
//   D5 ENA, D7 IN1, D8 IN2 → motor izquierdo     D6 ENB, D11 IN3, D12 IN4 → derecho
//   D2/D4 encoder izq C1/C2   D3/D10 encoder der C1/C2   D9 servo del poste
//   A4/A5 I2C (MPU6050 0x68, VL53L0X 0x29)   A2 INT del MPU   A3 corte de la Pi   A0 batería

#include <Wire.h>
#include <Servo.h>
#include <avr/sleep.h>
#include <avr/interrupt.h>

// ============================ CONFIGURACIÓN ================================
#define FW_VERSION "0.1"

#define USAR_TOF_LOCAL   1   // VL53L0X en el I2C del Arduino (partes 1-4). 0 cuando pase a la cabeza.
#define USAR_PID         1   // 0 = lazo abierto (PWM directo), útil si un encoder falla
#define HAY_DIVISOR_BAT  0   // 1 cuando el divisor de la batería esté en A0 (parte 6)

// Si un motor gira al revés de lo esperado, cambia su 0 por 1 (o cruza sus cables).
#define INVERTIR_MOTOR_IZQ 0
#define INVERTIR_MOTOR_DER 0
// Si con "M 80 80" un encoder cuenta hacia abajo, cambia su 0 por 1.
#define INVERTIR_ENC_IZQ   0
#define INVERTIR_ENC_DER   0
// Rumbo: + derecha. El giroscopio Z del MPU6050 montado con el chip arriba da +
// a la izquierda, por eso -1. Si "turn 90" gira a la izquierda, ponlo en 1.
#define SIGNO_GIRO_Z      (-1)
#define SERVO_INVERTIDO    0

// Velocidad: pulsos de encoder en 10 ms a PWM 255. 0.9 m/s × 6464 pulsos/m ≈ 58.
// Mídelo con "python -m calibrar velocidad" y ajústalo.
const float PULSOS_MAX_CICLO = 58.0;
const float KP = 2.0;          // PWM por pulso de error
const float KI = 0.3;
const int   RAMPA = 30;        // máximo cambio de objetivo por ciclo de 10 ms

const float TILT_FRENO = 35.0, TILT_LIBERA = 25.0;
const int   OBSTACULO_MM = 250;
const unsigned long WATCHDOG_MS = 500;
const unsigned long D_VALIDO_MS = 300;    // distancia que llega de la Pi con D
const unsigned long TOF_LOCAL_VALIDO_MS = 150;
const float SERVO_GRADOS_S = 300.0;

// Batería 3S: divisor 10 kΩ (arriba) / 4.7 kΩ (abajo) → 12.6 V se leen como 4.03 V.
const float BAT_FACTOR = (10.0 + 4.7) / 4.7;
const float BAT_BAJA_V = 10.5;      // bandera "bateria": avisar y buscar el cargador
const float BAT_CRITICA_V = 9.9;    // 3.3 V/celda: rechaza M y frena

// Corte de la Pi (parte 6). Nivel en A3 que APAGA la Pi. Con un P-MOSFET de lado
// alto y la compuerta con pull-down de 100 kΩ, el Arduino en reset deja la Pi
// encendida: abrir el puerto serial reinicia el Arduino y no debe apagar la Pi.
#define NIVEL_CORTE_PI HIGH
// ===========================================================================

const uint8_t PIN_ENA = 5, PIN_IN1 = 7, PIN_IN2 = 8;
const uint8_t PIN_ENB = 6, PIN_IN3 = 11, PIN_IN4 = 12;
const uint8_t PIN_SERVO = 9;
const uint8_t PIN_MPU_INT = A2, PIN_CORTE_PI = A3, PIN_BAT = A0;
const uint8_t MPU = 0x68;

#if USAR_TOF_LOCAL
#include <VL53L0X.h>
VL53L0X tof;
bool tofOk = false;
int tofLocalMm = 0;
unsigned long tofLocalT = 0;
#endif

Servo servo;

// --- Encoders en cuadratura ×4 ------------------------------------------------
// C1 en INT0/INT1, C2 en interrupción por cambio de pin (D4 = PCINT20, D10 = PCINT2).
static const int8_t QDEC[16] = {0, -1, 1, 0, 1, 0, 0, -1, -1, 0, 0, 1, 0, 1, -1, 0};
volatile long encIzq = 0, encDer = 0;
volatile uint8_t prevIzq = 0, prevDer = 0;

static inline uint8_t leerIzq() { return (((PIND >> 2) & 1) << 1) | ((PIND >> 4) & 1); }  // D2, D4
static inline uint8_t leerDer() { return (((PIND >> 3) & 1) << 1) | ((PINB >> 2) & 1); }  // D3, D10

void isrIzq() { uint8_t s = leerIzq(); encIzq += QDEC[(prevIzq << 2) | s]; prevIzq = s; }
void isrDer() { uint8_t s = leerDer(); encDer += QDEC[(prevDer << 2) | s]; prevDer = s; }
ISR(PCINT2_vect) { isrIzq(); }   // D4
ISR(PCINT0_vect) { isrDer(); }   // D10
ISR(PCINT1_vect) {}              // A2: solo despierta del reposo profundo

// --- Estado -------------------------------------------------------------------
float objetivo[2] = {0, 0};      // lo que pidió la Pi (−255…255)
float rampa[2] = {0, 0};         // objetivo después de la rampa
float integ[2] = {0, 0};
long encPrevCiclo[2] = {0, 0};
unsigned long ultimoM = 0;

float pitch = 0, roll = 0, inclinacion = 0, rumbo = 0, sesgoGz = 0;
bool imuOk = false;

int distPiMm = 0;
unsigned long distPiT = 0;

float cabeza = 0, cabezaObjetivo = 0;
float bateria = 0;
unsigned long batBajaDesde = 0;

bool fTilt = false, fObstaculo = false, fWatchdog = false, fBatBaja = false, fBatCritica = false;
bool dormido = false;

long apagarEnMs = -1;            // P <s>: cuenta atrás para cortar la Pi
unsigned long apagarDesde = 0;

char linea[64];
uint8_t largo = 0;
bool desborde = false;

// ============================ MOTORES ========================================
// pwm > 0 adelante, < 0 atrás, 0 freno (IN1 = IN2 = HIGH con EN alto: corto del motor).
void motor(uint8_t en, uint8_t a, uint8_t b, int pwm, bool invertir) {
  if (invertir) pwm = -pwm;
  if (pwm > 0) {
    digitalWrite(a, HIGH); digitalWrite(b, LOW); analogWrite(en, pwm);
  } else if (pwm < 0) {
    digitalWrite(a, LOW); digitalWrite(b, HIGH); analogWrite(en, -pwm);
  } else {
    digitalWrite(a, HIGH); digitalWrite(b, HIGH); analogWrite(en, 255);
  }
}

void motores(int izq, int der) {
  motor(PIN_ENA, PIN_IN1, PIN_IN2, izq, INVERTIR_MOTOR_IZQ);
  motor(PIN_ENB, PIN_IN3, PIN_IN4, der, INVERTIR_MOTOR_DER);
}

// Sin PWM ni freno: para dormir (el L298N no tiene modo sleep).
void motoresLibres() {
  analogWrite(PIN_ENA, 0); analogWrite(PIN_ENB, 0);
  digitalWrite(PIN_IN1, LOW); digitalWrite(PIN_IN2, LOW);
  digitalWrite(PIN_IN3, LOW); digitalWrite(PIN_IN4, LOW);
}

void frenar() {
  for (uint8_t i = 0; i < 2; i++) { objetivo[i] = 0; rampa[i] = 0; integ[i] = 0; }
  motores(0, 0);
}

long leerEnc(uint8_t i) {
  noInterrupts();
  long v = i == 0 ? encIzq : encDer;
  interrupts();
  if (i == 0 && INVERTIR_ENC_IZQ) v = -v;
  if (i == 1 && INVERTIR_ENC_DER) v = -v;
  return v;
}

// Un ciclo de 10 ms: rampa → PID → PWM.
void controlRuedas() {
  int salida[2];
  for (uint8_t i = 0; i < 2; i++) {
    float d = objetivo[i] - rampa[i];
    rampa[i] += constrain(d, -RAMPA, RAMPA);

    long e = leerEnc(i);
    float medido = e - encPrevCiclo[i];
    encPrevCiclo[i] = e;

    if (rampa[i] == 0) { integ[i] = 0; salida[i] = 0; continue; }
#if USAR_PID
    float deseado = rampa[i] * PULSOS_MAX_CICLO / 255.0;
    float err = deseado - medido;
    integ[i] = constrain(integ[i] + err, -200, 200);
    float u = rampa[i] + KP * err + KI * integ[i];   // prealimentación + PI
    // Nunca invertir el sentido para frenar: eso lo hacen la rampa y el freno.
    if (rampa[i] > 0) u = constrain(u, 0, 255); else u = constrain(u, -255, 0);
    salida[i] = (int)u;
#else
    salida[i] = (int)rampa[i];
#endif
  }
  motores(salida[0], salida[1]);
}

// ============================ IMU MPU6050 ====================================
void mpuEscribir(uint8_t reg, uint8_t v) {
  Wire.beginTransmission(MPU); Wire.write(reg); Wire.write(v); Wire.endTransmission();
}

uint8_t mpuLeer(uint8_t reg) {
  Wire.beginTransmission(MPU); Wire.write(reg);
  if (Wire.endTransmission(false) != 0) return 0xFF;
  if (Wire.requestFrom(MPU, (uint8_t)1) != 1) return 0xFF;
  return Wire.read();
}

bool mpuMuestras(int16_t *ax, int16_t *ay, int16_t *az, int16_t *gx, int16_t *gy, int16_t *gz) {
  Wire.beginTransmission(MPU); Wire.write(0x3B);
  if (Wire.endTransmission(false) != 0) return false;
  if (Wire.requestFrom(MPU, (uint8_t)14) != 14) return false;
  int16_t v[7];
  for (uint8_t i = 0; i < 7; i++) v[i] = (Wire.read() << 8) | Wire.read();
  *ax = v[0]; *ay = v[1]; *az = v[2]; *gx = v[4]; *gy = v[5]; *gz = v[6];  // v[3] = temperatura
  return true;
}

void mpuConfigurarNormal() {
  mpuEscribir(0x6B, 0x01);   // despierto, reloj del giroscopio X
  mpuEscribir(0x6C, 0x00);   // todos los ejes activos
  mpuEscribir(0x1A, 0x03);   // filtro paso bajo 44 Hz
  mpuEscribir(0x1B, 0x08);   // giroscopio ±500 °/s → 65.5 LSB/(°/s)
  mpuEscribir(0x1C, 0x08);   // acelerómetro ±4 g → 8192 LSB/g
  mpuEscribir(0x38, 0x00);   // sin interrupciones
}

bool iniciarImu() {
  if (mpuLeer(0x75) == 0xFF) return false;   // WHO_AM_I: 0x68 en el original, 0x70/0x72 en clones
  mpuConfigurarNormal();
  delay(50);
  // Sesgo del giroscopio Z: el robot tiene que estar quieto el primer segundo.
  long suma = 0; int n = 0;
  int16_t ax, ay, az, gx, gy, gz;
  for (int i = 0; i < 200; i++) {
    if (mpuMuestras(&ax, &ay, &az, &gx, &gy, &gz)) { suma += gz; n++; }
    delay(5);
  }
  if (n < 100) return false;
  sesgoGz = (float)suma / n;
  return true;
}

void leerImu(float dt) {
  int16_t ax, ay, az, gx, gy, gz;
  if (!imuOk || !mpuMuestras(&ax, &ay, &az, &gx, &gy, &gz)) { imuOk = false; return; }
  const float G = 65.5;
  float accPitch = atan2(-(float)ax, sqrt((float)ay * ay + (float)az * az)) * RAD_TO_DEG;
  float accRoll = atan2((float)ay, (float)az) * RAD_TO_DEG;
  // Filtro complementario: giroscopio a corto plazo, acelerómetro a largo plazo.
  pitch = 0.98 * (pitch + gy / G * dt) + 0.02 * accPitch;
  roll = 0.98 * (roll + gx / G * dt) + 0.02 * accRoll;
  inclinacion = max(fabs(pitch), fabs(roll));
  float gzDps = (gz - sesgoGz) / G;
  if (fabs(gzDps) > 0.3) rumbo += SIGNO_GIRO_Z * gzDps * dt;   // zona muerta contra la deriva
}

// ============================ DISTANCIA ======================================
#if USAR_TOF_LOCAL
void iniciarTof() {
  tof.setTimeout(50);
  tofOk = tof.init();
  if (tofOk) {
    tof.setMeasurementTimingBudget(33000);
    tof.startContinuous();
  }
}

// Sin bloquear: solo lee si ya hay medida (la función de la librería espera hasta 33 ms).
void leerTof(unsigned long ahora) {
  if (!tofOk) return;
  if ((tof.readReg(VL53L0X::RESULT_INTERRUPT_STATUS) & 0x07) == 0) return;
  uint16_t mm = tof.readReg16Bit(VL53L0X::RESULT_RANGE_STATUS + 10);
  tof.writeReg(VL53L0X::SYSTEM_INTERRUPT_CLEAR, 0x01);
  tofLocalMm = (mm > 0 && mm < 2000) ? mm : 0;   // 8190 = nada a la vista
  tofLocalT = ahora;
}
#endif

// Distancia al frente que usa el freno: gana la local; si no, la que mandó la Pi.
int distanciaMm(unsigned long ahora) {
#if USAR_TOF_LOCAL
  if (tofOk && ahora - tofLocalT < TOF_LOCAL_VALIDO_MS) return tofLocalMm;
#endif
  if (distPiMm > 0 && ahora - distPiT < D_VALIDO_MS) return distPiMm;
  return 0;
}

// ============================ BATERÍA ========================================
void leerBateria(unsigned long ahora) {
#if HAY_DIVISOR_BAT
  float v = analogRead(PIN_BAT) * (5.0 / 1023.0) * BAT_FACTOR;
  bateria = bateria == 0 ? v : 0.95 * bateria + 0.05 * v;
  // 5 s seguidos por debajo, para que un arranque de motor no dispare la bandera.
  if (bateria < BAT_BAJA_V) {
    if (batBajaDesde == 0) batBajaDesde = ahora | 1;
    if (ahora - batBajaDesde > 5000) {
      fBatBaja = true;
      if (bateria < BAT_CRITICA_V) fBatCritica = true;
    }
  } else if (bateria > BAT_BAJA_V + 0.3) {
    batBajaDesde = 0; fBatBaja = false; fBatCritica = false;
  }
#else
  (void)ahora;
  bateria = 0;   // 0.00 = no se mide (fuente de pared)
#endif
}

// ============================ SERVO ==========================================
void moverServo(float dt) {
  float paso = SERVO_GRADOS_S * dt;
  cabeza += constrain(cabezaObjetivo - cabeza, -paso, paso);
  if (!dormido) {
    float g = SERVO_INVERTIDO ? -cabeza : cabeza;
    servo.writeMicroseconds(1500 + (int)(g * 1000.0 / 90.0));   // ±90° ≈ 500…2500 µs
  }
}

// ============================ REPOSO PROFUNDO ================================
// La Pi ya se apagó (P <s>): cortar su alimentación y dormir hasta que la IMU
// detecte movimiento. Al despertar se reconecta la Pi y el Arduino se reinicia
// cuando ella abre el puerto serial.
void reposoProfundo() {
  frenar();
  motoresLibres();
  servo.detach();
#if USAR_TOF_LOCAL
  if (tofOk) tof.stopContinuous();
#endif
  digitalWrite(PIN_CORTE_PI, NIVEL_CORTE_PI);
  Serial.flush();

  if (imuOk) {
    mpuEscribir(0x1C, 0x09);   // ±4 g con filtro paso alto de 5 Hz (requisito de la detección)
    mpuEscribir(0x1F, 20);     // umbral de movimiento: 20 × 2 mg = 40 mg
    mpuEscribir(0x20, 1);      // duración 1 ms
    mpuEscribir(0x37, 0x20);   // INT activo alto y retenido hasta leer INT_STATUS
    mpuEscribir(0x38, 0x40);   // interrupción de movimiento
    mpuEscribir(0x6C, 0x47);   // ciclo a 5 Hz, giroscopios apagados
    mpuEscribir(0x6B, 0x20);   // modo ciclo: ~10 µA
    mpuLeer(0x3A);             // limpiar
  }

  // Sin IMU, A2 con pull-up nunca cambia: duerme hasta que se apague el interruptor.
  pinMode(PIN_MPU_INT, imuOk ? INPUT : INPUT_PULLUP);
  PCMSK1 |= _BV(PCINT10);      // A2
  PCIFR |= _BV(PCIF1);
  PCICR |= _BV(PCIE1);
  ADCSRA &= ~_BV(ADEN);

  for (;;) {
    set_sleep_mode(SLEEP_MODE_PWR_DOWN);
    noInterrupts();
    sleep_enable();
    interrupts();
    sleep_cpu();               // aquí se queda hasta el cambio en A2
    sleep_disable();
    delay(5);
    if (imuOk && digitalRead(PIN_MPU_INT) == HIGH) break;   // movimiento de verdad, no ruido
  }

  PCICR &= ~_BV(PCIE1);
  PCMSK1 &= ~_BV(PCINT10);
  ADCSRA |= _BV(ADEN);
  if (imuOk) { mpuLeer(0x3A); mpuConfigurarNormal(); }
#if USAR_TOF_LOCAL
  if (tofOk) tof.startContinuous();
#endif
  digitalWrite(PIN_CORTE_PI, !NIVEL_CORTE_PI);   // la Pi arranca (~25 s)
  servo.attach(PIN_SERVO, 500, 2500);
  dormido = false;
  apagarEnMs = -1;
  ultimoM = millis();
}

// ============================ ÓRDENES ========================================
bool entero(const char *s, long *v) {
  if (!s || !*s) return false;
  char *fin;
  *v = strtol(s, &fin, 10);
  return *fin == '\0';
}

void responderEstado(unsigned long ahora) {
  Serial.print(F("OK V=")); Serial.print(bateria, 2);
  Serial.print(F(" T=")); Serial.print(inclinacion, 1);
  Serial.print(F(" Y=")); Serial.print(rumbo, 1);
  Serial.print(F(" E=")); Serial.print(leerEnc(0)); Serial.print(','); Serial.print(leerEnc(1));
  Serial.print(F(" H=")); Serial.print((int)round(cabeza));
  Serial.print(F(" D=")); Serial.print(distanciaMm(ahora));
  Serial.print(F(" F="));
  bool alguna = false;
  const char *nombres[] = {"bateria", "imu", "obstacle", "sleep", "tilt", "watchdog"};
  bool activas[] = {fBatBaja, !imuOk, fObstaculo, dormido, fTilt, fWatchdog};
  for (uint8_t i = 0; i < 6; i++) {
    if (!activas[i]) continue;
    if (alguna) Serial.print(',');
    Serial.print(nombres[i]);
    alguna = true;
  }
  if (!alguna) Serial.print('-');
  Serial.println();
}

void ejecutar(char *s) {
  unsigned long ahora = millis();
  char *tok[4];
  uint8_t n = 0;
  for (char *t = strtok(s, " "); t && n < 4; t = strtok(NULL, " ")) tok[n++] = t;
  if (n == 0 || strlen(tok[0]) != 1) { Serial.println(F("ERR syntax")); return; }
  char op = tok[0][0];
  long a = 0, b = 0;

  switch (op) {
    case 'M': {
      if (n != 3 || !entero(tok[1], &a) || !entero(tok[2], &b)) break;
      if (a < -255 || a > 255 || b < -255 || b > 255) { Serial.println(F("ERR range")); return; }
      if (dormido) { Serial.println(F("ERR sleep")); return; }
      if (fTilt) { Serial.println(F("ERR tilt")); return; }
      if (fBatCritica) { frenar(); Serial.println(F("ERR bateria")); return; }
      int d = distanciaMm(ahora);
      if (a + b > 0 && d > 0 && d < OBSTACULO_MM) { frenar(); Serial.println(F("ERR obstacle")); return; }
      objetivo[0] = a; objetivo[1] = b;
      ultimoM = ahora;
      fWatchdog = false;
      Serial.println(F("OK"));
      return;
    }
    case 'S':
      if (n != 1) break;
      frenar();
      Serial.println(F("OK"));
      return;
    case 'H':
      if (n != 2 || !entero(tok[1], &a)) break;
      if (a < -90 || a > 90) { Serial.println(F("ERR range")); return; }
      if (dormido) { Serial.println(F("ERR sleep")); return; }
      cabezaObjetivo = a;
      Serial.println(F("OK"));
      return;
    case 'D':
      if (n != 2 || !entero(tok[1], &a)) break;
      if (a < 0 || a > 4000) { Serial.println(F("ERR range")); return; }
      distPiMm = a; distPiT = ahora;
      Serial.println(F("OK"));
      return;
    case '?':
      if (n != 1) break;
      responderEstado(ahora);
      return;
    case 'Z':
      if (n != 1) break;
      frenar();
      motoresLibres();
      servo.detach();          // ~100 mA → ~5 mA
      dormido = true;
      Serial.println(F("OK"));
      return;
    case 'W':
      if (n != 1) break;
      if (dormido) {
        servo.attach(PIN_SERVO, 500, 2500);
        motores(0, 0);
      }
      dormido = false;
      Serial.println(F("OK"));
      return;
    case 'I':
      if (n != 1) break;
      Serial.println(F("OK BB8 fw=" FW_VERSION));
      return;
    case 'P':   // P <s>: la Pi se apaga; cortarle la corriente en s segundos y dormir. P 0 cancela.
      if (n != 2 || !entero(tok[1], &a)) break;
      if (a != 0 && (a < 5 || a > 120)) { Serial.println(F("ERR range")); return; }
      if (a == 0) { apagarEnMs = -1; Serial.println(F("OK")); return; }
      frenar();
      apagarEnMs = a * 1000L;
      apagarDesde = ahora;
      Serial.println(F("OK"));
      return;
  }
  Serial.println(F("ERR syntax"));
}

void leerSerial() {
  while (Serial.available()) {
    char c = Serial.read();
    if (c == '\r') continue;
    if (c == '\n') {
      if (desborde) Serial.println(F("ERR syntax"));
      else if (largo > 0) { linea[largo] = '\0'; ejecutar(linea); }
      else Serial.println(F("ERR syntax"));
      largo = 0; desborde = false;
    } else if (largo < sizeof(linea) - 1) {
      linea[largo++] = c;
    } else {
      desborde = true;
    }
  }
}

// ============================ CICLO ==========================================
void setup() {
  pinMode(PIN_CORTE_PI, OUTPUT);
  digitalWrite(PIN_CORTE_PI, !NIVEL_CORTE_PI);   // la Pi sigue encendida
  pinMode(PIN_ENA, OUTPUT); pinMode(PIN_IN1, OUTPUT); pinMode(PIN_IN2, OUTPUT);
  pinMode(PIN_ENB, OUTPUT); pinMode(PIN_IN3, OUTPUT); pinMode(PIN_IN4, OUTPUT);
  motores(0, 0);

  pinMode(2, INPUT_PULLUP); pinMode(4, INPUT_PULLUP);
  pinMode(3, INPUT_PULLUP); pinMode(10, INPUT_PULLUP);
  prevIzq = leerIzq(); prevDer = leerDer();
  attachInterrupt(digitalPinToInterrupt(2), isrIzq, CHANGE);
  attachInterrupt(digitalPinToInterrupt(3), isrDer, CHANGE);
  PCMSK2 |= _BV(PCINT20);   // D4
  PCMSK0 |= _BV(PCINT2);    // D10
  PCICR |= _BV(PCIE2) | _BV(PCIE0);

  Serial.begin(115200);
  Wire.begin();
  Wire.setClock(400000);
  Wire.setWireTimeout(3000, true);   // un bus I2C colgado por ruido de motores no congela el lazo

  servo.attach(PIN_SERVO, 500, 2500);
  servo.writeMicroseconds(1500);

  imuOk = iniciarImu();
#if USAR_TOF_LOCAL
  iniciarTof();
#endif
  leerBateria(millis());
  ultimoM = millis();
}

void loop() {
  static unsigned long previo = micros();
  leerSerial();

  unsigned long t = micros();
  if (t - previo < 10000) return;
  float dt = (t - previo) * 1e-6;
  previo = t;
  unsigned long ahora = millis();

  leerImu(dt);
#if USAR_TOF_LOCAL
  leerTof(ahora);
#endif
  leerBateria(ahora);

  // Reflejo 1: inclinación (con histéresis).
  if (inclinacion > TILT_FRENO) {
    if (!fTilt) frenar();
    fTilt = true;
    objetivo[0] = objetivo[1] = 0;
  } else if (inclinacion < TILT_LIBERA) {
    fTilt = false;
  }

  // Reflejo 2: obstáculo al frente mientras avanza.
  int d = distanciaMm(ahora);
  if (d > 0 && d < OBSTACULO_MM) {
    if (objetivo[0] + objetivo[1] > 0 || rampa[0] + rampa[1] > 0) frenar();
    fObstaculo = true;
  } else {
    fObstaculo = false;
  }

  // Reflejo 3: watchdog. Solo M lo alimenta.
  if ((objetivo[0] != 0 || objetivo[1] != 0) && ahora - ultimoM > WATCHDOG_MS) {
    frenar();
    fWatchdog = true;
  }

  // Reflejo 4: batería crítica.
  if (fBatCritica && (objetivo[0] != 0 || objetivo[1] != 0)) frenar();

  if (!dormido) controlRuedas();
  moverServo(dt);

  if (apagarEnMs >= 0 && (long)(ahora - apagarDesde) >= apagarEnMs) {
    reposoProfundo();
    previo = micros();
  }
}
