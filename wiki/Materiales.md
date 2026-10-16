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
| Anillo WS2812 16 LED 45 mm (datos a **GPIO10 / pin 19** de la Zero, por SPI; ya no GPIO18) | 1 | 58 | 5 |
| Amplificador PAM8403 5 V con volumen | 1 | 29 | 5 |
| Bocina 4 Ω 5 W | 2 | 80 | 5 |
| Par de conectores XT60 + par de cables XT60 12 AWG 15 cm | 1 | 18 + 59 | 6 |
| Checador de voltaje LiPo/Li-ion 1–8S | 1 | 53 | 6 |
| 25 tornillos M3×6 con tuerca, separadores de latón M3 (6 de 10 mm, 4 de 5 mm), cable 22 AWG rojo/negro 2 m c/u, termorretráctil 328 pzs, cinta doble cara 5 m | — | — | 2 |

El VL53L0X de este pedido va en el I2C del Arduino durante las Partes 1–4
(`USAR_TOF_LOCAL = 1`). Al cerrar la esfera ya no ve hacia afuera y se muda a la cabeza
(`USAR_TOF_LOCAL = 0`); no hace falta comprar otro.

**Cambio respecto al documento técnico:** el L298N reemplaza al DRV8871. Si se
calienta, el respaldo es un BTS7960. El L298N tira ~2 V, así que el motor ve ~10 V
(~0.9 m/s máx.). Pinout en [Protocolo](Protocolo.md#cambio-por-el-l298n).

## Ya tienes (no se compran)

Revisado el 15 oct 2026 entrando por SSH a las dos Pi.

| Pieza | Detalle | Parte |
| --- | --- | --- |
| Arduino Uno R3 + cable USB | Conectado por USB a la Pi 3, aparece como `/dev/ttyACM0` | 1 |
| Raspberry Pi 3 Model B+ (1 GB) | Hostname `bb8`. Debian 13 (trixie) de 64 bits, arranca de una **memoria USB de 8 GB** con 3.8 GB libres | 3 |
| Raspberry Pi Zero W (la original) | Hostname `bb8-head`. **No es Zero 2 W**: armv6, un núcleo, 512 MB. Raspbian 13 (trixie) en microSD de 16 GB. I2C apagado todavía | 5 |

**Lo que cambia por ser Zero W y no Zero 2 W:** Piper (la voz) no tiene versión armv6, así
que la cabeza no puede generar voz; la bocina se queda en el cuerpo (Pi 3 → PAM8403) o
la Pi 3 genera el audio y se lo manda a la cabeza. La cámara, el ojo y el ToF funcionan
igual; la detección de caras con OpenCV será lenta (un núcleo). `instalar_zero.sh` está
escrito para la Zero 2 W y hay que adaptarlo. Si quieres la voz en la cabeza, la Zero 2 W
está en el carrito como opcional.

## Falta comprar: cotización del 9 oct 2026

Actualizada el 15 oct 2026 con lo decidido al probar el simulador: ReSpeaker 2-Mic en
lugar del micrófono USB, cámara gran angular, red WiFi propia del robot y más
almacenamiento para la Pi 3.

Precios en MXN tomados de las fichas de cada tienda el 9 de octubre de 2026, sin envío;
cambian seguido, así que confirma antes de pagar. "Est." es un estimado sin ficha
consultada (tienda física o sin precio visible). Nada de esto se ha comprado.

### Falta para la Parte 1 (detectado al armar, 15 oct 2026)

| Pieza | Cant. | Tienda | MXN | Nota |
| --- | --- | --- | --- | --- |
| Cable JST PH 2.0 mm de 6 pines a Dupont hembra, 20 cm | 2 | Mercado Libre / UNIT | 50–80 | El motor trae el conector (M1, GND, C2, C1, VCC, M2) pero no el cable |
| Cautín 60 W con control de temperatura + estaño con flux + base | 1 | Mercado Libre / Amazon | 250–350 | El MPU6050 y el VL53L0X vienen con los pines sueltos; sin soldar el escáner I2C no los ve |
| Multímetro básico | 1 | Mercado Libre / Steren | ~200 | Para ajustar el LM2596 a 5.1 V antes de conectar el servo |

### Carrito recomendado, ahora (Partes 2, 4 y 5)

Se puede pedir ya; nada depende del tamaño final de la esfera.

| Pieza | Cant. | Tienda | MXN | Nota |
| --- | --- | --- | --- | --- |
| Llantas RC 1:10 de 65 × 26 mm con hexágono de 12 mm (paquete de 4) + acoplamiento hexagonal de latón WEFEINO (4 pzs, opción **6 mm**: barreno 6 mm, hexágono 12 mm, 18 mm de largo, prisionero y rosca M4) | 1 + 1 | Amazon | llantas + 253 | La llanta con accesorios de UNIT se agotó. Las RC miden lo mismo que el CAD (65 × 26 mm): nada cambia en `cad/` ni en `config.py`. El acoplamiento entra en el eje D de 6 mm con su prisionero y un tornillo M4 con rondana detiene la llanta. Sobran 2 llantas de repuesto. Respaldo sin hexágono: [brida de 6 mm](https://www.amazon.com.mx/Acoplamiento-Aluminio-Cojinete-Accesorio-Acoplador/dp/B0DB4WXRR9) atornillada a cualquier llanta de 65 mm |
| [Soporte en L para motor 37 mm](https://articulo.mercadolibre.com.mx/MLM-652432552-soporte-motorreductor-bracket-37d-37gb-en-l-_JM) | 2 | Mercado Libre | ~40 c/u (est.) | Ya no es opcional: la llanta nueva no trae escuadra. Se atornilla abajo de la melamina en lugar del soporte impreso del CAD. Alternativa: [uxcell, 4 pzs](https://www.amazon.com.mx/uxcell-Soporte-reductor-engranajes-unidades/dp/B07QX82WS2) en Amazon |
| Ball transfer 1″ de acero, montaje de 2 tornillos ([TruePower, 6 pzs](https://www.amazon.com.mx/TruePower-Roller-Ball-Transfer-Bearings/dp/B009KASQZW)) | 2 | Amazon | ~120 (est.) | Vienen en paquete; sobran para la cabeza o repuesto |
| Melamina 12–15 mm 30 × 30 cm + varilla roscada 1/4″ de 30 cm con 4 tuercas y 4 rondanas | 1 | Maderería, ferretería | ~120 (est.) | La varilla es el poste: va de pie en el centro de la melamina, apretada con 2 tuercas arriba y 2 abajo. Así no hace falta imprimir el pie del poste |
| ReSpeaker 2-Mic Pi HAT **v2.0** (Seeed) | 1 | Amazon / Mercado Libre | ~450 (est.) | Reemplaza al micrófono USB: con dos micrófonos se calcula de dónde viene la voz (`listen`, ver [Simulador](Simulador.md)). Va sobre el GPIO de la Pi 3 y trae su propia salida de audio. Comprar la **v2.0**: la v1 depende del driver `seeed-voicecard`, que se rompe con kernels nuevos; confirmar soporte para el kernel de trixie antes de pagar. No cancela eco por hardware |
| Cámara OV5647 5 MP **gran angular** (120–160°) | 1 | Amazon / Mercado Libre | ~200–300 (est.) | Confirmado: no la tienes. En el simulador, con la lente normal (54°) y a 35 cm del piso, una pared o un marco de puerta llena la foto; con 90–120° se ve el cuarto. Confirmar que traiga cable **15 → 22 pines** (la Zero W usa el de 22). Respaldo: [Arducam OV5647 normal](https://www.amazon.com.mx/Arducam-megap%C3%ADxeles-Sensor-OV5647-Raspberry/dp/B012V1HEP4), 134, trae los dos cables |
| Adaptador WiFi USB 2.4 GHz con modo AP (chip MediaTek, el MT7612U es lo más seguro) | 1 | Amazon / Mercado Libre | ~150–400 (est.) | Hace la red propia del robot (`bb8-net`): la Pi 3 es el punto de acceso y la cabeza se conecta ahí, aunque el robot salga del alcance del router. Opcional si el punto de acceso va en la WiFi integrada de la Pi 3, que es más frágil. Antes de comprar, confirmar en la ficha que soporte modo AP en Linux |
| Memoria USB 3.0 o microSD A1 de 32 GB | 1 | Steren / Amazon | ~120 (est.) | La Pi 3 arranca de una USB de 8 GB con 3.8 GB libres; la voz (Whisper, wake word, Piper) y el venv no caben con holgura |
| Raspberry Pi Zero 2 W (opcional) | 1 | UNIT / Amazon | ~400 (est.) | Solo si quieres la voz saliendo de la cabeza (Piper no corre en la Zero W) y caras más rápidas. Mismo tamaño y cable de cámara |
| [Amplificador I2S MAX98357A](https://www.amazon.com.mx/MAX98357A-amplificador-decodificaci%C3%B3n-selecci%C3%B3n-proyectos/dp/B0F1Y3KHJF) | 1 | Amazon | ~85 | Le da audio a la Zero y mueve directo una bocina de 4 Ω (3.2 W). Pack de 2: [DAOKAI](https://www.amazon.com.mx/DAOKAI-unidades-MAX98357-amplificador-decodificador/dp/B0B2NSMV55) 231 |
| [Power bank 1 × 18650, 2600 mAh](https://www.mercadolibre.com.mx/cargador-tipo-power-bank-para-bateria-18650-2600mah-incluida/up/MLMU3701037772) | 1 | Mercado Libre | ~250 (est.) | 97 × 26 mm, 63 g, salida 5 V 1 A, batería incluida. Para la cabeza |
| Imanes neodimio disco 20 × 5 mm, [paquete de 10](https://listado.mercadolibre.com.mx/imanes-de-neodimio-20x5) | 1 | Mercado Libre | ~607 | Hacen falta 8 (4 en el poste, 4 en la cabeza). Revisar que la ficha diga N42; 2 paquetes de 5 salen en ~568 |
| [Rueda loca mini, bola de nylon 1″](https://uelectronics.com/producto/mini-rueda-universal-con-bola-de-nylon/) | 3 | UNIT | 37–51 c/u | Las 3 "ruedecitas" de la cabeza; el nylon no raya la pintura del casco |

**Subtotal ahora: ~$2,550** sin la Zero 2 W opcional (~$2,400 si el punto de acceso va en la
WiFi integrada).

### Carrito recomendado, después (Parte 6)

Conviene esperar a que la Parte 3 ruede, por si cambia el tamaño de la esfera o el
consumo real de los motores.

| Pieza | Cant. | Tienda | MXN | Nota |
| --- | --- | --- | --- | --- |
| [LiPo 3S 11.1 V 5200 mAh 80C XT60, Hilldow (2 pzs)](https://www.amazon.com.mx/Hilldow-conector-veh%C3%ADculos-Camiones-paquetes/dp/B0FW4WPFNT) | 1 paquete | Amazon | 813 | ~407 por batería: más barato que una sola de [PCEONAMP](https://www.amazon.com.mx/PCEONAMP-capacidad-Quadcopter-Helic%C3%B3ptero-almacenamiento/dp/B0CHJHHK9V) (710). La segunda es repuesto |
| [Cargador balanceador iMAX B6AC 80 W](https://www.mercadolibre.com.mx/cargador-balanceador-imax-b6-b6ac-80w/p/MLM47578491) | 1 | Mercado Libre | 600–750 | La versión **AC** trae fuente interna; la B6 sin AC necesita una fuente de 12 V aparte. [HetPro](https://hetpro-store.com/cargador-de-baterias-lipo-80w/) la tiene en 850 |
| [Switch de balancín con piloto 12 Vcc, 16 A](https://www.steren.com.mx/switch-de-balancin-de-1-polo-1-tiro-2-posiciones-on-off-con-piloto-12-vcc.html) | 1 | Steren | 19 | |
| [Portafusible europeo de cartucho 5 × 20](https://www.steren.com.mx/porta-fusible-europeo-de-cartucho.html) + 3 fusibles 5 A 5 × 20 | 1 | Steren | 15 + ~12 | |
| Buck XL4015 5 A ajustable, para la Pi 3 ([Mercado Libre](https://listado.mercadolibre.com.mx/xl4015)) | 1 | Mercado Libre / UNIT | ~60 | La Pi 3 pide 5.1 V 2.5 A y el LM2596 da ~2 A reales: el XL4015 deja margen. El LM2596 del pedido 377466 se queda solo para el servo. [Versión con medidores en UNIT](https://uelectronics.com/producto/modulo-de-carga-reductor-xl4015-5a-con-medidores/): 140 |
| Cable micro-USB corto y grueso (o pigtail micro-USB macho) | 1 | Steren | ~50 (est.) | Del buck a la Pi 3. Un cable delgado provoca avisos de bajo voltaje; no alimentar por los pines GPIO |
| P-MOSFET AO3401 (SOT-23) | 2 | Mercado Libre | ~80 | Reposo profundo de la Pi; el IRF520 no sirve. Necesita placa adaptadora SOT-23 o soldar fino |
| Resistencias 100 kΩ, 10 kΩ y 4.7 kΩ | — | UNIT / Steren | ~30 (est.) | Pull-up del MOSFET y divisor de batería en A0 |
| XT60 hembra de panel + extensión JST-XH 4 pines | 1 | Amazon / UNIT | ~140 (est.) | Puerto de carga en la tapa del casco |
| Impresión 3D por servicio (gajos de la esfera y cabeza) | 1 | Servicio local | ~2,000–3,000 (est.) | No hay impresora: se manda a imprimir con los STL de `cad/stl` (~1.6 kg de PLA). Los soportes de motor se compran y el poste es varilla roscada |
| Primer, pintura blanca/naranja/plata, laca, lijas | 1 | Tlapalería | 300–500 (est.) | |
| Lastre (pesas o plomo) | 200–400 g | Mercado Libre | ~50 (est.) | |

**Subtotal Parte 6: ~$4,700** con la impresión por servicio. Total pendiente con los dos carritos: **~$7,250** más envíos.

### Cámara OV5647

Confirmado el 9 oct 2026: no la tienes y va en otro pedido, así que está en el carrito
de ahora. El 15 oct se cambió a gran angular después de verla en el simulador
(`lanzar_dummy.py --fov 54` contra `--fov 90`). Las lentes de 120–160° deforman las
orillas (ojo de pez), cosa que el simulador no imita. La
[de visión nocturna de UNIT](https://uelectronics.com/producto/camara-vision-nocturna-raspberry-pi-5mp-1080p-ov5647/)
cuesta 240 pero solo trae cable de 15 pines; con la Zero haría falta además un
[cable 15 → 22](https://www.amazon.com.mx/Onyehn-flexible-c%C3%A1mara-Raspberry-unidades/dp/B07XZ5DX5H).

### Lo que ya no hace falta

- **Bocina o DAC USB para la Pi principal:** ya llegaron 2 bocinas y un PAM8403, y la
  Pi 3 trae jack de 3.5 mm: jack → PAM8403 → bocina.
- **Tarjeta de sonido USB para la Zero:** la reemplaza el MAX98357A del carrito.
- **Micrófono USB mini SunFounder (191):** lo reemplaza el ReSpeaker 2-Mic HAT.
- **Cúpula de la cabeza:** la imprime el servicio junto con la esfera.
- **Checador de voltaje LiPo y conectores XT60 macho:** ya vienen en el pedido 377466.

### Notas de la cotización

- **Ruedas:** la "Llanta de Goma 65 mm" de UNIT con accesorios sí es para el JGB37
  (buje hexagonal de 12 mm con prisionero sobre el eje D de 6 mm). La que es para motor
  TT es otra: la "Llanta para Motorreductor TT de 66 × 27 mm".
- **Imanes:** los N42 de 20 mm lastiman dedos y rompen piezas impresas; separarlos con
  una cuña, nunca deslizando.
- **LiPo:** cargar siempre en modo Balance con el B6AC; el checador de voltaje del
  pedido 377466 sirve como alarma.
- Para la versión anterior de la lista, con todas las piezas y tiendas, ver el
  [documento técnico](https://github.com/erickdsama/bb8/blob/main/docs/proyecto-bb8-agente-llm.md#3-materiales).

## Estimaciones

- Peso total 2–2.5 kg.
- Velocidad útil ~0.6 m/s (PWM limitado a ~55 %).
- 2.5–3 h de uso activo con 5200 mAh.
