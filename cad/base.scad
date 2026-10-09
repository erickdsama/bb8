// BB-8 · base motriz (Partes 2 y 6).
//
// La plataforma de melamina va arriba y todo lo pesado cuelga debajo: motores en soportes
// impresos, la LiPo en una charola justo bajo los motores y, a los lados de la LiPo, dos
// compartimentos para 1–1.5 kg de lastre. Los ball transfers van adelante y atrás en
// soportes inclinados que apuntan al centro de la esfera.
//
// Las medidas que salen de la geometría (altura del eje, de la plataforma, inclinación de
// los ball transfers) se calculan en parametros.scad; corre `reportar()` para verlas.
//
// Uso: openscad -D 'parte="soporte_motor"' -o soporte_motor.stl base.scad
//   partes: plataforma (3D)  plataforma_2d (plantilla DXF/SVG 1:1)  soporte_motor
//           soporte_motor_izq  charola  soporte_bt (imprime 2)  panel_carga  vista
// Las piezas sueltas salen ya orientadas para imprimir.

include <parametros.scad>
use <componentes.scad>

parte = "vista";

x_cara = a_rueda - rueda_offset;           // cara de la caja del motor derecho
x_silla = x_cara - caja_L - motor_L + 6;   // donde empieza la silla (el encoder queda libre)
silla_ancho = 52;
ly = lipo[1] + 1;                          // hueco de la LiPo
lx = lipo[0] + 1;
charola_z = lipo_z_inf - charola_piso;     // cara de abajo de la charola

// Posiciones sobre la plataforma (esquina inferior izquierda de cada placa).
POS_PI4   = [-42.5, 34];
POS_UNO   = [5 - 68.6 / 2, -88.7];
POS_L298N = [-71.5, -21.5];
POS_BUCK  = [[28.5, 0.5], [28.5, -21.5]];
POS_MPU   = [-75, 30];
PANEL_AZ  = 225;   // panel de carga: atrás a la izquierda, mirando hacia afuera
PANEL_R   = 86;
alto_separadores = 8;

// ───────────── Agujeros compartidos por la plataforma y las piezas ─────────────
function agujeros_motor(lado) = [for (x = [x_silla + 8, x_cara - 8], s = [-1, 1]) [lado * x, excentrico + s * 22]];
function agujeros_charola() = [for (x = [-15, 15], s = [-1, 1]) [x, s * (ly / 2 - 4)]];
function agujeros_bt(s) = [for (p = [[-14, 86], [14, 86], [0, 98]]) [p[0], s * p[1]]];
function agujeros_pie() = [for (k = [0 : 3]) 22 * [cos(45 + 90 * k), sin(45 + 90 * k)]];
function agujeros_panel() = [for (t = [-18, 18]) (PANEL_R - 9) * [cos(PANEL_AZ), sin(PANEL_AZ)] + t * [-sin(PANEL_AZ), cos(PANEL_AZ)]];
PI4_AG   = [[3.5, 3.5], [61.5, 3.5], [3.5, 52.5], [61.5, 52.5]];
UNO_AG   = [[14, 2.5], [15.3, 50.7], [66.1, 7.6], [66.1, 35.5]];
L298N_AG = [[3, 3], [40, 3], [3, 40], [40, 40]];   // MEDIR

// ───────────── Plataforma ─────────────
module plataforma_2d() {
    // la rueda cruza la plataforma: medio ancho de la muesca a la altura de su cara inferior
    dz = plat_z_inf - z_eje;
    muesca_y = sqrt(rueda_d * rueda_d / 4 - dz * dz) + 4;
    muesca_x = a_rueda - rueda_ancho / 2 - 4;
    difference() {
        circle(d = plat_d, $fn = 180);
        for (l = [-1, 1]) translate([l > 0 ? muesca_x : -muesca_x - plat_d, -muesca_y]) square([plat_d, 2 * muesca_y]);
        for (p = concat(agujeros_motor(1), agujeros_motor(-1), agujeros_charola(),
                        agujeros_bt(1), agujeros_bt(-1), agujeros_pie(), agujeros_panel()))
            translate(p) circle(d = 3.4, $fn = 16);
        for (p = PI4_AG) translate(POS_PI4 + p) circle(d = 2.8, $fn = 16);
        for (p = UNO_AG) translate(POS_UNO + p) circle(d = 3.4, $fn = 16);
        for (p = L298N_AG) translate(POS_L298N + p) circle(d = 3.4, $fn = 16);
        // pasacables (motores, LiPo, ball transfers) y centro marcado
        for (x = [-55, 55]) translate([x, -40]) circle(d = 22, $fn = 48);
        circle(d = 2, $fn = 8);
    }
}

module plataforma() color("burlywood") translate([0, 0, plat_z_inf]) linear_extrude(plat_e) plataforma_2d();

