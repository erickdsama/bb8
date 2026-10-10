# Parte 5 · Agente controla la cabeza

**Entregable:** la cabeza (todavía sin carcasa) gira hacia quien habla, la cámara
manda una foto al modelo y BB-8 describe lo que ve; el ojo NeoPixel cambia de color
según el estado.

Código: [`partes/5-cabeza`](https://github.com/erickdsama/bb8/tree/main/partes/5-cabeza).

| Archivo | Hace |
| --- | --- |
| `head/server.py` | API HTTP `:8080`: `/estado`, `/foto`, `/tof`, `/ojo`, `/hablar`, `/cara`, `/reposo` |
| `head/backends_real.py` | picamera2, VL53L0X o VL53L1X (se detecta solo), NeoPixel, Piper. **Sin probar en hardware todavía** |
| `head/backends_sim.py` | Webcam del PC o vista sintética, ToF simulado (lo usa el simulador) |
| `head/colors.py` | Colores con nombre y patrones del ojo |
| `head/sounds.py` | Pitidos de droide sintetizados: 15 emociones y balbuceo para cada frase |
| `head/personalidad.py` | Emoción → pitido + color de ojo, frases cortas y la sección de personalidad del system prompt |
| `requirements-zero.txt` | Dependencias de la Pi Zero |

## Hardware

Pi Zero 2 W con su propio power bank 18650, cámara OV5647 por CSI (cable 15 → 22 pines,
el conector de la Zero es más chico), anillo WS2812 de 16 LED en **GPIO18**, PAM8403 + altavoz 4 Ω y, si se muda
a la cabeza, el VL53L0X por I2C.

- La Zero no trae salida de audio: el PAM8403 necesita una tarjeta de sonido USB o PWM en GPIO13 con filtro RC (o un MAX98357 por I2S, que es lo que recomienda la [cotización](Materiales.md) y mueve la bocina sin el PAM8403).
- El anillo a blanco pleno pide ~1 A: aliméntalo del power bank. El brillo se limita a 0.3 en el código.
- Alimenta el PAM8403 a 5 V, nunca a 11.1 V.

El casco de la cabeza, el plato interior y los soportes del ojo (anillo + cámara) y de
la bocina están en el [Diseño 3D](Diseno-3D.md#cabeza). En esa cabeza el VL53L0X va en
el ojo chico, conectado a la Zero.

## Instalar en la Zero

Hostname `bb8-head`, I2C y cámara activados en `sudo raspi-config`:

```bash
sudo git clone https://github.com/erickdsama/bb8 /home/bb8/bb8
cd /home/bb8/bb8 && sudo bash sistema/instalar_zero.sh
curl http://bb8-head.local:8080/estado
```

A mano:

```bash
sudo apt install python3-picamera2 python3-opencv alsa-utils
python3 -m venv --system-site-packages .venv && source .venv/bin/activate
pip install -r partes/5-cabeza/requirements-zero.txt
pip install --no-deps -e .
sudo .venv/bin/python -m head.server          # root por el NeoPixel en GPIO18
```

## Probar la cabeza

```bash
curl -o foto.jpg "http://bb8-head.local:8080/foto?ancho=640"
curl http://bb8-head.local:8080/tof
curl -X POST bb8-head.local:8080/ojo -d '{"color":"azul","patron":"respirar","brillo":0.3}'
curl -X POST bb8-head.local:8080/hablar -d '{"texto":"hola, soy BB-8"}'
curl -X POST bb8-head.local:8080/hablar -d '{"sonido":"feliz"}'
curl -X POST bb8-head.local:8080/hablar -d '{"sonido":"curioso","texto":"¿qué es eso?"}'
```

Detalle de cada ruta en [Protocolo § 3](Protocolo.md#3-http-de-la-cabeza-pi-zero-2-w-puerto-8080).

## Sonidos y personalidad

Los pitidos se generan con matemática pura en `head/sounds.py` (sin archivos WAV ni
numpy): cada sonido es una serie de sílabas, cada una un barrido de frecuencia con
vibrato y un timbre (`suave`, `brillante` o `zumbido`). Cada vez que suena cambia un poco
el tono y el tempo, para que no parezca grabado. Un texto cualquiera da su propio
balbuceo: mismo texto, mismos pitidos, y la entonación sigue la puntuación (`¿…?` sube al
final, `¡…!` va más agudo y rápido). Es lo que suena antes de cada frase de Piper.

| Emoción | Suena como | Ojo | Cuándo |
| --- | --- | --- | --- |
| `feliz` | chirridos que suben, saltarines | verde | algo salió bien |
| `emocionado` | trino rápido que se dispara | amarillo parpadeo | algo nuevo y genial |
| `saludo` | bi-du-íiip | cian | alguien llega o se va |
| `curioso` | sube y baja, preguntándose | cian respirar | algo raro o nuevo |
| `pregunta` | ¿eh? | cian | va a preguntar o no entendió |
| `pensando` | blips suaves sin prisa | naranja respirar | mira o calcula |
| `si` / `no` | dos blips que suben / dos notas gruñonas que bajan | verde / naranja | acepta / se niega |
| `alerta` | tres pitidos secos | rojo parpadeo | obstáculo, batería baja |
| `asustado` | grito tembloroso y trino nervioso | morado parpadeo | lo empujan, casi se cae |
| `triste` | dos notas largas que caen | azul respirar | algo salió mal |
| `error` | zumbido grave | rojo | falla una herramienta o la red |
| `bostezo` | sube despacio y cae largo | azul respirar | a dormir |
| `despertar` | barrido grave → agudo | blanco | al arrancar |
| `risa` | ji-ji-ji que baja | amarillo | un chiste |

`head/personalidad.py` une cada emoción con su pitido y su color de ojo (la herramienta
MCP `express` hace las dos cosas en una llamada), guarda las frases fijas por situación
(`frase("buenas_noches")` elige una distinta a la anterior) y arma la sección de
personalidad que el servidor MCP agrega a sus instrucciones. Para agregar una emoción:
sus sílabas en `SONIDOS` de `sounds.py` y su fila en `EMOCIONES` de `personalidad.py`
(con el mismo nombre; un `assert` lo exige).

Escuchar en el PC, sin el robot:

```bash
python -m head.sounds                        # todos, con su descripción
python -m head.sounds curioso risa           # algunos
python -m head.sounds --texto "¿Quién anda ahí?"
python -m head.sounds --guardar sonidos/     # un WAV por sonido
python -m head.sounds --html sonidos.html    # página con todos para el navegador
```

## Conectarla al resto

En la Pi principal, en `/etc/bb8.env`:

```
BB8_HEAD_URL=http://bb8-head.local:8080
BB8_VOZ_SALIDA=cabeza       # la voz sale por la cabeza
BB8_BUSCAR_CARA=1           # girar hacia quien habla
```

Si el VL53L0X se muda a la cabeza, pon `USAR_TOF_LOCAL 0` en el firmware y súbelo de
nuevo: el servicio de movimiento leerá `/tof` cada 100 ms y se lo mandará al Arduino
con `D <mm>`.

## Mecánica

- Modifica el MG996R del poste a rotación continua solo si la cabeza va a girar 360°; con ±90° basta el servo normal.
- La cabeza sigue al poste por imanes, no tiene cable. Prueba la unión magnética sobre una superficie curva (una pelota de playa sirve).

## Lista cuando

- [ ] `/estado` muestra cámara y ToF
- [ ] `take_photo` desde Claude devuelve lo que ve la cabeza
- [ ] El ojo cambia de color y la voz sale por la cabeza
- [ ] Gira la cabeza hacia ti cuando le hablas y describe lo que ve
