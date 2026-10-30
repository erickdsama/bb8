# Parte 2 · Base de MDF

**Entregable:** la plataforma de MDF con motores, llantas, ruedas locas, la charola con
la LiPo, el poste del servo y la electrónica cableada. Se levanta con una mano y nada
se mueve.

No hay código nuevo. Guía 3D paso a paso, sobre el modelo real:
<https://claude.ai/artifact/6XfezBdSw2956qa97WJ6cf>.

## Materiales

| Pieza | Nota |
| --- | --- |
| Plataforma de MDF de 5.5 mm cortada en láser con [`plataforma.dxf`](https://github.com/erickdsama/bb8/blob/main/cad/plantillas/plataforma.dxf) | Repasa los agujeros con broca de 3.5 mm: el láser los deja justos |
| Piezas impresas de la [fase 1](https://github.com/erickdsama/bb8/tree/main/cad/stl/fase1-base-y-poste), en PETG | Soporte de rueda loca: **2 copias** |
| 2 llantas RC de 65 mm con cople hexagonal para eje D de 6 mm | |
| 2 ruedas locas RL4 | Van en los soportes inclinados |
| Tubo PVC conduit de 1″, ~160 mm del tramo liso | Muesca de 10 × 10 mm abajo |
| Kit de tornillos M2–M5 de 1230 piezas, separadores de latón, 2 escuadras de 20 × 20 mm, velcro de 20 mm, epóxico | Reparto de tornillos en el [README de la fase 1](https://github.com/erickdsama/bb8/blob/main/cad/stl/fase1-base-y-poste/README.md#tornillería) |
| LiPo 3S XT60 de máximo 46 × 138 × 40 mm | El hueco de la charola mide 48 × 140 mm |

Lo que falta comprar está en [Materiales](Materiales.md#falta-para-armar-la-fase-1-detectado-al-armar-29-oct-2026).

## Pasos

1. **Motores:** cada JGB37 en su soporte con 4 M3 × 8; el soporte bajo el MDF con 4 M3 × 16 y la tuerca en su hueco. Cople y llanta.
2. **Ruedas locas:** primero cada soporte bajo el MDF con M3 × 16 (el del centro entra por el agujero de la cara). En el de adelante no pongas el de la esquina junto a la Pi: su tuerca choca con el separador de la Pi, y el M2 × 16 de la Pi se rosca en el soporte. Luego mete las 2 tuercas M4 deslizando por las puntas del soporte y atornilla la RL4 con 2 M4 × 12 en las ranuras. En una mesa plana las ruedas locas quedan ~9 mm en el aire: es correcto, porque van inclinadas 48° y tocan la pared curva de la esfera, no el piso.
3. **Charola:** corta las tiras delgadas de arriba de las paredes y cuélgala con 2 escuadras por dentro. LiPo con 2 correas de velcro; lastre en los huecos de los lados.
4. **Poste:** pie al centro con 4 M3 × 16, tubo hasta el fondo, soporte del servo arriba. El cable del servo sale por la ventana del costado y baja por fuera del tubo con cinchos ([cómo se arma](https://github.com/erickdsama/bb8/blob/main/cad/png/soporte_servo_armado.png)).
5. **Tarjetas:** Uno sobre separadores de 10 mm; Pi con M2 × 16 y separadores de 5 mm; L298N con M3 × 16 y 2 tuercas de separador, roscado en el soporte del motor (taládralo antes con 3 mm). Ajusta los 2 bucks a 5.1 V antes de conectar nada.
6. **Cableado:** LiPo → fusible 5 A → rocker → placa de distribución (14 AWG); de ahí 18 AWG a L298N, VIN y bucks. Señales en 22 AWG con jumpers Dupont; una gota de silicón caliente en cada conector y los mazos amarrados a las bases adhesivas. Pines en la guía 3D.
7. Pesa el conjunto y anota el centro de gravedad: tiene que quedar lo más abajo posible.

## Comprobar

Repite dos pruebas de la Parte 1 después de montar: un cable flojo aparece ahí.

- `p2_motores_encoders`: cada rueda en los dos sentidos y su encoder cuenta.
- `p4_mpu6050`: la IMU quedó bien orientada (chip arriba) y el rumbo sube al girar a la derecha.

## Lista cuando

- [ ] La base se levanta con una mano y nada se mueve ni cuelga
- [ ] En la mesa, las ruedas apoyan y las ruedas locas quedan ~9 mm arriba, apuntando hacia el centro de la esfera
- [ ] `p2` y `p4` pasan otra vez
- [ ] Peso y centro de gravedad anotados
