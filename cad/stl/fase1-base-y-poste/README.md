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
| `base_soporte_bt` | **2** | 40 %, sin soportes (los canales de tuerca libran a 47°) | Las dos ruedas locas RL4 (adelante y atrás, es la misma pieza) | 32 g c/u |
| `base_panel_interruptor` | 1 | 20 % | Rocker de encendido y LED de carga | 11 g |
| `poste_pie_poste` | 1 | 30 % | Pie del poste, atornillado al centro de la plataforma | 21 g |
| `poste_soporte_servo` | 1 | 30 % | Sostiene el servo (MG995 o MG996R) arriba del poste | 37 g |
| `poste_portaimanes` | 1 | **50 %** | Los 4 imanes N42 que jalan la cabeza; flojo se rompe | 41 g |

**No se imprime:** la plataforma (MDF de 5.5 mm, ya cortada con
[`cad/plantillas/plataforma.dxf`](../../plantillas)) y el poste: tubo PVC conduit de 1″ (29.6 mm por fuera),
cortado a ~160 mm **del tramo liso** (el casquillo de la punta mide ~32.5 mm por fuera y no entra: córtalo y tíralo), con una muesca de 10 × 10 mm abajo para el cable del servo. Tornillería M3/M4 y separadores en [Materiales](../../../wiki/Materiales.md).

## Mide antes de mandar

Estas medidas salen de hojas de vendedor, no de tus piezas. Si alguna no coincide, cambia
el número en `cad/parametros.scad` (o `base.scad`) y regenera con `bash cad/exportar.sh stl`.

| Qué medir | En el diseño | Afecta a |
| --- | --- | --- |
| Interruptor SH12RO: largo × alto del cuerpo (lo que entra en el hueco, sin la ceja) | 19 × 13 mm (`rocker_corte`) | `base_panel_interruptor` |
| ~~MG995: base a orejas y entre agujeros~~ | 26.5 mm; 47.8 × 10.2 mm (plano del MG995, coincide con lo medido) | `poste_soporte_servo` |
| Cuerno redondo del servo: distancia del centro a sus agujeros de afuera (Ø20 ya medido) | 7 mm (`cuerno_r_torn`) | `poste_portaimanes` |
| Llanta montada con su cople: de la cara de la caja de engranes al centro de la llanta | 20 mm (`rueda_offset`) | Entre la llanta y el soporte quedan 3.5 mm: si tu cople es más corto, rozan |

## Holguras revisadas (24 oct)

- Nada choca en el ensamble; cada pieza solo apoya contra la plataforma.
- A la pared de la esfera: charola 5 mm, soportes de motor y de ruedas locas ≥ 23 mm.
  El portaimanes queda a 2.5 mm a propósito (fuerza de los imanes); se ajusta cortando el tubo.
- Huecos de tuerca a 6.7 mm (M3) y 8.4 mm (M4) entre puntas, para que entren sin forzar.
- Plataforma ya cortada: un tornillo del panel cae en la orilla del pasacables (usa rondana)
  y el agujero del Uno junto a la rueda loca de atrás se encima: el Uno va con 3 tornillos.

## Arreglos de la primera impresión (29 oct)

La primera tanda se imprimió con estos defectos. Todos se arreglan sobre la pieza, sin reimprimir:

| Pieza | Defecto | Arreglo |
| --- | --- | --- |
| `base_charola` | El hueco de la LiPo se comía las pestañas: de cada pared solo quedaban tiras de 1 mm | Corta las tiras y cuelga la charola con 2 escuadras de 20 × 20 mm por dentro de las paredes altas (M3 × 12). El STL de ahora ya trae pestañas con chaflán de 45° |
| `base_soporte_motor_izq` | Tapa los 4 agujeros del L298N | Taladra 3 mm usando el MDF de guía; el L298N va con M3 × 16 que se roscan en el soporte, con 2 tuercas de separador |
| `base_soporte_bt` (adelante) | Tapa la orilla de un agujero de la Raspberry Pi | Taladra 1.7 mm y rosca ahí el M2 × 16 de la Pi. El STL de ahora ya trae ese agujero |
| `base_soporte_bt` | La cara medía 26 mm de ancho y la base de la RL4 30; los agujeros estaban a 35 mm y la brida los trae a 38 | **Reimprime las 2 copias.** El STL de ahora sale de la RL4 medida a mano: cara de 54 × 36 mm, ranuras de 35 a 41 mm entre agujeros para M4 × 12; la tuerca entra deslizando por la punta del soporte; la cara 5 mm más afuera porque la bola sale 20 mm y no 25 |
| `poste_soporte_servo` | El cable del servo no tiene salida hacia el tubo | Sácalo por la ventana del costado y bájalo por fuera del tubo con cinchos |

Cómo se arma el poste: [`cad/png/soporte_servo_armado.png`](../../png/soporte_servo_armado.png).
Guía 3D paso a paso: <https://claude.ai/artifact/6XfezBdSw2956qa97WJ6cf>.

## Tornillería

Un kit surtido M2–M5 de 1230 piezas (Mercado Libre, ~$244) cubre toda la fase:

| Tornillo | Cuántos | Dónde |
| --- | --- | --- |
| M3 × 16 | 21 | Soportes de motor (8), de ruedas locas (5: en el de adelante, el de la esquina junto a la Pi no va) y pie del poste (4) al MDF; L298N (4) |
| M3 × 12 | 13 | Panel (2), escuadras (4), orejas del servo (4), Arduino Uno (3) |
| M3 × 8 | 8 | Motor a la cara de su soporte (la cabeza se hunde ~1.2 mm en el avellanado) |
| M3 × 6 | 3 | Arduino Uno a sus separadores |
| M4 × 12 | 4 | Ruedas locas: un M4 × 16 topa al fondo de la ranura |
| M4 × 16 | 4 | Imanes, con tuerca y fijador de roscas |
| M2 × 16 | 4 | Raspberry Pi, con los separadores de latón de 5 mm como espaciador; el de la esquina sobre el soporte de la rueda loca se rosca en él |
| M2 × 12 | 4 | Portaimanes al disco del servo |

Rondana del lado del MDF y apriete a mano: el MDF se aplasta.
