# Wake word "oye BB-8"

Entrena el modelo de openWakeWord para que el BB-8 despierte con "oye BB-8" en lugar de
"hey Jarvis". Se corre en Google Colab o en una PC con Linux, no en la Pi.

```
entrenar_oye_bb8.ipynb      cuaderno para Colab: corre todo y descarga oye_bb8.onnx
entrenar.py                 pasos: datos, muestras, aumentar, modelo (o todo)
oye_bb8.yml                 frases, voces, cantidades y parámetros de entrenamiento
muestras.py                 "oye BB-8" y frases parecidas con las voces de Piper en español
datos.py                    eco, ruido y negativos genéricos de Hugging Face
lanzar_train.py             openwakeword/train.py con parches para torch/torchaudio nuevos
probar.py                   mide un modelo: archivos, horas de ruido o micrófono en vivo
sin_generador/              sustituto de piper-sample-generator (train.py lo importa siempre)
requirements-entrenar.txt
```

```bash
pip install -r requirements-entrenar.txt
pip install --no-deps "openwakeword @ git+https://github.com/dscripka/openWakeWord@368c03716d1e92591906a84949bc477f3a834455"
python entrenar.py --trabajo ~/ww todo [--mis-grabaciones carpeta/]
python probar.py ~/ww/oye_bb8.onnx --mic
```

Luego copia `oye_bb8.onnx` a `partes/4-agente-movimiento/voz/modelos/` en la Pi y
reinicia `bb8-voz`: el agente lo usa solo.

Guía: [wiki/Wake-word-oye-BB-8.md](../../../wiki/Wake-word-oye-BB-8.md)
