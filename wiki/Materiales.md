# Materiales

## Ya comprado: pedido UNIT Electronics 377466 (8 oct 2026)

Total MXN 1,898.25 con envío (Estafeta). Precios por unidad.

| Pieza | Cant. | MXN | Parte |
| --- | --- | --- | --- |
| Motorreductor JGB37-520B 12 V 319 RPM con encoder | 2 | 231 | 1 |
| Driver L298N (uno de repuesto) | 2 | 59 | 1 |
| IMU MPU6050 6 DOF | 1 | 87 | 1 |
| Servo MG996R engranes metálicos (uno de repuesto) | 2 | 114 | 1 |
| Buck LM2596 ajustable 3 A | 1 | 42 | 1 |
| Sensor ToF láser VL53L0X I2C | 1 | 47 | 1 |
| Condensador electrolítico 1000 µF 16 V | 4 | 3 | 1 |
| Adaptador 12 V 2 A + par plug/jack DC | 1 | 83 + 14 | 1 |
| Protoboard 830 + fuente MB102 + 65 jumpers | 1 | 90 | 1 |
| Anillo WS2812 16 LED 45 mm | 1 | 58 | 5 |
| Amplificador PAM8403 5 V con volumen | 1 | 29 | 5 |
| Bocina 4 Ω 5 W | 2 | 80 | 5 |
| Par de conectores XT60 + par de cables XT60 12 AWG 15 cm | 1 | 18 + 59 | 6 |
| Checador de voltaje LiPo/Li-ion 1–8S | 1 | 53 | 6 |
| 25 tornillos M3×6 con tuerca, separadores de latón M3 (6 de 10 mm, 4 de 5 mm), cable 22 AWG rojo/negro 2 m c/u, termorretráctil 328 pzs, cinta doble cara 5 m | — | — | 2 |

**Cambio respecto al documento técnico:** el L298N reemplaza al DRV8871. Si se
calienta, el respaldo es un BTS7960. El L298N tira ~2 V, así que el motor ve ~10 V
(~0.9 m/s máx.). Pinout en [Protocolo](Protocolo.md#cambio-por-el-l298n).

## Falta comprar

| Pieza | Parte | Nota |
| --- | --- | --- |
| 2 ruedas 65 mm con cople hexagonal para eje de 6 mm | 2 | La de UNIT es para motor TT y no entra |
| 2 soportes de motor JGB37, 2 ball transfers 1″ | 2 | Mercado Libre |
| Melamina 12–15 mm 30 × 30 cm, tubo PVC 25 mm o varilla 1/4″ | 2 | Maderería, ferretería |
| Micrófono USB y bocina o DAC USB para la Pi principal | 4 | |
| Cámara OV5647 + cable de 15 pines para Zero | 5 | El documento técnico la marca como comprada, pero no está en el pedido 377466: confirmar |
| Power bank 18650 para la cabeza | 5 | |
| Tarjeta de sonido USB o MAX98357 para la Zero | 5 | La Zero no trae salida de audio |
| 3 bolas de acero 1/2″, mica acrílica ahumada Ø 50 × 3 mm, 8 imanes neodimio N42 20 × 5 mm avellanados | 5 | La cúpula se imprime ([Diseño 3D](Diseno-3D.md#cabeza)). Imanes: 4 con la cara N avellanada (poste) y 4 con la S (cabeza) |
| LiPo 3S 11.1 V 5200 mAh XT60 | 6 | 2.5–3 h de uso activo |
| Cargador balanceador iMAX B6AC | 6 | Versión AC |
| Interruptor rocker 10 A, portafusible + fusibles 5 A | 6 | |
| Segundo LM2596 (solo para el servo) | 6 | Ver [Parte 6](Parte-6-Esfera-y-baterias.md) |
| P-MOSFET de lado alto (AO3401 o módulo) + 100 kΩ | 6 | Reposo profundo; el IRF520 no sirve |
| Resistencias 10 kΩ y 4.7 kΩ | 6 | Divisor de batería en A0 |
| Filamento PLA 2–3 kg, pintura, primer, laca, lastre 1–1.5 kg | 6 | Tornillería de las piezas impresas: [Diseño 3D](Diseno-3D.md#tornillería-y-extras) |

La lista completa con tiendas y notas de compra está en el
[documento técnico](https://github.com/erickdsama/bb8/blob/main/docs/proyecto-bb8-agente-llm.md#3-materiales).

## Estimaciones

- Peso total 2–2.5 kg.
- Velocidad útil ~0.6 m/s (PWM limitado a ~55 %).
- 2.5–3 h de uso activo con 5200 mAh.
