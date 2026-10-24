# Parte 4 · Agente controla el movimiento

**Entregable:** le dices "Oye Jarvis… ven acá" o "da una vuelta" y la base lo hace;
log de la conversación con las llamadas a herramientas.

El cerebro vive en `bb8-body` (Go). En la Pi también corre, en Python, el lado del
micrófono: wake word, transcripción e identificación de voz.

| Código | Hace |
| --- | --- |
| [`go/internal/tools`](https://github.com/erickdsama/bb8/tree/main/go/internal/tools), [`body/robot_tools.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/body/robot_tools.go) | Un solo registro de herramientas: `move`, `turn`, `stop`, `look_at`, `take_photo`, `set_eye_color`, `say`, `express`, `listen`, `get_map`, `set_position`, `update_map`, `get_pose`, `get_setting`, `set_setting`, `people`, `remember_about`, `forget_about` y las de los conectores (`weather`, `web_search`, `calendar_*` y las de servidores MCP externos, `<nombre>_<herramienta>`) |
| [`go/internal/mcpserver`](https://github.com/erickdsama/bb8/tree/main/go/internal/mcpserver) | Servidor MCP en `:8765/mcp` (token desde la red) |
| [`go/internal/agent`](https://github.com/erickdsama/bb8/tree/main/go/internal/agent) | Agente propio: cualquier modelo de OpenRouter con herramientas; `POST /agent/ask` |
| [`go/internal/persona`](https://github.com/erickdsama/bb8/blob/main/go/internal/persona/persona.go) | Personalidad y reglas: el system prompt para Claude y para el agente |
| [`go/internal/connectors`](https://github.com/erickdsama/bb8/tree/main/go/internal/connectors) | Clima (Open-Meteo) y calendario (Google), como módulos |
| `voz/escucha.py` | openWakeWord + Silero VAD (ONNX, sin torch) |
| `voz/stt.py` | faster-whisper o API en la nube |
| `voz/hablantes.py` | Registro e identificación de voces (Resemblyzer, opcional) |
| `voz/agente.py` | Manda lo transcrito a `bb8-body` (`POST /agent/ask`) con quién habló |
| `voz/salida.py` | Piper local, o el altavoz de la cabeza a través de `bb8-body`; pitidos |
| `voz/__main__.py` | El bucle; sus frases fijas (buenas noches, sin señal…) salen de `head.personalidad.frase()` |
| `voz/cara.py` | Gira la cabeza hacia la voz (Parte 5) |
| `wakeword/` | Entrenamiento de la wake word "oye BB-8" (Colab o PC, no en la Pi) |

## Paso 1: MCP desde Claude Code o Desktop

Con la [Parte 3](Parte-3-Base-andando.md) funcionando (`bb8-body` corriendo), el MCP ya
está en `http://bb8.local:8765/mcp`. Conéctalo como en
[Conectar con Claude](Conectar-con-Claude.md) y prueba con el robot en el piso:
*"avanza medio metro y gira a la derecha"*. Antes de tener el robot, todo esto funciona
igual con el [simulador](Simulador.md).

## Paso 2: el agente propio

Las páginas no viven en el robot: las sirve `bb8-web` desde tu PC y el robot se conecta
hacia ella. Instálalo una vez en la PC:

```bash
make && cp bin/bb8-web ~/.local/bin/
cp sistema/bb8-web.service ~/.config/systemd/user/ && systemctl --user enable --now bb8-web
sudo loginctl enable-linger $USER          # que siga corriendo sin sesión abierta
cat ~/.bb8-web/password                    # tu contraseña de la página
scp ~/.bb8-web/device-token bb8.local:/tmp/ # luego en la Pi: /etc/bb8/web-token y BB8_WEB en /etc/bb8.env
```

En la Pi, `/etc/bb8.env` lleva `BB8_WEB=ws://debian.local:8780/robot` y
`BB8_WEB_TOKEN_FILE=/etc/bb8/web-token` (ver [Instalación en la Pi](Instalacion-en-la-Pi.md#variables-de-entorno)).

En la página de configuración (`http://debian.local:8780/`, con tu contraseña) pon `agent.api_key` (tu
clave de [OpenRouter](https://openrouter.ai)) y, si quieres, otro `agent.model`. Pruébalo
con el chat de la misma página: *"da una vuelta y dime qué ves"*.

BB-8 también contesta preguntas generales. Para lo que cambia (noticias, precios, horarios)
usa `web_search`, que busca con el plugin web de OpenRouter y la misma clave (~$0.02 por
búsqueda; `connectors.web.model` puede ser un modelo más barato). El clima (`weather`)
viene encendido y solo necesita `home.city`.

**Servicios y cuentas.** En la página, *Services → Connect a service*:

- **Zapier** (Gmail, Google Calendar, Sheets…), **Notion**, **Linear**: eliges de quién es la
  cuenta, presionas *Sign in with…*, entras y aceptas. En Zapier luego eliges qué puede hacer
  BB-8 (p. ej. *Gmail: Send Email*).
- **Home Assistant**: BB-8 lo busca en la red; entras con tu usuario de Home Assistant. Antes,
  agrega allí la integración *Model Context Protocol Server* y expón lo que BB-8 puede controlar.
- **Alexa**: a través de Home Assistant, con su integración *Alexa Devices*.
- **Agregar dispositivos sin abrir Home Assistant**: con Home Assistant conectado, la tarjeta
  *Apps and smart home* muestra *Casa · N dispositivos* y **+ Agregar**. Eliges la marca (Smart
  Life, Alexa, SmartThings, LG ThinQ, Hue, TP-Link, TV Samsung, Google Cast) o lo que Home
  Assistant encontró en tu red, y la página te muestra cada paso: un campo, un QR para escanear
  con la app de la marca o un botón para iniciar sesión. El robot hace la configuración en Home
  Assistant con su token (la página nunca lo ve) y le da a BB-8 los dispositivos nuevos al
  terminar y cada 5 minutos (luces, contactos, clima, ventiladores, persianas, TV, aspiradoras;
  nunca cerraduras ni alarmas). Un link de Home Assistant (`…/api/mcp`) con token queda como
  *Home Assistant* y sus herramientas se llaman `home_…`.
- **Google Home**: el [servidor MCP oficial de Google](https://developers.home.google.com/mcp/home)
  (`https://home.googleapis.com/mcp`): luces, enchufes, termostatos y cámaras. Usa el mismo
  cliente de Google que el calendario (activa *Home API* en el proyecto) y es de la casa
  (compartido). Está en acceso anticipado y pide una suscripción **Google Home Premium Advanced**.
- **Other**: cualquier servicio que dé un link para asistentes de IA (con token si lo pide).
- **Google Calendar** (en la tarjeta *Calendar*): Google no deja que las apps se registren
  solas: se crea una vez un proyecto gratis en Google Cloud (API de Calendar, *Publish app* para
  que no caduque a los 7 días, cliente *Web application* con `https://bb8.zerock.name/oauth/` como dirección de
  regreso). Su Client ID y secret van en `google_client.env` (ignorado por git; ver
  `google_client.env.example`) y `make` los mete en `bb8-body`. Después cada persona presiona
  *Connect Google* y entra con su cuenta (o con varias). Al preguntarle junta todos tus calendarios;
  para agendar usa el principal de tu primera cuenta, o el que nombres ("en el de trabajo").
- **Avisos del calendario** (tarjeta *Calendar* → *Notices*): BB-8 dice en voz alta cuando alguien
  más te agenda, te mueve o te cancela algo ("Erick, Ana te agendó «Revisión» mañana a las 10:00"),
  y te recuerda los eventos 10 minutos antes. Revisa todos los calendarios de todas tus cuentas
  (puedes conectar varias, p. ej. la personal y la del trabajo) y dice cada cosa una sola vez aunque
  aparezca en dos. Con *Also the ones I add myself* también avisa de lo que agregas tú. Solo las
  cuentas con dueño; de 22:00 a 8:00 se guarda los avisos y los dice al terminar. Con `connectors.calendar.push_url` (la dirección pública de
  bb8-web, p. ej. `https://bb8.zerock.name`) Google avisa al instante a `/hooks/google` y bb8-web
  se lo pasa al robot por su enlace; sin ella, el robot revisa cada 2 minutos.
- **Termostato Nest** (tarjeta *Nest thermostat*): por la API de Google
  [Device Access](https://console.nest.google.com/device-access) (pago único de US$5). Ahí creas un
  proyecto con el mismo Client ID de Google y pegas su *Project ID* en *Settings*; luego
  *Connect Google* te lleva a la página de Nest para elegir el termostato. "¿A cuánto está la
  casa?", "pon el clima en 23". Cualquier voz conocida lo cambia (9–32 °C, quita el eco); una
  desconocida solo pregunta.

Cada cuenta tiene dueño: cuando Ana habla, BB-8 usa la cuenta de Ana; cuando hablas tú, la
tuya; las compartidas (la casa) las usa cualquiera conocido. Antes de mandar, crear o borrar
algo, BB-8 pregunta "¿lo hago?" y solo lo hace si dices que sí en tu siguiente frase. Una voz
desconocida no usa cuentas personales.

Para el login, Zapier, Notion, Linear y Google regresan a `https://bb8.zerock.name/oauth/`
(bb8-web), que se lo pasa al robot por su enlace. Es la dirección de regreso que se registra en
Google (`connectors.mcp.oauth_bridge`; vacío = la dirección de la página + `/oauth/`).

**Personas.** En `/people` asignas el rol de cada quien: `owner`, `family` (lo puede mover)
o `guest` (solo platica). Una voz registrada entra como `family` la primera vez que se oye.
Si le cuentas de alguien ("Ana es alérgica a las nueces"), BB-8 la guarda como invitada con
esa nota y la recuerda cuando habla con ella. La pestaña **Activity** de `/control` muestra
en vivo lo que oye, lo que contesta, las herramientas que usa y las alertas del robot.

## Paso 3: micrófono y altavoz

Micrófono USB (o ReSpeaker 2-Mic HAT) en la Pi principal (una Pi 3); la voz sale por su
jack de 3.5 mm → PAM8403 → bocina. `sistema/instalar_pi.sh` instala el venv, las
dependencias de voz y la voz de Piper `es_ES-davefx-medium` (a mano, de
`huggingface.co/rhasspy/piper-voices`, archivos `.onnx` y `.onnx.json`, en
`partes/4-agente-movimiento/voz/modelos/`).

## Paso 4: probar por partes

```bash
python -m voz.agente "da una vuelta y dime qué ves"   # texto, sin micrófono
python -m voz.hablantes registrar Erick               # 4 frases de ~3 s; necesita resemblyzer
python -m voz.hablantes lista
python -m voz                                         # el bucle completo
python -m voz --sin-voz-registrada                    # obedece a cualquiera
```

Ciclo: wake word → graba hasta 0.8 s de silencio → Whisper → ¿quién habla? →
`bb8-body` (`/agent/ask`: modelo de OpenRouter con las herramientas) → Piper.

## Ajustes

- **Wake word:** "hey Jarvis" de fábrica. Para "oye BB-8", entrena tu modelo con el cuaderno de `wakeword/` en Colab y copia `oye_bb8.onnx` a `voz/modelos/`; el agente lo usa solo. Ver [Wake word "oye BB-8"](Wake-word-oye-BB-8.md).
- **Modelo:** `agent.model` en la página de configuración (por defecto `anthropic/claude-sonnet-5.5`). `agent.max_steps` limita las llamadas a herramientas por orden (8) y `agent.reset_after_s` olvida la conversación tras 5 min de silencio.
- **Voces desconocidas:** pueden platicar, pero `move`, `turn` y `look_at` se bloquean en el código de `bb8-body` (`speaker_known: false`), no solo en el prompt. Mientras no registres ninguna voz, obedece a todos. No es biometría segura.
- **Ajustes por voz:** BB-8 puede cambiar algunos ajustes cuando se lo piden ("habla más bajo", "vivo en Monterrey") con `set_setting`. Solo los marcados para el agente; los límites de movimiento no. El cambio aparece al momento en la página de configuración.
- **Transcripción:** en la Pi 3, Whisper `base` local tarda ~4–8 s por frase (estimado). Con `BB8_STT_API_KEY` (Groq) se transcribe en la nube en ~1 s. Comparación y memoria en [Instalación en la Pi](Instalacion-en-la-Pi.md#transcripción-local-o-en-la-nube).
- **Identificación de voz:** `resemblyzer` instala torch (~1 GB de disco, ~350 MB de RAM). En la Pi 3 solo cabe con la transcripción en la nube; descoméntalo en `requirements-voz.txt` si lo quieres.
- **Dormir:** "BB-8, a dormir" pasa al reposo (profundo si está habilitado, Parte 6).

Todas las variables en [Instalación en la Pi](Instalacion-en-la-Pi.md#variables-de-entorno).
Para que arranque solo: servicios `bb8-body` y `bb8-voz`.

## Lista cuando

- [ ] Claude Code mueve la base por MCP
- [ ] El chat de la página de configuración responde y usa herramientas
- [ ] `python -m voz.agente "..."` responde
- [ ] Tu voz registrada; una voz desconocida no logra moverlo
- [ ] "Oye Jarvis… ven acá" y se mueve
- [ ] (Opcional) "Oye BB-8… ven acá" con tu modelo entrenado
