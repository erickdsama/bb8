// BB-8 · cuna de carga (Parte 6).
//
// La esfera se sienta en un asiento esférico y carga por inducción: la bobina emisora va
// en el fondo del asiento, bajo una piel de cuna_tapa mm, y la receptora cuelga bajo la
// charola de la base (base.scad), que por gravedad queda siempre encima. Sin tapas, sin
// girar nada y sin cables adentro de la esfera.
//
// Por debajo: hueco para la placa del emisor y salida lateral para el jack de corriente.
// Medidas del emisor aproximadas: MEDIR la placa y la bobina que lleguen.
//
// Uso: openscad -D 'parte="cuna"' -o cuna.stl cuna.scad
//   partes: cuna  vista

include <parametros.scad>

parte = "vista";

cuna_d     = 150;                  // Ø exterior
placa      = [45, 35, 12];         // placa del emisor (MEDIR)
jack_d     = 8;                    // jack DC de panel (o el cable del eliminador)
z_bobina   = placa[2] + 1;         // fondo de la bobina emisora
z_asiento  = z_bobina + bobina_e + cuna_tapa;   // fondo del asiento (donde toca la esfera)
holgura_s  = 0.6;                  // el asiento es un poco más grande que la esfera
r_borde    = cuna_d / 2 - 4;
cuna_alto  = z_asiento + (Re + holgura_s) - sqrt(pow(Re + holgura_s, 2) - r_borde * r_borde);
z_centro   = z_asiento + Re + holgura_s;        // centro de la esfera sentada

module cuna() {
    difference() {
        cylinder(d = cuna_d, h = cuna_alto, $fn = 160);
        // asiento esférico
        translate([0, 0, z_centro]) sphere(Re + holgura_s, $fn = 240);
        // bobina emisora, entra por abajo hasta la piel
        translate([0, 0, z_bobina - 0.01]) cylinder(d = bobina_d + 0.8, h = bobina_e + 0.01, $fn = 96);
        // placa del emisor y su cableado
        translate([-placa[0] / 2, -placa[1] / 2, -1]) cube([placa[0], placa[1], placa[2] + 1]);
        translate([-4, -4, -1]) cube([8, 8, z_bobina + 1]);
        // canal al costado y jack
        translate([0, -5, -1]) cube([cuna_d / 2 + 1, 10, 8]);
        translate([cuna_d / 2 - 6, 0, 6]) rotate([0, 90, 0]) cylinder(d = jack_d, h = 10, $fn = 32);
        // 4 patas de hule de 10 mm
        for (k = [0 : 3]) rotate([0, 0, 45 + 90 * k]) translate([cuna_d / 2 - 14, 0, -0.01]) cylinder(d = 10.5, h = 1.5, $fn = 32);
    }
}

if (parte == "cuna") cuna();
else {
    cuna();
    color("#555") translate([0, 0, z_bobina]) cylinder(d = bobina_d, h = bobina_e, $fn = 64);
    %translate([0, 0, z_centro]) sphere(Re, $fn = 96);
    // bobina receptora bajo la base, con la esfera sentada
    color("#555") translate([0, 0, z_centro - (Ri - bobina_aire)]) cylinder(d = bobina_d, h = bobina_e, $fn = 64);
    echo(str("cuna: Ø", cuna_d, " × ", cuna_alto, " mm; bobina a bobina ",
             (z_centro - (Ri - bobina_aire)) - (z_bobina + bobina_e), " mm"));
}
