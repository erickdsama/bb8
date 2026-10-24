# Fase 1 · Base y poste del servo

Lo primero que se imprime: con esto la base ya rueda en el piso (Partes 2 y 3) y el
poste mueve la cabeza con el servo (Parte 5), todavía sin esfera.

**Material:** PETG (aguanta mejor el calor de los motores y no se quiebra con los
imanes). Boquilla 0.4, capa 0.2 mm. Todas caben en una cama de 220 × 220 mm.

| STL | Copias | Relleno | Para qué | Peso máx. |
| --- | --- | --- | --- | --- |
| `base_soporte_motor` | 1 | 30 %, 4 perímetros | Motor derecho, colgado bajo la plataforma | 40 g |
| `base_soporte_motor_izq` | 1 | 30 %, 4 perímetros | Motor izquierdo (espejo) | 40 g |
| `base_charola` | 1 | 30 %, soportes bajo el piso | Carga la LiPo y el lastre; abajo lleva el portabobina de la carga | 306 g |
| `base_soporte_bt` | **2** | 40 %, soportes en el hueco | Las dos ruedas locas RL4 (adelante y atrás, es la misma pieza) | 32 g c/u |
| `base_panel_interruptor` | 1 | 20 % | Rocker de encendido y LED de carga | 11 g |
| `poste_pie_poste` | 1 | 30 % | Pie del poste, atornillado al centro de la plataforma | 21 g |
| `poste_soporte_servo` | 1 | 30 % | Sostiene el MG996R arriba del poste | 37 g |
| `poste_portaimanes` | 1 | **50 %** | Los 4 imanes N42 que jalan la cabeza; flojo se rompe | 41 g |

**No se imprime:** la plataforma es MDF de 6 mm (o dos de 3 mm pegadas), cortada con
[`cad/plantillas/plataforma.dxf`](../../plantillas). Tornillería M3 y separadores en
[Materiales](../../../wiki/Materiales.md).

Antes de mandarlas, imprime la plantilla en papel y presenta encima los soportes de motor
y las tarjetas.