// ───────────── Soporte de motor (derecho; el izquierdo es su espejo) ─────────────
// Silla colgada bajo la plataforma más una placa frontal que se atornilla a la cara de la
// caja con tornillos M3 avellanados (la cabeza no puede salir: la rueda pasa a 3 mm).
module soporte_motor() {
    yc = excentrico;
    alto = plat_z_inf - z_eje;
    difference() {
        union() {
            translate([x_silla, yc - silla_ancho / 2, z_eje]) cube([x_cara - x_silla, silla_ancho, alto]);
            hull() {
                translate([x_cara, yc - silla_ancho / 2, z_eje]) cube([4, silla_ancho, alto]);
                translate([x_cara, yc, z_eje]) rotate([0, 90, 0]) cylinder(d = caja_d + 8, h = 4);
            }
        }
        translate([x_cara - caja_L - 0.5, yc, z_eje]) rotate([0, 90, 0]) cylinder(d = caja_d + 2 * holg, h = caja_L + 0.51);
        translate([x_silla - 1, yc, z_eje]) rotate([0, 90, 0]) cylinder(d = motor_d + 1.5, h = x_cara - caja_L - x_silla + 1.01);
        translate([x_cara - 1, 0, z_eje]) rotate([0, 90, 0]) cylinder(d = buje_d + 1, h = 10);
        for (k = [0 : 5]) translate([x_cara - 1, yc + caja_pcd / 2 * cos(60 * k), z_eje + caja_pcd / 2 * sin(60 * k)])
            rotate([0, 90, 0]) {
                cylinder(d = 3.4, h = 10, $fn = 16);
                translate([0, 0, 1 + 4 - 1.8]) cylinder(d1 = 3.4, d2 = 6.6, h = 1.81, $fn = 16);
            }
        // ranuras para 2 bridas alrededor del motor
        for (x = [x_silla + 12, x_cara - caja_L - 10])
            translate([x, yc - silla_ancho, z_eje + motor_d / 2 + 1]) cube([5, 2 * silla_ancho, 2.2]);
        // tornillos M3 a la plataforma con tuerca atrapada
        for (p = agujeros_motor(1)) translate([p[0], p[1], plat_z_inf - 14]) {
            cylinder(d = 3.4, h = 15, $fn = 16);
            translate([0, 0, 4]) rotate([0, 0, 30]) cylinder(d = 6.4, h = 2.8, $fn = 6);
            translate([0, (p[1] > yc ? 1 : -1) * 6, 4 + 1.4]) cube([5.6, 12, 2.8], center = true);
        }
    }
}

// ───────────── Charola de la LiPo con lastre ─────────────
module charola() {
    ancho = lx + 2 * (lastre_ancho + 2 * charola_pared);
    largo = ly + 2 * charola_pared;
    alto_muro = lipo[2] - 5;
    difference() {
        union() {
            intersection() {
                translate([-ancho / 2, -largo / 2, charola_z]) cube([ancho, largo, charola_piso + alto_muro]);
                sphere(Ri - 3, $fn = 160);
            }
            for (s = [-1, 1]) {
                translate([-25, s > 0 ? ly / 2 : -ly / 2 - charola_pared, charola_z]) cube([50, charola_pared, plat_z_inf - charola_z]);
                translate([-25, s > 0 ? ly / 2 - 10 : -ly / 2, plat_z_inf - 5]) cube([50, 10, 5]);
                // costillas
                for (x = [-25, 21]) translate([x, s > 0 ? ly / 2 - 6 : -ly / 2, charola_z]) cube([4, 6, plat_z_inf - charola_z]);
            }
        }
        translate([-lx / 2, -ly / 2, charola_z + charola_piso]) cube([lx, ly, 200]);
        for (s = [-1, 1]) translate([s > 0 ? lx / 2 + charola_pared : -lx / 2 - charola_pared - lastre_ancho,
                                     -lastre_largo / 2, charola_z + charola_piso]) cube([lastre_ancho, lastre_largo, 200]);
        // ranuras para 2 correas de velcro de 20 mm
        for (y = [-35, 35], s = [-1, 1]) translate([s * (lx / 2 + 1.2) - 1.5, y - 11, charola_z - 1]) cube([3, 22, 10]);
        for (p = agujeros_charola()) translate([p[0], p[1], plat_z_inf - 20]) cylinder(d = 3.4, h = 21, $fn = 16);
        // tuercas M3 por debajo de la pestaña
        for (p = agujeros_charola()) translate([p[0], p[1], plat_z_inf - 5 - 3]) rotate([0, 0, 30]) cylinder(d = 6.4, h = 5.6, $fn = 6);
        // salida del cable de la LiPo
        translate([-8, -ly / 2 - charola_pared - 1, charola_z + charola_piso + 4]) cube([16, charola_pared + 2, 30]);
    }
}

