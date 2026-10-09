# Wake word "oye BB-8"

El agente de voz despierta con **"hey Jarvis"**, un modelo preentrenado de openWakeWord.
Esta página explica cómo entrenar el tuyo para **"oye BB-8"** y cómo lo usa el robot.

Código: [`partes/4-agente-movimiento/wakeword`](https://github.com/erickdsama/bb8/tree/main/partes/4-agente-movimiento/wakeword).

| Archivo | Hace |
| --- | --- |
| `entrenar_oye_bb8.ipynb` | Cuaderno para Google Colab: corre todo y descarga el modelo |
| `entrenar.py` | Los cuatro pasos: `datos`, `muestras`, `aumentar`, `modelo` (o `todo`) |
| `oye_bb8.yml` | Configuración: frases, voces, cantidades, pasos de entrenamiento |
| `muestras.py` | Genera "oye BB-8" y frases parecidas con las voces de Piper en español |
| `datos.py` | Baja eco de cuartos, ruido de fondo y los negativos genéricos |
| `lanzar_train.py` | Corre `openwakeword/train.py` con parches para torch y torchaudio nuevos |
| `probar.py` | Mide un modelo con archivos, con horas de ruido o en vivo con el micrófono |
| `requirements-entrenar.txt` | Dependencias del entrenamiento (no van en la Pi) |

## Cómo se entrena

Es la receta automática de openWakeWord con un cambio: openWakeWord hace sus muestras
con una voz inglesa que no sabe decir "oye", así que aquí las hacemos en español.

1. **Datos.** Respuestas al impulso de cuartos reales (MIT), ruido de AudioSet, ~2000 h
   de audio genérico ya convertido a features (ACAV100M) como negativos y ~11 h para
   medir falsas activaciones. Todo sale de Hugging Face; son unos 20 GB.
2. **Muestras.** Todas las voces de Piper en español (España, México, Argentina…) dicen
   "oye bi bi eit" en varias escrituras, con velocidad, entonación y tono al azar.
   Además dicen frases parecidas que **no** deben despertarlo: "oye", "oye bebé", "hoy
   veo", "oye vivi", "bi bi eit" a secas, "hey jarvis"… 20 000 de cada una para
   entrenar y 2 000 para validar. Se recortan las orillas y las pausas largas.
3. **Aumento.** A cada muestra le pone eco y ruido de fondo y la convierte en features
   de openWakeWord.
4. **Modelo.** Entrena la red pequeña de openWakeWord (50 000 pasos) buscando menos de
   0.2 falsas activaciones por hora, y la exporta a `oye_bb8.onnx` (~300 KB).

## Paso a paso en Colab

1. Abre [`entrenar_oye_bb8.ipynb`](https://github.com/erickdsama/bb8/blob/main/partes/4-agente-movimiento/wakeword/entrenar_oye_bb8.ipynb)
   en Colab (Archivo → Abrir cuaderno → GitHub) y elige un entorno con GPU T4.
2. Opcional pero recomendado: graba 20–50 veces "oye BB-8" con tu voz y súbelas en un
   `.zip`. Si puedes, con el micrófono del robot:
   `arecord -r 16000 -c 1 -f S16_LE -d 2 oye_01.wav`. Van a las positivas, repetidas
   con distinto ruido y eco, y el 20 % se aparta para medir.
3. Corre las celdas en orden y descarga `oye_bb8.onnx` al final.

Cada paso es reanudable: si Colab se corta, vuelve a correr la celda y sigue donde iba
(mientras no se borre la máquina). Las muestras son lo más lento sin GPU.

## En una PC con Linux

```bash
cd partes/4-agente-movimiento/wakeword
python3.12 -m venv .venv-ww && source .venv-ww/bin/activate
pip install -r requirements-entrenar.txt
pip install --no-deps "openwakeword @ git+https://github.com/dscripka/openWakeWord@368c03716d1e92591906a84949bc477f3a834455"
python entrenar.py --trabajo ~/ww todo --mis-grabaciones ~/oye_bb8_wavs
python entrenar.py --trabajo ~/ww todo --rapido     # solo para ver que todo corre (modelo inútil)
```

Necesita unos 30 GB libres en la carpeta de trabajo. No lo corras en la Pi.

## Ponerlo en el robot

```bash
scp oye_bb8.onnx bb8@<ip-de-la-pi>:/home/bb8/bb8/partes/4-agente-movimiento/voz/modelos/
sudo systemctl restart bb8-voz
journalctl -u bb8-voz | grep "Wake word"     # debe decir: Wake word: oye_bb8
```

Si `BB8_WAKEWORD` no está definida, el agente usa `voz/modelos/oye_bb8.onnx` cuando
existe y "hey Jarvis" cuando no. Si `BB8_WAKEWORD` apunta a un `.onnx` que no existe,
avisa en el log y vuelve a "hey Jarvis" en lugar de no arrancar. `voz/modelos/` no se
sube a git.

## Ajustar el umbral

```bash
python probar.py oye_bb8.onnx --mic                       # en vivo: di "oye BB-8" y otras cosas
python probar.py oye_bb8.onnx mis_grabaciones/            # cuántas detecta
python probar.py oye_bb8.onnx --ruido grabacion_sala.wav  # falsas activaciones por hora
```

Con eso elige `BB8_WAKEWORD_UMBRAL` en `/etc/bb8.env` (0.5 por defecto): bájalo si no te
oye, súbelo si se despierta solo. Si se dispara con una frase en particular, agrégala a
`negativas` en `oye_bb8.yml` y vuelve a entrenar; si no te entiende, agrega más
grabaciones tuyas.

## Estado

El pipeline se corrió completo el 2026-10-09 (Python 3.12, torch 2.14, CPU) con una
voz en español y datos de relleno, y exporta un `.onnx` que el agente carga. El modelo
de verdad no se ha entrenado todavía: Hugging Face, de donde salen las voces de Piper y
los datos, no es accesible desde las sesiones de Claude en la nube. Hay que correr el
cuaderno en Colab.
