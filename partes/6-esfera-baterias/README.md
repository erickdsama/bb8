# Parte 6 · Esfera y baterías

El gestor de energía de la Pi principal: reposo ligero y profundo, y batería baja.

```
energia/__main__.py     máquina de estados activo → escuchando → ligero → profundo, API :8771
```

```bash
python -m energia --prueba    # tiempos cortos y sin apagar la Pi
python -m energia             # el profundo solo con BB8_REPOSO_PROFUNDO=1
```

Hardware de esta parte: LiPo 3S con fusible e interruptor, segundo buck para el
servo, Arduino alimentado por VIN, divisor de batería en A0 (`HAY_DIVISOR_BAT 1`) y
P-MOSFET de lado alto en A3 para cortar la Pi. La esfera impresa y la cabeza con
imanes también se arman aquí.

**Lista cuando** rueda sin cable, se duerme solo y despierta al moverlo.

Guía completa: [wiki/Parte-6-Esfera-y-baterias.md](../../wiki/Parte-6-Esfera-y-baterias.md)
