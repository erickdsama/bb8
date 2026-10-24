# Parte 6 · Esfera y baterías

**Entregable:** esfera de 30–35 cm en dos mitades que cierran sin cinta, con la base
rodando adentro y la cabeza flotando encima. El BB-8 completo, sin cable, que se
duerme solo y despierta al moverlo.

Código: [`partes/6-esfera-baterias`](https://github.com/erickdsama/bb8/tree/main/partes/6-esfera-baterias)
(gestor de energía `energia`).

## Alimentación con la LiPo

LiPo 3S 11.1 V 5.2 Ah (XT60) → fusible 5 A → interruptor rocker → L298N, buck de la Pi
y buck del servo. La cabeza sigue con su power bank.

- **Segundo buck para el servo.** Un LM2596 da ~2 A reales; la Pi 3 pide hasta ~1.2 A con Whisper en los 4 núcleos y un MG996R trabado 2.5 A. Usa uno para la Pi (con el condensador de 1000 µF, a 5.1 V) y otro solo para el servo.
- **El Arduino come por VIN desde la batería** (después del interruptor), no solo por el USB de la Pi, para seguir vivo cuando corta la Pi.
- Nunca bajar de 3.3 V por celda (~10 V); la alarma de voltaje (en el pedido) se pone a 3.5 V.
- Divisor 10 kΩ / 4.7 kΩ de la batería a **A0** y `HAY_DIVISOR_BAT 1` en el firmware: así `?` reporta `V=` y el firmware rechaza `M` por debajo de 9.9 V.
- **Carga inalámbrica en un plato:** Bebocho rueda encima del plato (o lo pones tú) y carga. No importa hacia dónde mire: la bobina receptora va abajo y abajo siempre es abajo.
  - El plato (`cad/plato.scad`) mide Ø34 cm: sube por una rampa de ~7° a una orilla de 8 mm y cae a un hoyo de 2.5 mm (máx. ~3°) que lo deja centrado. Para salir solo sube esos ~3°; Bebocho, que se mueve inclinando su base como péndulo, aguanta 10–15°.
  - La base siempre queda abajo y gira alrededor del centro de la esfera, así que la bobina receptora, bajo la charola, queda siempre a 4 mm de la pared. La emisora va en el centro del plato bajo 1 mm de plástico: 9–10 mm de bobina a bobina.
  - Eliminador 24 V → emisor (en una cajita atrás del plato) → bobina emisora. Receptor → cargador CC/CV a **12.6 V y 1.5 A** → BMS 3S con balance → LiPo (~4 h de vacía a llena). El LED del panel prende mientras recibe corriente.
  - El receptor y el cargador van pegados con cinta doble cara bajo la plataforma, con los componentes hacia abajo, entre los soportes de motor y la pestaña de atrás de la charola. El BMS va de canto, detrás del cargador. Los dos pasacables quedan libres.
  - El rocker del panel queda adentro: solo para apagar del todo al abrir la esfera. En uso normal se duerme solo.
  - Cada ~10 cargas, o si las celdas se separan más de 0.05 V, abrir la esfera y cargar en modo Balance con el iMAX B6AC.
  - Pendiente: que el robot sepa que está cargando (para quedarse quieto en el plato) y que vaya solo al plato con "ve a cargarte" o con batería baja (necesita la cámara de la cabeza). Hoy solo se ve en el voltaje de la batería.

Diagrama "banco contra LiPo" en el artefacto [Diagramas](Artefactos.md#diagramas), sección 2.

## Reposo

```bash
python -m energia            # reposo ligero; el profundo solo con BB8_REPOSO_PROFUNDO=1
python -m energia --prueba   # tiempos cortos (10 s / 20 s / 40 s) y sin apagar la Pi
```

```
activo ──30 s sin actividad──▶ escuchando ──5 min──▶ ligero ──30 min o "a dormir"──▶ profundo
cualquier actividad (wake word, orden MCP, mando, alguien lo mueve) ──▶ activo
```

- **Ligero:** `Z` al Arduino (servo sin PWM, motores libres), cámara pausada y ojo tenue.
- **Profundo:** `P <s>` al Arduino y apagado de la Pi; el Arduino corta la Pi y la vuelve a encender cuando la IMU nota movimiento (~25 s de arranque).
- **Batería:** aviso a 10.5 V; a 10.0 V se duerme (profundo, o ligero si no hay MOSFET).
- API en `127.0.0.1:8771`: `POST /actividad`, `POST /dormir`, `GET /estado`.

### Corte de la Pi para el reposo profundo

El corte va en **A3** con un **P-MOSFET de lado alto** de nivel lógico (p. ej. AO3401 o
un módulo "high-side") y pull-down de 100 kΩ en la compuerta: A3 en alto apaga la Pi, y
un reset del Arduino (pasa cada vez que la Pi abre el puerto) la deja encendida. El
módulo IRF520 del documento técnico corta la tierra y **no sirve** aquí: la Pi y el
Arduino comparten tierra por el USB.

Con el MOSFET instalado: `BB8_REPOSO_PROFUNDO=1` en `/etc/bb8.env` y
`sudo systemctl enable --now bb8-energia`. El instalador ya da permiso al usuario `bb8`
para `systemctl poweroff`.

## Esfera

El modelo ya está hecho: [Diseño 3D](Diseno-3D.md#esfera) tiene los 14 gajos, los 6
círculos, la bayoneta de la junta y el plato de carga, en OpenSCAD paramétrico y en STL.

1. Imprime los gajos de [`cad/stl/fase2-esfera-y-cabeza`](https://github.com/erickdsama/bb8/tree/main/cad/stl/fase2-esfera-y-cabeza) (6 círculos naranjas + 14 piezas blancas que forman los 8 gajos), con espigas de filamento de 1.75 mm.
2. Imprime con capa de 0.28 mm y 10–15 % de relleno (~2–3 kg de PLA).
3. Pega, **refuerza por dentro** (fibra de vidrio o segunda capa de resina), lija, primer, pinta y laca.
4. Prueba que la base rueda dentro de la esfera **antes** de poner imanes y lastre; si las ruedas giran pero la esfera no, las ruedas locas tocan primero.
5. Recorta el poste para que los imanes queden a 2–3 mm de la pared.
6. Lastre hasta 1–1.5 kg en total, lo más abajo posible; prueba que la base no vuelque al frenar.
7. Cabeza: cúpula impresa con 3 bolas de acero de 1/2″ para que los imanes no rocen la pintura (unas ruedecitas solo ruedan en un sentido). Imanes N42 20 × 5 mm: 4 en el poste cara N arriba, 4 en la cabeza cara S abajo.

Los N42 de 20 mm lastiman dedos y rompen piezas impresas: sepáralos con una cuña de
madera, nunca deslizando.

Ver el artefacto [Mecánica](Artefactos.md#mecánica) y [Materiales](Materiales.md) para
lo que falta comprar (batería, cargador, imanes, filamento).

## Lista cuando

- [ ] LiPo cargada en modo Balance, 12.6 V con celdas parejas
- [ ] `?` reporta el voltaje real (`HAY_DIVISOR_BAT 1`)
- [ ] La esfera rueda con la base adentro y la cabeza no se cae al frenar
- [ ] Se duerme solo y despierta al moverlo
