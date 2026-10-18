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
| Anillo WS2812 16 LED 45 mm (datos a **GPIO12**, puerto Grove digital del ReSpeaker; ver [pines](#pines-de-la-zero-w-con-el-respeaker)) | 1 | 58 | 5 |
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

## Ya comprado: Electrozone (16 oct 2026)

Tienda física (sucursales Tecnológico y Torres). Buscador: `uv run tools/electrozone.py <palabras>`.

| Clave | Pieza | Cant. | MXN | Parte |
| --- | --- | --- | --- | --- |
| AR-CARGA12 | Buck XL4015 5 A ajustable (para la Pi 3) | 1 | 75 | 3 |
| 699-927 | Cable USB a micro USB 1 m (se corta y se suelda al XL4015) | 1 | 40 | 3 |
| RL4 | Rueda loca con bola de ~1″, base 50 × 70 mm, 32 mm de alto | 2 | 140 | 2 |
| PM15 | Puntas de multímetro con caimanes | 1 | 30 | 1 |
| JUMPER-EXT20 / JUMPER-H20 | Jumpers macho-hembra y hembra-hembra 20 cm | 10 + 10 | 45 | 1 |
| R330E-1/4W, 4.7 kΩ, 10 kΩ, 100 kΩ | Resistencias | 5 + 2 + 2 + 2 | ~12 | 5, 6 |
| RL7 | Rueda loca mini, 16 mm de alto (ruedecitas de la cabeza) | 3 | 90 | 5 |
| IMAN-ND20x2 | Imán de neodimio 20 × 2 mm, para la prueba magnética | 4 | 100 | 5 |
| AR-GROVE4 | Cable Grove 4 pines a Dupont hembra (ReSpeaker) | 2 | 40 | 5 |
| SH12RO | Interruptor de balancín con foco, 12 V DC | 1 | 22 | 6 |
| PF19-3 | Portafusible europeo + fusibles 5 A 5 × 20 | 1 | ~20 | 6 |
| — | Tiras de pines macho ×2 y hembra ×1, borneras de 2 pines ×4, cinchos, cinta de aislar, desarmador de precisión | — | ~125 | 2 |

**Comprado también:** cautín 40 W, soldadura 60/40 con flux, base y multímetro (Home Depot, 15 oct).

**Sigue faltando para la Parte 1:** el cable de los motores. El kit "PH 2.0 Connector Pre-Crimped Cable Kit" (Amazon, B08T89ZK2Q) trae varios conectores hembra de 6P y 90 cables con terminal; alcanza para los dos motores.

## Ya tienes (no se compran)

Revisado el 15 oct 2026 entrando por SSH a las dos Pi.

| Pieza | Detalle | Parte |
| --- | --- | --- |
| Arduino Uno R3 + cable USB | Conectado por USB a la Pi 3, aparece como `/dev/ttyACM0` | 1 |
| Raspberry Pi 3 Model B+ (1 GB) | Hostname `bb8`. Debian 13 (trixie) de 64 bits, arranca de una **memoria USB de 8 GB** con 3.8 GB libres | 3 |
| Raspberry Pi Zero W (la original) | Hostname `bb8-head`. **No es Zero 2 W**: armv6, un núcleo, 512 MB. Raspbian 13 (trixie) en microSD de 16 GB. I2C apagado todavía | 5 |

**Lo que cambia por ser Zero W y no Zero 2 W:** Piper (la voz) no tiene versión armv6.
La cabeza habla con `espeak-ng` (voz robótica) por la salida de audio del ReSpeaker, o la
voz de Piper sale por la bocina del cuerpo (Pi 3 → PAM8403). La cámara, el ojo, el ToF y
el micrófono funcionan igual: `bb8-head` es Go y no usa OpenCV. `instalar_zero.sh` ya está
hecho para la Zero W. Si quieres la voz de Piper en la cabeza, la Zero 2 W
está en el carrito como opcional.

## Falta comprar: cotización del 9 oct 2026

Actualizada el 15 oct 2026 con lo decidido al probar el simulador: ReSpeaker 2-Mic **en
la cabeza** en lugar del micrófono USB, cámara gran angular, red WiFi propia del robot y más
almacenamiento para la Pi 3.

Precios en MXN tomados de las fichas de cada tienda el 9 de octubre de 2026, sin envío;
cambian seguido, así que confirma antes de pagar. "Est." es un estimado sin ficha
consultada (tienda física o sin precio visible). Nada de esto se ha comprado.

### Falta para la Parte 1 (detectado al armar, 15 oct 2026)

| Pieza | Cant. | Tienda | MXN | Nota |
| --- | --- | --- | --- | --- |
| Cable JST PH 2.0 mm de 6 pines a Dupont hembra, 20 cm | 2 | Mercado Libre / UNIT | 50–80 | El motor trae el conector (M1, GND, C2, C1, VCC, M2) pero no el cable |
| ~~Cautín 60 W con control de temperatura + estaño con flux + base~~ | 1 | Mercado Libre / Amazon | 250–350 | El MPU6050 y el VL53L0X vienen con los pines sueltos; sin soldar el escáner I2C no los ve. **Comprado (Home Depot, 15 oct): cautín 40 W, soldadura 60/40 con flux, base** |
| ~~Multímetro básico~~ | 1 | Mercado Libre / Steren | ~200 | Para ajustar el LM2596 a 5.1 V antes de conectar el servo. **Comprado (Home Depot, 15 oct)** |

### Cableado y fijación del robot final (revisión del 18 oct 2026)

La protoboard solo sirve para las pruebas de la Parte 1: no cabe en el disco y los jumpers se aflojan al rodar. En el robot, 5 V y GND se reparten en un protoshield sobre el Uno y las borneras. Esta lista sale de recorrer, pieza por pieza, cómo se conecta y se sujeta todo.

| Pieza | Cant. | Dónde | MXN | Para qué |
| --- | --- | --- | --- | --- |
| Protoshield para Arduino Uno (sin protoboard mini) | 1 | UNIT / Electrozone | ~60–90 (est.) | Se monta sobre el Uno y reparte 5 V y GND; ahí se sueldan encoders, I2C y servo |
| Cable USB A a B corto, 15–30 cm | 1 | Steren / Electrozone | ~50 (est.) | Pi ↔ Arduino dentro de la esfera; el de pruebas es largo |
| Jumpers macho-hembra de 30–40 cm (JUMPER-EXT30 o EXT40, Electrozone) | 6 | Electrozone | ~15 | Extensión del servo, que queda en lo alto del poste, hasta el Arduino |
| Separadores M3 de 8 mm | 12 | Electrozone / ferretería | ~50 (est.) | Pi, Uno y L298N sobre la melamina; los del pedido son de 10 y de 5 mm |
| Tornillos M3 × 20 mm con tuerca de nylon | 24 | Ferretería | ~30 (est.) | Desde abajo de la melamina hasta cada separador (12) y para las cuñas y los soportes (12); los M3 × 6 del pedido solo cierran la placa por arriba |
| Bridas de 10–15 cm con 10 bases adhesivas | 30 + 10 | Ferretería / Steren | ~30 (est.) | Agrupar y sujetar los cables a la melamina, con una brida antes de cada conector |
| Velcro de 2 cm | 1 m | Ferretería | ~20 (est.) | Cerrar la LiPo en su charola |

| Cable 18 AWG rojo y negro | 1 m de cada uno | Electrozone / Steren | ~40 (est.) | La potencia de 12 V hacia el L298N y los motores; el de 22 AWG del pedido se queda para señales y los bucks |
| Jumpers hembra-hembra de 15–20 cm (JUMPER-H20, Electrozone) | 10 más | Electrozone | ~25 | Hacen falta unos 15 hacia el protoshield; tienes 10 |

Ya los tienes, y alcanzan: borneras de 2 pines, cable 22 AWG, termorretráctil, tiras de pines macho, tornillos M3 × 6 con tuerca, XT60, portafusible e interruptor.

#### Lista de cables del robot final

Largos medidos sobre la vista superior de la base (artifact de diagramas, sección 3), con margen.

| Cable | De → a | Calibre | Largo | Conexión |
| --- | --- | --- | --- | --- |
| Batería | LiPo → fusible → interruptor → bornera de 12 V | 12 AWG (cables XT60) y 18 AWG | 15–20 cm | XT60 en la LiPo; soldado al fusible y al interruptor; atornillado en la bornera |
| Potencia del puente H | bornera de 12 V → L298N +12V y GND | 18 AWG | 10 cm | borne de tornillo |
| Bucks | bornera de 12 V → XL4015 IN y LM2596 IN | 22 AWG | 10–15 cm | soldado al buck |
| Pi | XL4015 OUT → micro USB de la Pi | el cable USB cortado | 25 cm | ya está hecho |
| Motores | L298N OUT1·2 y OUT3·4 → M1·M2 de cada motor | cables del kit PH 2.0, empalmados a 22 AWG | 15 cm | PH 2.0 en el motor; borne de tornillo en el L298N |
| Encoders | VCC, GND, C1 y C2 de cada motor → protoshield | cables del kit PH 2.0 | 15–20 cm | PH 2.0 en el motor; soldados al protoshield |
| Control del puente H | ENA, IN1, IN2, IN3, IN4 y ENB → D5, D7, D8, D11, D12 y D6 | jumpers hembra-hembra | 15 cm | pines macho en el L298N y en el protoshield |
| GND común | GND del L298N → GND del protoshield | 22 AWG | 10 cm | borne de tornillo y soldado |
| Servo | señal → D9; 5 V y GND → salida del LM2596 | jumpers macho-hembra de 30–40 cm | 30–40 cm | conector del servo; la alimentación no pasa por el Arduino |
| IMU | MPU6050 VCC, GND, SDA, SCL e INT → protoshield (5 V, GND, A4, A5, A2) | jumpers hembra-hembra | 20 cm | el jumper entra en los pines que ya tiene el sensor y en una tira de pines macho soldada al protoshield; al sensor no se le suelda nada |
| ToF (Partes 1–4) | VL53L0X VIN, GND, SCL y SDA → protoshield | jumpers hembra-hembra | 20 cm | pasa a la cabeza en la Parte 5 |
| Pi ↔ Arduino | USB de la Pi → USB-B del Uno | USB A–B corto | 15–30 cm | también lleva el GND común |
| Divisor de batería (Parte 6) | 12 V → 10 kΩ → A0 → 4.7 kΩ → GND | 22 AWG | 10 cm | soldado en el protoshield |

Los cables de 12 V van separados de los de señal (I2C y encoders), sujetos con bridas cada 8–10 cm y con una brida antes de cada conector.

### Carrito recomendado, ahora (Partes 2, 4 y 5)

Se puede pedir ya; nada depende del tamaño final de la esfera.

| Pieza | Cant. | Tienda | MXN | Nota |
| --- | --- | --- | --- | --- |
| Llantas RC 1:10 de 65 × 26 mm con hexágono de 12 mm (paquete de 4) + acoplamiento hexagonal de latón WEFEINO (4 pzs, opción **6 mm**: barreno 6 mm, hexágono 12 mm, 18 mm de largo, prisionero y rosca M4) | 1 + 1 | Amazon | llantas + 253 | La llanta con accesorios de UNIT se agotó. Las RC miden lo mismo que el CAD (65 × 26 mm): nada cambia en `cad/` ni en `config.py`. El acoplamiento entra en el eje D de 6 mm con su prisionero y un tornillo M4 con rondana detiene la llanta. Sobran 2 llantas de repuesto. Respaldo sin hexágono: [brida de 6 mm](https://www.amazon.com.mx/Acoplamiento-Aluminio-Cojinete-Accesorio-Acoplador/dp/B0DB4WXRR9) atornillada a cualquier llanta de 65 mm |
| [Soporte en L para motor 37 mm](https://articulo.mercadolibre.com.mx/MLM-652432552-soporte-motorreductor-bracket-37d-37gb-en-l-_JM) | 2 | Mercado Libre | ~40 c/u (est.) | Ya no es opcional: la llanta nueva no trae escuadra. Se atornilla abajo de la melamina en lugar del soporte impreso del CAD. Alternativa: [uxcell, 4 pzs](https://www.amazon.com.mx/uxcell-Soporte-reductor-engranajes-unidades/dp/B07QX82WS2) en Amazon |
| ~~Ball transfer 1″ de acero, montaje de 2 tornillos ([TruePower, 6 pzs](https://www.amazon.com.mx/TruePower-Roller-Ball-Transfer-Bearings/dp/B009KASQZW))~~ | 2 | Amazon | ~120 (est.) | Vienen en paquete; sobran para la cabeza o repuesto. **Reemplazado: 2 ruedas locas RL4 compradas en Electrozone (16 oct). Van sobre cuña de ~45°** |
| Melamina 12–15 mm 30 × 30 cm + varilla roscada 1/4″ de 30 cm con 4 tuercas y 4 rondanas | 1 | Maderería, ferretería | ~120 (est.) | La varilla es el poste: va de pie en el centro de la melamina, apretada con 2 tuercas arriba y 2 abajo. Así no hace falta imprimir el pie del poste |
| ReSpeaker 2-Mic Pi HAT **v2.0** (Seeed) | 1 | Amazon / Mercado Libre | ~450 (est.) | Reemplaza al micrófono USB: con dos micrófonos se calcula de dónde viene la voz (`listen`, ver [Simulador](Simulador.md)). **Va sobre el GPIO de la Zero W, en la cabeza** (ver [pines](#pines-de-la-zero-w-con-el-respeaker)): la cabeza detecta la voz y le manda solo ese audio al cuerpo. Trae su propia salida de audio (JST y jack), así que el MAX98357A ya no hace falta. Comprar la **v2.0**: la v1 depende del driver `seeed-voicecard`, que se rompe con kernels nuevos; confirmar soporte para el kernel de trixie antes de pagar. No cancela eco por hardware |
| Cámara OV5647 5 MP **gran angular** (120–160°) | 1 | Amazon / Mercado Libre | ~200–300 (est.) | Confirmado: no la tienes. En el simulador, con la lente normal (54°) y a 35 cm del piso, una pared o un marco de puerta llena la foto; con 90–120° se ve el cuarto. Confirmar que traiga cable **15 → 22 pines** (la Zero W usa el de 22). Respaldo: [Arducam OV5647 normal](https://www.amazon.com.mx/Arducam-megap%C3%ADxeles-Sensor-OV5647-Raspberry/dp/B012V1HEP4), 134, trae los dos cables |
| Adaptador WiFi USB 2.4 GHz con modo AP (chip MediaTek, el MT7612U es lo más seguro) | 1 | Amazon / Mercado Libre | ~150–400 (est.) | Hace la red propia del robot (`bb8-net`): la Pi 3 es el punto de acceso y la cabeza se conecta ahí, aunque el robot salga del alcance del router. Opcional si el punto de acceso va en la WiFi integrada de la Pi 3, que es más frágil. Antes de comprar, confirmar en la ficha que soporte modo AP en Linux |
| Memoria USB 3.0 o microSD A1 de 32 GB | 1 | Steren / Amazon | ~120 (est.) | La Pi 3 arranca de una USB de 8 GB con 3.8 GB libres; la voz (Whisper, wake word, Piper) y el venv no caben con holgura |
| Raspberry Pi Zero 2 W (opcional) | 1 | UNIT / Amazon | ~400 (est.) | Solo si quieres la voz saliendo de la cabeza (Piper no corre en la Zero W) y caras más rápidas. Mismo tamaño y cable de cámara |
| [Power bank 1 × 18650, 2600 mAh](https://www.mercadolibre.com.mx/cargador-tipo-power-bank-para-bateria-18650-2600mah-incluida/up/MLMU3701037772) | 1 | Mercado Libre | ~250 (est.) | 97 × 26 mm, 63 g, salida 5 V 1 A, batería incluida. Para la cabeza |
| Imanes neodimio disco 20 × 5 mm, [paquete de 10](https://listado.mercadolibre.com.mx/imanes-de-neodimio-20x5) | 1 | Mercado Libre | ~607 | Hacen falta 8 (4 en el poste, 4 en la cabeza). Revisar que la ficha diga N42; 2 paquetes de 5 salen en ~568. **Para la prueba se compraron 4 imanes 20 × 2 mm (IMAN-ND20x2, Electrozone): dos apilados = 20 × 4. Si aguantan, comprar 12 más ahí** |
| ~~[Rueda loca mini, bola de nylon 1″](https://uelectronics.com/producto/mini-rueda-universal-con-bola-de-nylon/)~~ | 3 | UNIT | 37–51 c/u | Las 3 "ruedecitas" de la cabeza; el nylon no raya la pintura del casco. **Comprado: 3 RL7 en Electrozone (16 oct)** |

**Subtotal ahora: ~$2,465** sin la Zero 2 W opcional (~$2,315 si el punto de acceso va en la
WiFi integrada). Bajó $85 al quitar el MAX98357A.

### Carrito recomendado, después (Parte 6)

Conviene esperar a que la Parte 3 ruede, por si cambia el tamaño de la esfera o el
consumo real de los motores.

| Pieza | Cant. | Tienda | MXN | Nota |
| --- | --- | --- | --- | --- |
| [LiPo 3S 11.1 V 5200 mAh 80C XT60, Hilldow (2 pzs)](https://www.amazon.com.mx/Hilldow-conector-veh%C3%ADculos-Camiones-paquetes/dp/B0FW4WPFNT) | 1 paquete | Amazon | 813 | ~407 por batería: más barato que una sola de [PCEONAMP](https://www.amazon.com.mx/PCEONAMP-capacidad-Quadcopter-Helic%C3%B3ptero-almacenamiento/dp/B0CHJHHK9V) (710). La segunda es repuesto |
| [Cargador balanceador iMAX B6AC 80 W](https://www.mercadolibre.com.mx/cargador-balanceador-imax-b6-b6ac-80w/p/MLM47578491) | 1 | Mercado Libre | 600–750 | La versión **AC** trae fuente interna; la B6 sin AC necesita una fuente de 12 V aparte. [HetPro](https://hetpro-store.com/cargador-de-baterias-lipo-80w/) la tiene en 850 |
| ~~[Switch de balancín con piloto 12 Vcc, 16 A](https://www.steren.com.mx/switch-de-balancin-de-1-polo-1-tiro-2-posiciones-on-off-con-piloto-12-vcc.html)~~ | 1 | Steren | 19 | . **Comprado: SH12RO en Electrozone (16 oct)** |
| ~~[Portafusible europeo de cartucho 5 × 20](https://www.steren.com.mx/porta-fusible-europeo-de-cartucho.html) + 3 fusibles 5 A 5 × 20~~ | 1 | Steren | 15 + ~12 | . **Comprado: PF19-3 + fusibles 5 A en Electrozone (16 oct)** |
| ~~Buck XL4015 5 A ajustable, para la Pi 3 ([Mercado Libre](https://listado.mercadolibre.com.mx/xl4015))~~ | 1 | Mercado Libre / UNIT | ~60 | La Pi 3 pide 5.1 V 2.5 A y el LM2596 da ~2 A reales: el XL4015 deja margen. El LM2596 del pedido 377466 se queda solo para el servo. [Versión con medidores en UNIT](https://uelectronics.com/producto/modulo-de-carga-reductor-xl4015-5a-con-medidores/): 140. **Comprado: AR-CARGA12 en Electrozone (16 oct), con cable USB–micro USB 699-927. Se usa ya, por el bajo voltaje de la Pi** |
| Cable micro-USB corto y grueso (o pigtail micro-USB macho) | 1 | Steren | ~50 (est.) | Del buck a la Pi 3. Un cable delgado provoca avisos de bajo voltaje; no alimentar por los pines GPIO |
| P-MOSFET AO3401 (SOT-23) | 2 | Mercado Libre | ~80 | Reposo profundo de la Pi; el IRF520 no sirve. Necesita placa adaptadora SOT-23 o soldar fino |
| ~~Resistencias 100 kΩ, 10 kΩ y 4.7 kΩ~~ | — | UNIT / Steren | ~30 (est.) | Pull-up del MOSFET y divisor de batería en A0. **Comprado: 330 Ω ×5, 4.7 kΩ ×2, 10 kΩ ×2, 100 kΩ ×2 en Electrozone (16 oct)** |
| XT60 hembra de panel + extensión JST-XH 4 pines | 1 | Amazon / UNIT | ~140 (est.) | Puerto de carga en la tapa del casco |
| Impresión 3D por servicio (gajos de la esfera y cabeza) | 1 | Servicio local | ~2,000–3,000 (est.) | No hay impresora: se manda a imprimir con los STL de `cad/stl` (~1.6 kg de PLA). Los soportes de motor se compran y el poste es varilla roscada |
| Primer, pintura blanca/naranja/plata, laca, lijas | 1 | Tlapalería | 300–500 (est.) | |
| Lastre (pesas o plomo) | 200–400 g | Mercado Libre | ~50 (est.) | |

**Subtotal Parte 6: ~$4,700** con la impresión por servicio. Total pendiente con los dos carritos: **~$7,165** más envíos.

### Cámara OV5647

Confirmado el 9 oct 2026: no la tienes y va en otro pedido, así que está en el carrito
de ahora. El 15 oct se cambió a gran angular después de verla en el simulador
(`lanzar_dummy.py --fov 54` contra `--fov 90`). Las lentes de 120–160° deforman las
orillas (ojo de pez), cosa que el simulador no imita. La
[de visión nocturna de UNIT](https://uelectronics.com/producto/camara-vision-nocturna-raspberry-pi-5mp-1080p-ov5647/)
cuesta 240 pero solo trae cable de 15 pines; con la Zero haría falta además un
[cable 15 → 22](https://www.amazon.com.mx/Onyehn-flexible-c%C3%A1mara-Raspberry-unidades/dp/B07XZ5DX5H).

### Pines de la Zero W con el ReSpeaker

El ReSpeaker 2-Mic HAT v2.0 cubre los 40 pines de la Zero W. Revisado contra lo que va en
la cabeza (datos del HAT de la [wiki de Seeed](https://wiki.seeedstudio.com/respeaker_2_mics_pi_hat_raspberry_v2/);
confirmar con `aplay -l` y `i2cdetect -y 1` al armar):

| Pin (BCM / físico) | Lo usa | Notas |
| --- | --- | --- |
| GPIO2 / 3, GPIO3 / 5 (I2C1) | Códec TLV320AIC3104 del HAT (0x18) **y** VL53L0X (0x29) | Sin choque: direcciones distintas en el mismo bus. El ToF se conecta al puerto Grove I2C del HAT |
| GPIO18 / 12, GPIO19 / 35, GPIO20 / 38, GPIO21 / 40 (I2S) | Micrófonos y salida de audio del HAT | Por eso el ojo ya no puede ir en GPIO18 y el MAX98357A sobra |
| GPIO10 / 19 (MOSI), GPIO11 / 23 (SCLK) (SPI0) | LEDs APA102 del HAT | Por eso el anillo WS2812 no va en GPIO10 con el HAT |
| GPIO17 / 11 | Botón del HAT | Libre para usarlo (p. ej. "despertar") |
| GPIO12 / 32 (PWM0) | **Datos del ojo WS2812**, por el puerto Grove digital del HAT | PWM libre: el audio del HAT es I2S. `dtparam=audio=off` |
| GPIO13 / 33 | Puerto Grove digital del HAT (segundo hilo) | Libre |
| CSI | Cámara OV5647 | Sin choque |

**El ojo: WS2812 en GPIO12 (PWM0), sin comprar nada.** Los LEDs APA102 del HAT ocupan el
SPI0 (GPIO10 y GPIO11) y el I2S ocupa GPIO18, así que el anillo WS2812 que ya tienes va en
**GPIO12**. Ese pin sale en el **puerto Grove digital del HAT**, así que no hace falta un
header apilable ni soldar bajo el HAT. GPIO12 es el canal 0 del PWM (función ALT0) y el
PWM está libre: el HAT saca el audio por I2S, no por el PWM analógico (el instalador pone
`dtparam=audio=off` para que el driver de audio integrado no lo tome).

Cableado del anillo al puerto Grove digital (cable Grove de 4 hilos):

| Hilo Grove | Pin del HAT | Al anillo WS2812 |
| --- | --- | --- |
| Amarillo (señal 1) | **GPIO12** | DIN (entrada de datos) |
| Blanco (señal 2) | GPIO13 | — (sin usar) |
| Rojo | VCC | **No lo uses** si el puerto da 3.3 V: alimenta el anillo con 5 V del power bank |
| Negro | GND | GND (común con el power bank) |

- VCC del anillo: 5 V directo del power bank (a 0.3 de brillo pide ~0.3 A; a blanco pleno ~1 A).
- Pon una resistencia de 330 Ω en serie en DIN y un capacitor de 470–1000 µF entre 5 V y GND del anillo.
- La señal de la Zero es de 3.3 V. Con el anillo a 5 V casi siempre basta; si parpadea o da colores raros, pon un 74AHCT125 (o un diodo 1N4148 en la VCC del primer LED) para subir el nivel.

En software, `bb8-head` no usa cgo, así que el PWM con DMA lo hace un ayudante en C, `bb8-eye`:
- Usa [rpi_ws281x](https://github.com/jgarff/rpi_ws281x); su tabla de pines del PWM0 incluye GPIO12 con ALT0, y la Zero W viene en su lista de modelos.
- Se compila en la Zero al instalar.
- Corre como root por conexión desde `bb8-eye.socket`, y `bb8-head` le manda los cuadros por `/run/bb8-eye.sock`.

Es el ojo por defecto con micrófono (`--eye ws2812-pwm`). Otras opciones:
- `--eye apa102 --eye-leds 3`: solo los 3 LEDs del HAT.
- `--eye ws2812`: el anillo en SPI0 (GPIO10), solo **sin** el HAT.

**Audio.** La salida del HAT (JST de bocina o jack → PAM8403 → bocina de 4 Ω) reemplaza al
MAX98357A. El PAM8403 va a 5 V del power bank de la cabeza.

### Lo que ya no hace falta

- **Bocina o DAC USB para la Pi principal:** ya llegaron 2 bocinas y un PAM8403, y la
  Pi 3 trae jack de 3.5 mm: jack → PAM8403 → bocina.
- **Tarjeta de sonido USB y amplificador MAX98357A para la Zero:** el ReSpeaker trae su
  propio códec con salida de bocina (JST) y jack de 3.5 mm. El MAX98357A además chocaría:
  usa el mismo I2S (GPIO18, 19 y 21) que el HAT.
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
