# Parte 6 · Esfera y baterías

**Entregable:** esfera de 30–35 cm en dos mitades que cierran sin cinta, con la base
rodando adentro y la cabeza flotando encima. El BB-8 completo, sin cable, que se
duerme solo y despierta al moverlo.

Código: [`partes/6-esfera-baterias`](https://github.com/erickdsama/bb8/tree/main/partes/6-esfera-baterias)
(gestor de energía `energia`).

## Alimentación con la LiPo

LiPo 3S 11.1 V 5.2 Ah (XT60) → fusible 5 A → interruptor rocker → L298N, buck de la Pi
y buck del servo. La cabeza sigue con su power bank.

- **Un buck para la Pi y otro para el servo.** La Pi principal es una Pi 3: pide 5.1 V 2.5 A, y un LM2596 da ~2 A reales. Aliméntala con un XL4015 de 5 A ajustado a 5.1 V (con el condensador de 1000 µF) por un cable micro-USB corto y grueso. El LM2596 queda solo para el servo, que trabado pide 2.5 A.
- **El Arduino come por VIN desde la batería** (después del interruptor), no solo por el USB de la Pi, para seguir vivo cuando corta la Pi.
- Nunca bajar de 3.3 V por celda (~10 V); la alarma de voltaje (en el pedido) se pone a 3.5 V.
- Divisor 10 kΩ / 4.7 kΩ de la batería a **A0** y `HAY_DIVISOR_BAT 1` en el firmware: así `?` reporta `V=` y el firmware rechaza `M` por debajo de 9.9 V.
- Carga: tapa en uno de los círculos naranjas con un XT60 hembra y un JST-XH de panel; rocker apagado, iMAX B6AC en modo Balance a 2.6 A (~2 h). La batería nunca sale del robot.

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

1. Modela en Fusion 360: 6 círculos naranjas + 8 piezas blancas, cortadas para la cama de tu impresora, con espigas de alineación.
2. Imprime con capa de 0.28 mm y 10–15 % de relleno (~2–3 kg de PLA).
3. Pega, **refuerza por dentro** (fibra de vidrio o segunda capa de resina), lija, primer, pinta y laca.
4. Prueba que la base rueda dentro de la esfera **antes** de poner imanes y lastre; si las ruedas giran pero la esfera no, los ball transfers tocan primero.
5. Recorta el poste para que los imanes queden a 2–3 mm de la pared.
6. Lastre hasta 1–1.5 kg en total, lo más abajo posible; prueba que la base no vuelque al frenar.
7. Cabeza: cúpula impresa con 3 ruedecitas para que los imanes no rocen la pintura. Imanes N42 20 × 5 mm: 4 en el poste cara N arriba, 4 en la cabeza cara S abajo.

Los N42 de 20 mm lastiman dedos y rompen piezas impresas: sepáralos con una cuña de
madera, nunca deslizando.

Ver el artefacto [Mecánica](Artefactos.md#mecánica) y [Materiales](Materiales.md) para
lo que falta comprar (batería, cargador, imanes, filamento).

## Lista cuando

- [ ] LiPo cargada en modo Balance, 12.6 V con celdas parejas
- [ ] `?` reporta el voltaje real (`HAY_DIVISOR_BAT 1`)
- [ ] La esfera rueda con la base adentro y la cabeza no se cae al frenar
- [ ] Se duerme solo y despierta al moverlo
