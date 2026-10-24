# STL por fase

Tres fases de impresión, en el orden en que se arma Bebocho. Cada carpeta se manda
completa al servicio de impresión y trae su README con piezas, copias, material y peso.
Detalle de cada pieza en la wiki: [Diseño 3D](../../wiki/Diseno-3D.md#piezas-a-imprimir).

| Fase | Carpeta | Qué es | Piezas a imprimir | Peso máx. |
| --- | --- | --- | --- | --- |
| 1 | [`fase1-base-y-poste`](fase1-base-y-poste) | La base que rueda (soportes de motor, charola, ruedas locas, panel) y el poste con el servo y los imanes | 9 | ~0.6 kg |
| 2 | [`fase2-esfera-y-cabeza`](fase2-esfera-y-cabeza) | La esfera (14 gajos + 6 círculos) y la cabeza | 32 | ~2.1 kg |
| 3 | [`fase3-base-de-carga`](fase3-base-de-carga) | El plato de carga inalámbrica | 6 | ~0.55 kg |

El peso es de la pieza maciza en PLA: es un tope; con relleno pesa menos (menos la
esfera, que es casi maciza). Se regeneran con `bash cad/exportar.sh stl`.
