# Fase 1 · Base y poste del servo

Lo primero que se imprime: con esto la base ya rueda en el piso (Partes 2 y 3) y el
poste mueve la cabeza con el servo (Parte 5), todavía sin esfera.

**Material:** PETG (aguanta mejor el calor de los motores y no se quiebra con los
imanes). Boquilla 0.4, capa 0.2 mm. Todas caben en una cama de 220 × 220 mm.

| STL | Copias | Relleno | Para qué | Peso máx. |
| --- | --- | --- | --- | --- |
| `base_soporte_motor` | 1 | 30 %, 4 perímetros | Motor derecho, colgado bajo la plataforma | 40 g |
| `base_soporte_motor_izq` | 1 | 30 %, 4 perímetros | Motor izquierdo (espejo) | 40 g |
| `base_charola` | 1 | 30 %, soportes bajo el piso | Carga la LiPo y el lastre; abajo lleva el portabobina de la carga | 289 g |
| `base_soporte_bt` | **2** | 40 %, soportes en el hueco | Las dos ruedas locas RL4 (adelante y atrás, es la misma pieza) | 32 g c/u |
| `base_panel_interruptor` | 1 | 20 % | Rocker de encendido y LED de carga | 11 g |
| `poste_pie_poste` | 1 | 30 % | Pie del poste, atornillado al centro de la plataforma | 21 g |
| `poste_soporte_servo` | 1 | 30 % | Sostiene el MG996R arriba del poste | 37 g |
| `poste_portaimanes` | 1 | **50 %** | Los 4 imanes N42 que jalan la cabeza; flojo se rompe | 41 g |

**No se imprime:** la plataforma (MDF de 5.5 mm, ya cortada con
[`cad/plantillas/plataforma.dxf`](../../plantillas)) y el poste: tubo PVC conduit de 25 mm,
cortado a ~160 mm. Tornillería M3/M4 y separadores en [Materiales](../../../wiki/Materiales.md).

## Mide antes de mandar

Estas medidas salen de hojas de vendedor, no de tus piezas. Si alguna no coincide, cambia
el número en `cad/parametros.scad` (o `base.scad`) y regenera con `bash cad/exportar.sh stl`.

| Qué medir | En el diseño | Afecta a |
| --- | --- | --- |
| Interruptor SH12RO: largo × alto del cuerpo (lo que entra en el hueco, sin la ceja) | 19 × 13 mm (`rocker_corte`) | `base_panel_interruptor` |
| MG996R: de la base a la cara de abajo de las orejas, y entre agujeros de las orejas | 26.5 mm; 49.5 × 10 mm | `poste_soporte_servo` |
| Cuerno redondo del servo: Ø y distancia del centro a sus agujeros | Ø25 mm; 8 mm | `poste_portaimanes` |
| Tubo PVC: diámetro exterior | 25 mm (`tubo_d`) | `poste_pie_poste`, `poste_soporte_servo` |
| Llanta montada con su cople: de la cara de la caja de engranes al centro de la llanta | 20 mm (`rueda_offset`) | Entre la llanta y el soporte quedan 3.5 mm: si tu cople es más corto, rozan |

## Holguras revisadas (24 oct)

- Nada choca en el ensamble; cada pieza solo apoya contra la plataforma.
- A la pared de la esfera: charola 5 mm, soportes de motor y de ruedas locas ≥ 23 mm.
  El portaimanes queda a 2.5 mm a propósito (fuerza de los imanes); se ajusta cortando el tubo.
- Huecos de tuerca a 6.7 mm (M3) y 8.4 mm (M4) entre puntas, para que entren sin forzar.
- Plataforma ya cortada: un tornillo del panel cae en la orilla del pasacables (usa rondana)
  y el agujero del Uno junto a la rueda loca de atrás se encima: el Uno va con 3 tornillos.
