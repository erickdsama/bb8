# Fase 3 · Base de carga

El plato de carga inalámbrica: Bebocho rueda encima, cae al hoyo del centro y carga
mirando hacia donde sea. Va al final, cuando la esfera ya está cerrada.

**Material:** PETG, 30 % de relleno. Cama mínima **210 × 210 mm** (el centro mide 200 mm).
Fuente: [`cad/plato.scad`](../../plato.scad).

| STL | Copias | Para qué | Peso máx. |
| --- | --- | --- | --- |
| `plato_centro` | 1 | El hoyo donde se asienta la esfera; la bobina emisora va pegada en su hueco desde abajo, bajo 1 mm de plástico. Imprímelo con el hoyo hacia arriba | 252 g |
| `plato_rampa` | **3** | Cuartos de la rampa que rodean el centro | 71 g c/u |
| `plato_rampa_cable` | 1 | El cuarto con la ranura del cable; va atrás, hacia la caja | 70 g |
| `plato_caja` | 1 | Tapa la placa del emisor y el jack de 24 V. 20 % de relleno | 18 g |

Al armar: pega las 4 rampas alrededor del centro, la hoja de hule antiderrapante debajo de
todo y Plasti Dip solo en la rampa. Lo electrónico (kit inalámbrico, eliminador 24 V,
cargador CC/CV, BMS) está en [Materiales](../../../wiki/Materiales.md) y la conexión en la
[Parte 6](../../../wiki/Parte-6-Esfera-y-baterias.md#alimentación-con-la-lipo).
