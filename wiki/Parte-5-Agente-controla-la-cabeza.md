# Parte 5 · Agente controla la cabeza

**Entregable:** la cabeza (todavía sin carcasa) gira hacia quien habla, la cámara
manda una foto al modelo y BB-8 describe lo que ve; el ojo NeoPixel cambia de color
según el estado.

En la Pi Zero W corre `bb8-head`, un binario de Go
([`go/cmd/bb8-head`](https://github.com/erickdsama/bb8/tree/main/go/cmd/bb8-head)). No abre
puertos: se conecta al cuerpo por TLS mutuo y le responde fotos, ojo, voz y reposo, y le
empuja el ToF, la dirección de la voz y el audio de cada frase (el micrófono va en la
cabeza).

| Código | Hace |
| --- | --- |
| [`go/internal/head/head.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/head/head.go) | Atiende al cuerpo por el enlace (`photo`, `eye`, `say`, `rest`, `config`) y empuja `tof`, `sound_dir` y `state` |
| [`head/real.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/head/real.go) | Hardware: `rpicam-jpeg`, anillo NeoPixel por SPI (periph.io), `aplay` + `espeak-ng`. **Sin probar en hardware todavía** |
| [`head/vl53l0x.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/head/vl53l0x.go) | Driver del VL53L0X por I2C |
| [`head/sounds.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/head/sounds.go) | Pitidos de droide sintetizados: 15 emociones y balbuceo |
| [`head/mic.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/head/mic.go) | ReSpeaker 2-Mic HAT: captura con `arecord`, VAD por energía, audio de cada frase al cuerpo y dirección al final |
| [`head/doa.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/head/doa.go) | Dirección de llegada con GCC-PHAT entre las dos cápsulas |
| [`head/sim.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/head/sim.go) | `--sim`: reenvía a la cabeza simulada del dummy |
| [`go/internal/link`](https://github.com/erickdsama/bb8/tree/main/go/internal/link) | Enlace TLS 1.3 mutuo y `bb8-body pair` |
| `partes/5-cabeza/head/` | Cabeza anterior en Python. `sounds.py` y `personalidad.py` los siguen usando `voz` y el simulador |

## Hardware

Pi Zero W **original** (armv6, 512 MB), con su propio power bank 18650. Lleva:

- **ReSpeaker 2-Mic Pi HAT v2.0** sobre los 40 pines: los dos micrófonos (a la izquierda y
  a la derecha de la cabeza), el códec de audio con salida de bocina y 3 LEDs APA102;
- cámara OV5647 por CSI (cable de 15 a 22 pines: el conector de la Zero es más chico);
- el ojo: el anillo WS2812 de 16 LED con los datos en **GPIO12 (PWM0)**, por el **puerto
  Grove digital del HAT**;
- salida de audio del HAT → PAM8403 + altavoz de 4 Ω;
- si se muda a la cabeza, el VL53L0X en el puerto Grove I2C del HAT (I2C1, 0x29; el códec es 0x18).

> **El cable de datos del ojo va a GPIO12 (PWM0), en el puerto Grove digital del HAT**
> (hilo amarillo), con 5 V y GND del power bank.
> - No va en GPIO18, porque es del I2S del HAT, ni en GPIO10, porque el SPI0 es de los LEDs APA102 del HAT.
> - El PWM está libre: el audio del HAT es I2S, y el instalador pone `dtparam=audio=off`.
>
> `bb8-head` es Go sin cgo, así que el PWM con DMA lo hace `bb8-eye`, un ayudante en C con
> [rpi_ws281x](https://github.com/jgarff/rpi_ws281x). Lo arranca systemd como root desde
> `bb8-eye.socket`, y `bb8-head` le manda los cuadros por `/run/bb8-eye.sock`.
>
> Opciones de `--eye`:
> - `ws2812-pwm`: el de por defecto con micrófono;
> - `apa102 --eye-leds 3`: solo los LEDs del HAT;
> - `ws2812`: el anillo en GPIO10 por SPI, solo sin el HAT.
>
> Cableado completo en [Materiales](Materiales.md#pines-de-la-zero-w-con-el-respeaker).

**El micrófono.**
- La cabeza escucha todo el tiempo, pero solo manda audio mientras alguien habla: VAD por energía con umbral que se adapta al ruido, 300 ms de pre-roll y 600 ms de silencio para cerrar la frase.
- Al terminar cada frase calcula de dónde vino (GCC-PHAT) **respecto a hacia dónde mira la cabeza**. El cuerpo le suma el ángulo del poste, así que `listen` da los candidatos respecto al cuerpo.
- Con dos micrófonos no se distingue adelante de atrás de la cabeza: por eso `look_at` hacia un candidato y una foto lo resuelven.
- Mientras la cabeza habla, el micrófono se silencia.

Instala el driver del HAT (overlay `respeaker-2mic-v2_0` de
[seeed-linux-dtoverlays](https://github.com/Seeed-Studio/seeed-linux-dtoverlays)); la tarjeta
aparece como `seeed2micvoicec`. Flags de `bb8-head`:
- `--mic` (ALSA, por defecto `plughw:CARD=seeed2micvoicec`; vacío = sin micrófono);
- `--mic-spacing` (distancia entre cápsulas: mídela, por defecto 0.058 m);
- `--mic-swap`: si al aplaudir a la derecha `listen` da un ángulo negativo, la tarjeta entrega los canales al revés.

- Sin la cámara (va en otro pedido) la cabeza funciona igual: ojo, voz y ToF responden, `take_photo` dice que no hay cámara y Claude avanza en tramos cortos. Pruébalo en el PC con `python simulador/lanzar_dummy.py --camara ninguna`.
- La Zero no trae salida de audio: la da el códec del HAT (bocina en el JST, o jack → PAM8403). El MAX98357A ya no hace falta y chocaría con el I2S del HAT.
- Piper no corre en armv6: la cabeza habla con `espeak-ng` (voz robótica, va bien con un droide). Para voz de Piper usa `BB8_VOZ_SALIDA=local` y la bocina de la Pi 3.
- El anillo a blanco pleno pide ~1 A: aliméntalo del power bank. El brillo se limita a 0.3 en el código.
- Alimenta el PAM8403 a 5 V, nunca a 11.1 V.

El casco de la cabeza, el plato interior y los soportes del ojo (anillo + cámara) y de
la bocina están en el [Diseño 3D](Diseno-3D.md#cabeza). En esa cabeza el VL53L0X va en
el ojo chico, conectado a la Zero.

## Instalar en la Zero

Raspberry Pi OS Lite de 32 bits, hostname `bb8-head`. El binario se compila en el PC
(`make zero`) y los certificados salen de la Pi 3; los pasos están en
[Instalación en la Pi](Instalacion-en-la-Pi.md#pi-zero-w-cabeza):

```bash
sudo bash /tmp/instalar_zero.sh bb8.local
journalctl -fu bb8-head      # "link: connected to body"
```

## Probar la cabeza

La cabeza solo se alcanza a través del cuerpo. Desde la Pi 3:

```bash
curl localhost:8770/head                                  # conectada, cámara, rev de config
curl -o foto.jpg "localhost:8770/head/photo?width=640"
curl -X POST localhost:8770/tools/set_eye_color -d '{"color":"blue","pattern":"breathe"}'
curl -X POST localhost:8770/tools/say -d '{"text":"hola, soy BB-8"}'
curl -X POST localhost:8770/tools/say -d '{"sound":"happy"}'
curl -X POST localhost:8770/tools/express -d '{"emotion":"curious"}'
```

Detalle de cada mensaje del enlace en [Protocolo § 3](Protocolo.md#3-head-link-mutual-tls-13-port-8772).
El volumen (`head.volume`) y el brillo del ojo (`head.eye_brightness`) se cambian en la
página de configuración (o por voz: "habla más bajo") y le llegan a la cabeza al momento.

## Sonidos y personalidad

Los pitidos se generan con matemática pura (`go/internal/head/sounds.go`, port de
`head/sounds.py`): cada sonido es una serie de sílabas, cada una un barrido de
frecuencia con vibrato y un timbre (`soft`, `bright` o `buzz`). Cada vez que suena cambia
un poco el tono y el tempo, para que no parezca grabado. Un texto cualquiera da su propio
balbuceo: mismo texto, mismos pitidos, y la entonación sigue la puntuación (`¿…?` sube al
final, `¡…!` va más agudo y rápido). Es lo que suena antes de cada frase.

| Emoción | Suena como | Ojo | Cuándo |
| --- | --- | --- | --- |
| `happy` | chirridos que suben, saltarines | green | algo salió bien |
| `excited` | trino rápido que se dispara | yellow blink | algo nuevo y genial |
| `greeting` | bi-du-íiip | cyan | alguien llega o se va |
| `curious` | sube y baja, preguntándose | cyan breathe | algo raro o nuevo |
| `question` | ¿eh? | cyan | va a preguntar o no entendió |
| `thinking` | blips suaves sin prisa | orange breathe | mira o calcula |
| `yes` / `no` | dos blips que suben / dos notas gruñonas que bajan | green / orange | acepta / se niega |
| `alert` | tres pitidos secos | red blink | obstáculo, batería baja |
| `scared` | grito tembloroso y trino nervioso | purple blink | lo empujan, casi se cae |
| `sad` | dos notas largas que caen | blue breathe | algo salió mal |
| `error` | zumbido grave | red | falla una herramienta o la red |
| `yawn` | sube despacio y cae largo | blue breathe | a dormir |
| `wake` | barrido grave → agudo | white | al arrancar |
| `laugh` | ji-ji-ji que baja | yellow | un chiste |

La tabla emoción → color y patrón vive en
[`go/internal/persona`](https://github.com/erickdsama/bb8/blob/main/go/internal/persona/persona.go):
la herramienta `express` hace pitido y ojo en una llamada, y la personalidad del system
prompt sale de ahí. Para agregar una emoción: su fila en `persona.Emotions` y sus sílabas
en `Sounds` de `sounds.go` (con el mismo nombre; una prueba lo exige).

Escuchar en el PC, sin el robot (versión Python, nombres en español):

```bash
python -m head.sounds                        # todos, con su descripción
python -m head.sounds curioso risa           # algunos
python -m head.sounds --texto "¿Quién anda ahí?"
python -m head.sounds --html sonidos.html    # página con todos para el navegador
```

## Conectarla al resto

En la Pi principal, en `/etc/bb8.env`:

```
BB8_MIC=cabeza              # el agente de voz oye por el micrófono de la cabeza (por defecto)
BB8_VOZ_SALIDA=cabeza       # la voz sale por la cabeza (a través de bb8-body)
BB8_BUSCAR_CARA=1           # girar la cabeza hacia la voz
```

`voz` (wake word y Whisper en la Pi 3) recibe el audio de la cabeza por
`GET /audio/stream` de `bb8-body`. Entre frases no llega nada por la WiFi; `voz` rellena con
silencio. `BB8_MIC=local` vuelve a un micrófono USB en la Pi 3.

Si el VL53L0X se muda a la cabeza, pon `USAR_TOF_LOCAL 0` en el firmware y súbelo de
nuevo. `bb8-head` lo detecta solo en el I2C y empuja la distancia cada 100 ms, y
`bb8-body` se la manda al Arduino con `D <mm>`.

## Mecánica

- Modifica el MG996R del poste a rotación continua solo si la cabeza va a girar 360°; con ±90° basta el servo normal.
- La cabeza sigue al poste por imanes, no tiene cable. Prueba la unión magnética sobre una superficie curva (una pelota de playa sirve).

## Lista cuando

- [ ] `curl localhost:8770/head` dice `connected: true` con cámara y micrófono
- [ ] Aplaudir a la derecha de la cabeza: `listen` da un ángulo positivo
- [ ] `take_photo` desde Claude devuelve lo que ve la cabeza
- [ ] El ojo cambia de color (datos en GPIO12, puerto Grove del HAT) y la voz sale por la cabeza
- [ ] Gira la cabeza hacia ti cuando le hablas y describe lo que ve
