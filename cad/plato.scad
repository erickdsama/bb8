// BB-8 · plato de carga (Parte 6).
//
// Bebocho rueda encima y la gravedad lo deja en el centro, sobre la bobina emisora: no
// importa hacia dónde mire ni cómo llegue, porque la bobina receptora va bajo la base y la
// base siempre queda abajo. Sube por una rampa de ~6.8° hasta una orilla de 8 mm y cae a
// un hoyo de 2.5 mm de hondo (pendiente máxima ~2.9°): para salir sobra fuerza.
//
// Es más grande que la cama de impresión, así que va en piezas: el centro (con la bobina,
// que entra por abajo bajo una piel de piel_bobina mm) y la rampa en 4 cuartos que se pegan
// alrededor. La placa del emisor va en una cajita aparte, atrás, con el jack; su cable
// llega a la bobina por una ranura bajo el centro y uno de los cuartos.
//
// Uso: openscad -D 'parte="centro"' -o plato_centro.stl plato.scad
//   partes: centro  rampa (imprime 4; la ranura solo en uno: rampa_cable)  caja  vista

include <parametros.scad>

parte = "vista";

plato_r1    = 100;                         // radio del hoyo (pieza central, Ø200)
plato_r2    = 170;                         // radio total (Ø340)
plato_hondo = 2.5;                         // cuánto baja el centro respecto a la orilla
plato_c     = piel_bobina + bobina_e + 0.5;  // grosor en el centro: piel + bobina + 0.5
plato_h     = plato_c + plato_hondo;       // altura de la orilla del hoyo
plato_Rp    = (plato_r1 * plato_r1 + plato_hondo * plato_hondo) / (2 * plato_hondo);  // radio del hoyo esférico
ranura      = [6, 3];                      // ancho × alto de la ranura del cable, por abajo
caja        = [60, 50, 22];                // cajita del emisor (MEDIR la placa)

module centro() {
    difference() {
        cylinder(r = plato_r1, h = plato_h, $fn = 200);
        translate([0, 0, plato_c + plato_Rp]) sphere(plato_Rp, $fn = 400);
        // bobina, por abajo hasta la piel
        translate([0, 0, -1]) cylinder(d = bobina_d + 0.8, h = 1 + plato_c - piel_bobina, $fn = 96);
        // ranura del cable hacia atrás (−y)
        translate([-ranura[0] / 2, -plato_r1 - 1, -1]) cube([ranura[0], plato_r1 + 1, ranura[1] + 1]);
    }
}

function rampa_z(r) = plato_h * (plato_r2 - r) / (plato_r2 - plato_r1 - 3);   // altura de la rampa a radio r

// Sobre la rampa la esfera necesita fricción ≥ tan 7° ≈ 0.12: las líneas de agarre y
// una capa de Plasti Dip la dan aunque haya polvo. Debajo de todo el plato va una hoja
// de hule antiderrapante (al llegar con vuelo, Bebocho empujaría un plato de ~0.4 kg).
module rampa(cable = false) {
    difference() {
        intersection() {
            rotate_extrude($fn = 240) {
                polygon([[plato_r1 + 0.3, 0], [plato_r1 + 0.3, plato_h], [plato_r1 + 3, plato_h], [plato_r2, 0]]);
                // líneas de agarre: anillos de 0.4 mm cada 5 mm, de través a la subida
                for (r = [plato_r1 + 8 : 5 : plato_r2 - 6]) translate([r, rampa_z(r) - 0.2]) square([1.2, 0.6]);
            }
            translate([0, 0, -1]) cube([plato_r2 + 1, plato_r2 + 1, plato_h + 2]);   // un cuarto
        }
        if (cable) rotate([0, 0, 45]) translate([plato_r1 - 1, -ranura[0] / 2, -1]) cube([plato_r2, ranura[0], ranura[1] + 1]);
    }
}

module caja_emisor() {
    difference() {
        translate([-caja[0] / 2, 0, 0]) cube(caja);
        translate([-caja[0] / 2 + 2, 2, -1]) cube([caja[0] - 4, caja[1] - 4, caja[2] - 1]);     // abierta por abajo
        translate([0, caja[1] - 3, 9]) rotate([-90, 0, 0]) cylinder(d = 8, h = 5, $fn = 32);    // jack
        translate([-ranura[0] / 2, -1, -1]) cube([ranura[0], 4, ranura[1] + 1]);               // cable a la bobina
    }
}

if (parte == "centro") centro();
else if (parte == "rampa") rampa();
else if (parte == "rampa_cable") rampa(true);
else if (parte == "caja") caja_emisor();
else {
    color("orange") centro();
    color("darkorange") for (k = [0 : 3]) rotate([0, 0, 90 * k - 45 - 90]) rampa(k == 0);
    color("orange") translate([0, -plato_r2 - 2, 0]) mirror([0, 1, 0]) caja_emisor();
    color("#555") translate([0, 0, plato_c - piel_bobina - bobina_e]) cylinder(d = bobina_d, h = bobina_e, $fn = 64);
    z_centro = plato_c + Re;   // la esfera toca el centro del hoyo
    %translate([0, 0, z_centro]) sphere(Re, $fn = 96);
    color("#555") translate([0, 0, z_centro - (Ri - bobina_aire)]) cylinder(d = bobina_d, h = bobina_e, $fn = 64);
    echo(str("plato: Ø", 2 * plato_r2, ", orilla ", plato_h, " mm, rampa ",
             atan(plato_h / (plato_r2 - plato_r1 - 3)), "°, hoyo ", asin(plato_r1 / plato_Rp), "° en la orilla; bobina a bobina ",
             (z_centro - (Ri - bobina_aire)) - plato_c + piel_bobina, " mm"));
}
