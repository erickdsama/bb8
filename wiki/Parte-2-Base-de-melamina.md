# Parte 2 · Base de melamina

**Entregable:** un disco de melamina de 20–22 cm con motores, ruedas, apoyos y
electrónica fijos, con el hueco de la batería listo. Se levanta con una mano y nada
se mueve.

No hay código nuevo. La carpeta [`partes/2-base-melamina`](https://github.com/erickdsama/bb8/tree/main/partes/2-base-melamina)
solo tiene esta lista.

## Materiales

| Pieza | Nota |
| --- | --- |
| Melamina 12–15 mm, 30 × 30 cm | 3 mm de MDF se pandea con 1.5 kg encima |
| 2 ruedas de goma 65 mm con **cople hexagonal para eje D de 6 mm** | La "Llanta de Goma 65 mm" de UNIT **con accesorios** sí entra (buje hex 12 mm); la de motor TT es otra. Forrarlas con cinta de silicona. Ver [Materiales](Materiales.md) |
| 2 soportes de motor JGB37 | |
| 2 ball transfers de 1″, bola metálica | Resbalan mejor sobre el casco que una rueda loca |
| Tubo PVC 25 mm × 30 cm o varilla roscada 1/4″ + 4 tuercas | Poste central |
| Separadores M3 de latón, tornillos M3, cinta doble cara, velcro, bridas | Los separadores y tornillos vienen en el pedido UNIT |

Ninguno, salvo la tornillería, está todavía en el [pedido](Materiales.md).

## Pasos

1. Corta el disco de Ø 20–22 cm; marca el centro y los ejes en + y ×.
2. Haz dos muescas opuestas para las ruedas y atornilla los soportes de motor de modo que las ruedas asomen por ellas.
3. Monta los 2 ball transfers a 90° de las ruedas, **a la misma altura de contacto** que las ruedas. Si tocan antes, la esfera no rodará.
4. Deja el hueco de la LiPo (139 × 47 × 25 mm) centrado **abajo**: hace de lastre. Arduino, L298N y buck van arriba, sobre separadores M3.
5. Fija el poste central (PVC o varilla roscada) con el portaimanes arriba. Se recorta en la Parte 6.
6. Recablea lo de la [Parte 1](Parte-1-Protoboard.md) con cables cortos y termorretráctil; mantén un solo riel de GND.
7. Pesa el conjunto y anota el centro de gravedad: tiene que quedar lo más abajo posible.

## Comprobar

Repite dos pruebas de la Parte 1 después de montar: un cable flojo aparece ahí.

- `p2_motores_encoders`: cada rueda en los dos sentidos y su encoder cuenta.
- `p4_mpu6050`: la IMU quedó bien orientada (chip arriba) y el rumbo sube al girar a la derecha.

Mira también el artefacto [Mecánica](Artefactos.md#mecánica) para ver cómo se acomoda
todo dentro de la esfera.

## Lista cuando

- [ ] La base se levanta con una mano y nada se mueve ni cuelga
- [ ] Las ruedas y los ball transfers tocan una superficie plana al mismo tiempo
- [ ] `p2` y `p4` pasan otra vez
- [ ] Peso y centro de gravedad anotados
