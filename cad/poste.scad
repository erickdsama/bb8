// BB-8 · poste e imanes (Partes 2, 5 y 6).
//
// Tubo PVC fijo a la plataforma; arriba, el MG996R con el eje en el centro del tubo y,
// sobre su cuerno, el portaimanes: 4 imanes N42 de 20 × 5 que siguen la curva interior
// del casco a `hueco_iman_int` (2.5 mm). La cabeza sigue a estos imanes cuando el servo gira.
//
// Uso: openscad -D 'parte="portaimanes"' -o portaimanes.stl poste.scad
//   partes: pie_poste  soporte_servo  portaimanes  vista

include <parametros.scad>
use <componentes.scad>

parte = "vista";

r_porta = Ri - hueco_iman_int;     // esfera que sigue la cara de arriba del portaimanes
manga = 15;                        // cuánto abraza el soporte del servo al tubo

// Dirección del imán k (desde el centro de la esfera).
function dir_iman(k, R) = let(a = 45 + 90 * k) unitario([iman_r * cos(a), iman_r * sin(a), sqrt(R * R - iman_r * iman_r)]);

module pie_poste() {
    translate([0, 0, plat_z_sup]) difference() {
        union() {
            cylinder(d = 56, h = 5);
            cylinder(d = tubo_d + 8, h = pie_socket);
            for (k = [0 : 3]) rotate([0, 0, 90 * k]) hull() {
                translate([tubo_d / 2 + 2, -1.5, 0]) cube([1, 3, pie_socket - 2]);
                translate([25, -1.5, 0]) cube([1, 3, 5]);
            }
        }
        translate([0, 0, 2]) cylinder(d = tubo_d + 2 * holg, h = pie_socket);
        translate([0, 0, -1]) cylinder(d = tubo_d - 6, h = 5);               // cables hacia abajo
        rotate([0, 0, 45]) translate([0, -5, 2]) cube([30, 10, 12]);            // salida lateral del cable del servo
        for (k = [0 : 3]) rotate([0, 0, 45 + 90 * k]) translate([22, 0, -1]) {
            cylinder(d = 3.4, h = 10, $fn = 16);
            translate([0, 0, 4]) cylinder(d = 6.5, h = 10, $fn = 24);
        }
    }
}

module soporte_servo() {
    caja = [servo_oreja_L + 6, servo[1] + 10];
    cx = -servo_eje_x;    // el cuerpo del servo queda corrido para que el eje caiga en el centro
    difference() {
        union() {
            translate([0, 0, tubo_z_sup - manga]) cylinder(d = tubo_d + 8, h = manga + 3);
            translate([cx - caja[0] / 2, -caja[1] / 2, tubo_z_sup]) cube([caja[0], caja[1], servo_soporte_z - tubo_z_sup]);
        }
        translate([0, 0, tubo_z_sup - manga - 1]) cylinder(d = tubo_d + 2 * holg, h = manga + 1);
        translate([cx - servo[0] / 2 - holg, -servo[1] / 2 - holg, servo_z_inf]) cube([servo[0] + 2 * holg, servo[1] + 2 * holg, 100]);
        // cable: baja por el tubo
        translate([0, 0, tubo_z_sup - 1]) cylinder(d = 12, h = 20);
        translate([cx - servo[0] / 2 - 6, -5, tubo_z_sup + 3]) cube([10, 10, 100]);
        // tornillos de las orejas (M3 autorroscante en piloto de 2.5)
        for (sx = [-1, 1], sy = [-1, 1])
            translate([cx + sx * servo_agujeros[0] / 2, sy * servo_agujeros[1] / 2, servo_soporte_z - 14])
                cylinder(d = 2.5, h = 15, $fn = 12);
        // aligerar
        for (sx = [-1, 1]) translate([cx + sx * (servo[0] / 2 + 6.5) - 3.5, -caja[1] / 2 - 1, tubo_z_sup + 4]) cube([7, caja[1] + 2, servo_soporte_z - tubo_z_sup - 8]);
    }
}

module portaimanes() {
    difference() {
        intersection() {
            sphere(r_porta, $fn = 240);
            translate([0, 0, porta_z_inf]) cylinder(d = porta_d, h = 50, $fn = 96);
        }
        for (k = [0 : 3]) {
            d = dir_iman(k, r_porta);
            translate(d * r_porta) apuntar(d) {
                translate([0, 0, -iman_e]) cylinder(d = iman_d + 0.4, h = iman_e + 5, $fn = 64);       // imán
                translate([0, 0, -30]) cylinder(d = 4.5, h = 30, $fn = 16);                          // M4
            }
            // tuerca de nylon M4 por debajo, donde el eje del imán sale por la cara de abajo
            t = (r_porta * d[2] - porta_z_inf) / d[2];
            p = (r_porta - t) * d;
            translate([p[0], p[1], porta_z_inf - 1]) rotate([0, 0, 30]) cylinder(d = 8.1, h = 5, $fn = 6);
        }
        // cuerno redondo del servo y acceso al tornillo central
        translate([0, 0, porta_z_inf - 1]) cylinder(d = cuerno_d + 0.6, h = cuerno_e + 1, $fn = 64);
        translate([0, 0, porta_z_inf]) cylinder(d = 7, h = 50, $fn = 24);
        // 4 tornillos M2 al cuerno: MEDIR el cuerno y ajusta el radio
        for (k = [0 : 3]) rotate([0, 0, 90 * k]) translate([8, 0, porta_z_inf]) {
            cylinder(d = 2.2, h = 50, $fn = 12);
            translate([0, 0, 6]) cylinder(d = 4.2, h = 50, $fn = 16);
        }
    }
}

module poste_completo() {
    color("orange") pie_poste();
    color("lightgray") translate([0, 0, plat_z_sup + 2]) difference() {
        cylinder(d = tubo_d, h = tubo_z_sup - plat_z_sup - 2);
        translate([0, 0, -1]) cylinder(d = tubo_d - 3, h = 500);
    }
    color("orange") soporte_servo();
    translate([0, 0, servo_z_inf]) mg996r();
    translate([0, 0, porta_z_inf]) cuerno();
    color("orange") portaimanes();
    for (k = [0 : 3]) let(d = dir_iman(k, r_porta)) translate(d * r_porta) apuntar(d) iman();
}

if (parte == "vista") poste_completo();
else if (parte == "pie_poste") pie_poste();
else if (parte == "soporte_servo") soporte_servo();
else if (parte == "portaimanes") portaimanes();
