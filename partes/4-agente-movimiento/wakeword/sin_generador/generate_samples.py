"""Sustituto de piper-sample-generator.

`openwakeword/train.py` importa `generate_samples` siempre, aunque no se use. Nuestras
muestras las hace muestras.py en español, así que aquí solo hay un aviso.
"""


def generate_samples(*args, **kwargs):
    raise RuntimeError("Las muestras se generan con `python entrenar.py muestras`, no con --generate_clips")