// ───────────── Soporte de ball transfer (s = 1 adelante, −1 atrás) ─────────────
bt_c = [0, bt_y, bt_c_z];
bt_u = unitario(bt_c);
bt_cara = bt_c - (bt_bola_fuera - bola_d / 2) * bt_u;   // cara de la brida
bt_atras = bt_cara - bt_brida_e * bt_u;                  // espalda de la brida

module soporte_bt(s = 1) {
    mirror([0, s > 0 ? 0 : 1, 0]) difference() {
        hull() {
            translate([-24, 80, plat_z_inf - 6]) cube([48, 24, 6]);
            translate(bt_atras) apuntar(bt_u) translate([0, 0, -8]) cylinder(d = bt_brida_d + 2, h = 8);
        }
        // todo lo que va delante de la espalda de la brida (brida, bola) y el cuerpo
        translate(bt_atras) apuntar(bt_u) {
            cylinder(d = bt_brida_d + 30, h = 60);
            translate([0, 0, -bt_cuerpo_L - 1]) cylinder(d = bt_cuerpo_d + 1, h = bt_cuerpo_L + 1.01);
            // tornillos M4 con tuerca por detrás (cavidad para llave de vaso)
            for (k = [0 : 2]) rotate([0, 0, 90 + 120 * k]) translate([bt_pcd / 2, 0, 0]) {
                translate([0, 0, -12]) cylinder(d = 4.5, h = 13, $fn = 16);
                translate([0, 0, -60 - 8]) cylinder(d = 10, h = 60, $fn = 24);
            }
        }
        for (p = agujeros_bt(1)) translate([p[0], p[1], plat_z_inf - 20]) {
            cylinder(d = 3.4, h = 21, $fn = 16);
            translate([0, 0, -46]) cylinder(d = 7, h = 60, $fn = 24);   // cabeza del tornillo por abajo
        }
    }
}

// ───────────── Panel de carga ─────────────
// Rocker 10 A (corte 19 × 13), XT60 hembra pegado y extensión de balanceo JST-XH 4p.
// Va en la base, no en el casco: la esfera gira alrededor de la base y cualquier cable
// al casco se enredaría. Se alcanza por la tapa de carga girando la esfera con la mano.
module panel_carga() {
    rotate([0, 0, PANEL_AZ]) translate([PANEL_R, 0, plat_z_sup]) difference() {
        union() {
            translate([-12, -30, 0]) cube([15, 60, 3]);          // pie
            translate([0, -30, 0]) cube([3, 60, 36]);           // cara
            for (y = [-30, 27]) hull() {
                translate([-12, y, 0]) cube([1, 3, 3]);
                translate([0, y, 0]) cube([1, 3, 36]);
            }
        }
        translate([-1, -9.5 - 11, 18 - 6.5]) cube([5, 19, 13]);          // rocker
        translate([-1, 6, 6]) cube([5, 16.2, 8.4]);                      // XT60
        translate([-1, 6, 22]) cube([5, 12.6, 6]);                       // JST-XH 4p
        for (t = [-18, 18]) translate([-9, t, -1]) cylinder(d = 3.4, h = 5, $fn = 16);
    }
}

// ───────────── Vista ─────────────
module electronica() {
    z = plat_z_sup + alto_separadores;
    translate([POS_PI4[0], POS_PI4[1], z]) raspberry_pi4();
    translate([POS_UNO[0], POS_UNO[1], z]) arduino_uno();
    translate([POS_L298N[0], POS_L298N[1], z]) l298n();
    for (p = POS_BUCK) translate([p[0], p[1], plat_z_sup + 1]) lm2596();
    translate([POS_MPU[0], POS_MPU[1], plat_z_sup + 1]) mpu6050();
}

module motores_y_ruedas() {
    for (l = [-1, 1]) mirror([l < 0 ? 1 : 0, 0, 0]) {
        translate([x_cara, 0, z_eje]) jgb37();
        translate([a_rueda, 0, z_eje]) rueda();
    }
}

module base_completa() {
    plataforma();
    color("orange") { soporte_motor(); mirror([1, 0, 0]) soporte_motor(); }
    motores_y_ruedas();
    color("deepskyblue") charola();
    translate([0, 0, lipo_z_inf]) lipo();
    for (s = [-1, 1]) color("orange") soporte_bt(s);
    for (s = [-1, 1]) mirror([0, s > 0 ? 0 : 1, 0]) translate(bt_cara) apuntar(bt_u) ball_transfer();
    color("orange") panel_carga();
    electronica();
}

if (parte == "vista") { base_completa(); reportar(); }
else if (parte == "plataforma") plataforma();
else if (parte == "plataforma_2d") plataforma_2d();
else if (parte == "soporte_motor") rotate([180, 0, 0]) soporte_motor();
else if (parte == "soporte_motor_izq") rotate([180, 0, 0]) mirror([1, 0, 0]) soporte_motor();
else if (parte == "charola") charola();
else if (parte == "soporte_bt") rotate([180, 0, 0]) soporte_bt(1);
else if (parte == "panel_carga") panel_carga();
