// BB-8 · ensamble completo, para revisar que todo cabe.
//
//   corte = true   quita el cuarto de esfera que da a la cámara (vista de corte)
//   explotar = 0…1 separa la cabeza y las tapas para ver cómo se arma
//
// openscad -D corte=true ensamble.scad

include <parametros.scad>
use <esfera.scad>
use <base.scad>
use <poste.scad>
use <cabeza.scad>

corte = true;
explotar = 0;
con_esfera = true;

ALFA = acos(1 / sqrt(3));

module esfera_mundo() rotate(a = ALFA, v = [1, -1, 0]) esfera_completa(explosion = 60 * explotar);

if (con_esfera) {
    if (corte) difference() {
        esfera_mundo();
        translate([-Re - 10, -Re - 10, -Re - 10]) cube([Re + 10, Re + 10, 2 * Re + 20]);
    } else esfera_mundo();
}
base_completa();
poste_completo();
translate([0, 0, Re + 120 * explotar]) difference() {
    cabeza_completa();
    if (corte) translate([-200, -200, -60]) cube([200, 200, 200]);
}
reportar();
