"""Corre `openwakeword/train.py` con un parche: torchaudio ≥ 2.9 quitó `torchaudio.info` y
solo lee audio con torchcodec (y FFmpeg). Aquí `load` e `info` usan soundfile, que ya trae
libsndfile. Además exporta el ONNX con el exportador clásico (opset 13, el que espera
openWakeWord): desde torch 2.9 el nuevo es el de por defecto y sube a opset 18.
Y arregla que train.py siempre intente convertir a TFLite (sus banderas tienen
default="False", un texto que cuenta como verdadero); el agente usa el .onnx.
Con versiones viejas los parches no estorban.

    python lanzar_train.py --training_config oye_bb8.yml --augment_clips
"""
import argparse
import runpy

import soundfile as sf
import torch
import torchaudio


def _cargar(ruta, *args, **kwargs):
    datos, tasa = sf.read(str(ruta), dtype="float32", always_2d=True)
    return torch.from_numpy(datos.T.copy()), tasa


class _Info:
    def __init__(self, ruta):
        i = sf.info(str(ruta))
        self.sample_rate, self.num_frames, self.num_channels = i.samplerate, i.frames, i.channels
        self.bits_per_sample = 16
        self.encoding = "PCM_S"


_exportar = torch.onnx.export


def _exportar_clasico(*args, **kwargs):
    kwargs.setdefault("dynamo", False)
    try:
        return _exportar(*args, **kwargs)
    except TypeError:            # torch < 2.5 no conoce `dynamo`
        kwargs.pop("dynamo")
        return _exportar(*args, **kwargs)


_parse_args = argparse.ArgumentParser.parse_args


def _parse_args_bien(self, *args, **kwargs):
    ns = _parse_args(self, *args, **kwargs)
    for k, v in vars(ns).items():
        if v == "False":
            setattr(ns, k, False)
    return ns


argparse.ArgumentParser.parse_args = _parse_args_bien
torchaudio.load = _cargar
torchaudio.info = _Info
torch.onnx.export = _exportar_clasico
runpy.run_module("openwakeword.train", run_name="__main__", alter_sys=True)
