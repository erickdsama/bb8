# STL por parte

Una carpeta por parte del proyecto, en el orden en que se arma. Cada carpeta se puede
mandar completa al servicio de impresión. Cómo imprimir cada pieza (relleno,
perímetros, soportes) y cuánto pesa está en la wiki: [Diseño 3D](../../wiki/Diseno-3D.md#piezas-a-imprimir).

| Carpeta | Parte | Piezas | Copias |
| --- | --- | --- | --- |
| [`parte2-base`](parte2-base) | 2 · Base de melamina | `base_charola`, `base_soporte_bt`, `base_panel_carga`, `base_soporte_motor`, `base_soporte_motor_izq` | 1, 2, 1, 1, 1 |
| [`parte5-poste`](parte5-poste) | 5 · Cabeza (poste con imanes) | `poste_pie_poste`, `poste_soporte_servo`, `poste_portaimanes` | 1 de cada |
| [`parte5-cabeza`](parte5-cabeza) | 5 · Cabeza | `cabeza_casco`, `cabeza_plato`, `cabeza_soporte_ojo`, `cabeza_soporte_bocina`, `cabeza_tapon`, `cabeza_antena_corta`, `cabeza_antena_larga` | 1, 1, 1, 1, 3, 1, 1 |
| [`parte6-esfera`](parte6-esfera) | 6 · Esfera | 14 gajos (`esfera_gajo_A_*`, `esfera_gajo_B_*`), `esfera_circulo`, `esfera_tapa_carga` | 1 de cada gajo, 5 círculos, 1 tapa |

- Los soportes de motor (`base_soporte_motor*`) solo hacen falta si los motores no van
  sujetos con abrazaderas o soportes en L comprados.
- Antes de mandar los 14 gajos, imprime un gajo A y un gajo B y prueba que la bayoneta
  cierre.

Se regeneran con `bash cad/exportar.sh stl`.
