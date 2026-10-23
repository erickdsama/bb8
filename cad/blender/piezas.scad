// BB-8 · piezas visibles en su lugar, para animarlas en Blender.
//
// A diferencia de los STL de cad/stl (orientados para imprimir), estas salen armadas:
// origen en el centro de la esfera, +y adelante, la cabeza sobre el polo de arriba con
// su propio origen en ese polo (Blender la sube a z = Re).
//
// Uso: openscad -D 'pieza="esfera_blanca"' -o esfera_blanca.stl piezas.scad
//   piezas: esfera_blanca  esfera_naranja  cabeza  antenas  ojo_domo  ojo_anillo

include <../parametros.scad>
use <../esfera.scad>
use <../cabeza.scad>

pieza = "esfera_blanca";

ALFA = acos(1 / sqrt(3));
EJES = [[1, 0, 0], [0, 1, 0], [0, 0, 1], [-1, 0, 0], [0, -1, 0], [0, 0, -1]];

module esfera_mundo() rotate(a = ALFA, v = [1, -1, 0]) children();

if (pieza == "esfera_blanca") esfera_mundo() { mitad_A(); mitad_B(); }
else if (pieza == "esfera_naranja") esfera_mundo() for (e = EJES) apuntar(e) circulo();
else if (pieza == "cabeza") casco();
else if (pieza == "antenas") antenas();
else if (pieza == "ojo_domo") ojo_domo();
else if (pieza == "ojo_anillo") ojo_anillo();
