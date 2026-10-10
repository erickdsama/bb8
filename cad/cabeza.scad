// BB-8 · cabeza (Parte 5 y 6).
//
// Marco de la cabeza: origen en el polo de arriba de la esfera (z_robot = Re), +y adelante.
// La cabeza no toca el casco más que con 3 bolas de acero de 1/2″ en copas; los 4 imanes
// quedan a `hueco_iman_ext` (2 mm) de la pintura. Todo cuelga de un plato interior:
//   arriba  Pi Zero 2 W, PAM8403, soporte del ojo (anillo WS2812 + OV5647) y de la bocina;
//   abajo   power bank (baja el centro de gravedad), copas, imanes y el VL53L0X del ojo chico.
// El casco baja sobre el plato y se fija con 3 tornillos M3 radiales.
//
// Uso: openscad -D 'parte="casco"' -o cabeza_casco.stl cabeza.scad
//   partes: casco  plato  tapon (imprime 3)  soporte_ojo  soporte_bocina  vista
// Las piezas sueltas salen ya orientadas para imprimir.

include <parametros.scad>
use <componentes.scad>

parte = "vista";

Rd = cab_domo_r;
zc = cab_domo_zc;
r_hueco = Re + cab_hueco_borde;       // esfera del borde de abajo
z_borde = sqrt(r_hueco * r_hueco - cab_r_borde * cab_r_borde) - Re;
plato_sup = plato_z + plato_e;
plato_r = sqrt(pow(Rd - cab_pared, 2) - pow(plato_sup - zc, 2)) - 0.6;
TORNILLOS_AZ = [60, 180, 300];
COPAS_AZ = [90, 210, 330];

// ───────────── Perfil y casco ─────────────
function arco(R, z0, z1, n = 32) = [for (i = [0 : n]) let(z = z0 + (z1 - z0) * i / n) [sqrt(R * R - pow(z - zc, 2)), z]];

module perfil(p) {
    // p = 0: exterior; p = pared: interior
    polygon(concat(
        [[0, z_borde - 20], [cab_r_borde - p, z_borde - 20], [cab_r_borde - p, z_borde]],
        [[Rc - p, cab_z_ancho]],
        arco(Rd - p, cab_z_ancho, cab_z_tope - p),
        [[0, cab_z_tope - p]]));
}

// direcciones (desde el centro del domo) del ojo y de la bocina
n_ojo = [0, cos(ojo_elev), sin(ojo_elev)];
n_boc = [0, -cos(bocina_elev), sin(bocina_elev)];
n_ojo2 = [sin(ojo2_azim) * cos(ojo2_inclin), cos(ojo2_azim) * cos(ojo2_inclin), -sin(ojo2_inclin)];
// radio exterior de la banda a la altura z
function r_banda(z) = cab_r_borde + (Rc - cab_r_borde) * (z - z_borde) / (cab_z_ancho - z_borde);
p_ojo2 = [sin(ojo2_azim) * r_banda(ojo2_z), cos(ojo2_azim) * r_banda(ojo2_z), ojo2_z];

module casco() {
    difference() {
        rotate_extrude($fn = 180) perfil(0);
        rotate_extrude($fn = 180) perfil(cab_pared);
        translate([0, 0, -Re]) sphere(r_hueco, $fn = 360);
        // ojo principal
        translate([0, 0, zc]) apuntar(n_ojo) cylinder(d = ojo_hueco_d, h = Rd + 5, $fn = 96);
        // ojo chico (VL53L0X)
        translate(p_ojo2) apuntar(n_ojo2) cylinder(d = ojo2_d, h = 20, center = true, $fn = 48);
        // rejilla de la bocina: hexágono de agujeros de 3 mm
        translate([0, 0, zc]) apuntar(n_boc) for (i = [-3 : 3], j = [-3 : 3])
            let(x = 6 * (i + j / 2), y = 6 * j * sqrt(3) / 2)
                if (x * x + y * y < pow(bocina_d / 2 - 4, 2)) translate([x, y, Rd - 10]) cylinder(d = 3, h = 20, $fn = 12);
        // antenas (Ø6 y Ø4) en la tapa, hacia atrás
        for (p = [[0, -cab_tapa_r * 0.45, 6], [9, -cab_tapa_r * 0.72, 4]]) translate([p[0], p[1], cab_z_tope - 10]) cylinder(d = p[2], h = 20, $fn = 24);
        // puerto USB del power bank, atrás bajo la antena
        translate([0, -Rc, 2]) cube([13, 30, 7], center = true);
        // tornillos M3 avellanados al plato
        for (a = TORNILLOS_AZ) rotate([0, 0, a]) translate([0, 0, plato_z + plato_e / 2]) rotate([-90, 0, 0]) {
            cylinder(d = 3.4, h = Rc + 5, $fn = 16);
            translate([0, 0, r_banda(plato_z) - 1.9]) cylinder(d1 = 3.4, d2 = 6.8, h = 1.91, $fn = 24);
        }
        // líneas de panel decorativas
        for (z = [cab_z_ancho + 3, cab_z_ancho + 36]) translate([0, 0, z])
            difference() { cylinder(r = Rc + 5, h = 1.2, $fn = 180); cylinder(r = sqrt(Rd * Rd - pow(z - zc, 2)) - 0.6, h = 1.2, $fn = 180); }
    }
}

// ───────────── Plato interior ─────────────
// Dirección de los imanes y de las copas desde el centro de la esfera.
function dir_iman_cab(k) = let(a = 45 + 90 * k, R = Re + hueco_iman_ext) unitario([iman_r * cos(a), iman_r * sin(a), sqrt(R * R - iman_r * iman_r)]);
function dir_copa(a) = let(R = Re + bolita_d / 2) unitario([bolita_r * cos(a), bolita_r * sin(a), sqrt(R * R - bolita_r * bolita_r)]);
// del marco de la esfera al de la cabeza
function cab(p) = p - [0, 0, Re];

lip = sqrt(pow(bolita_d / 2, 2) - pow(0.85 * bolita_d / 2, 2));   // la copa abraza la bola hasta su 85 %

module poste_a_plato(p, d, largo, diam) {
    // cilindro sobre el eje d que arranca en p y sube hasta el plato
    hull() {
        translate(p) apuntar(d) cylinder(d = diam, h = largo);
        translate([p[0], p[1], plato_z]) cylinder(d = diam, h = 1);
    }
}

module plato() {
    difference() {
        union() {
            translate([0, 0, plato_z]) cylinder(r = plato_r, h = plato_e, $fn = 180);
            // postes de imanes
            for (k = [0 : 3]) let(d = dir_iman_cab(k)) poste_a_plato(cab(d * (Re + hueco_iman_ext)), d, iman_e + 3, iman_d + 4);
            // copas de las bolas
            for (a = COPAS_AZ) let(d = dir_copa(a)) poste_a_plato(cab(d * (Re + bolita_d / 2)) - lip * d, d, 10, bolita_d + 6);
            // abrazaderas del power bank (se abre por abajo)
            for (x = [-30, 30]) translate([x, -66, plato_z - 14]) difference() {
                union() {
                    rotate([0, 90, 0]) cylinder(d = powerbank[0] + 6, h = 12, center = true);
                    translate([-6, -(powerbank[0] + 6) / 2, 0]) cube([12, powerbank[0] + 6, 14]);
                }
                rotate([0, 90, 0]) cylinder(d = powerbank[0] + 0.6, h = 14, center = true);
                translate([-7, -powerbank[0] * 0.35, -30]) cube([14, powerbank[0] * 0.7, 30]);
            }
            // soporte del VL53L0X detrás del ojo chico
            soporte_tof();
        }
        // imanes (cara S avellanada hacia el casco) con M4 y tuerca arriba del plato
        for (k = [0 : 3]) let(d = dir_iman_cab(k), p = cab(d * (Re + hueco_iman_ext))) {
            translate(p) apuntar(d) {
                translate([0, 0, -1]) cylinder(d = iman_d + 0.4, h = iman_e + 1, $fn = 64);
                cylinder(d = 4.5, h = 60, $fn = 16);
            }
            translate([p[0], p[1], plato_sup - 3.2]) rotate([0, 0, 30]) cylinder(d = 8.1, h = 10, $fn = 6);
        }
        // copas: la bola entra por arriba y la detiene un tapón
        for (a = COPAS_AZ) let(d = dir_copa(a), c = cab(d * (Re + bolita_d / 2))) {
            translate(c) apuntar(d) {
                cylinder(d = bolita_d + 0.4, h = 60, $fn = 48);
                translate([0, 0, -lip - 0.01]) cylinder(d1 = 0.85 * bolita_d, d2 = bolita_d + 0.4, h = lip + 0.02, $fn = 48);
            }
        }
        // tornillos M3 radiales del casco con tuerca atrapada
        for (a = TORNILLOS_AZ) rotate([0, 0, a]) translate([0, 0, plato_z + plato_e / 2]) {
            rotate([-90, 0, 0]) translate([0, 0, plato_r - 14]) cylinder(d = 3.4, h = 20, $fn = 16);
            translate([0, plato_r - 9, 0]) cube([6, 2.8, 20], center = true);
        }
        // pasacables y agujeros de los soportes y de la Pi Zero
        translate([0, -45, plato_z - 1]) cube([14, 9, 20], center = true);
        for (p = agujeros_pizero()) translate([p[0], p[1], plato_z - 1]) cylinder(d = 2.8, h = 10, $fn = 16);
        for (p = concat(agujeros_soporte_ojo(), agujeros_soporte_bocina())) translate([p[0], p[1], plato_z - 1]) {
            cylinder(d = 3.4, h = 10, $fn = 16);
            rotate([0, 0, 30]) cylinder(d = 6.4, h = 3.6, $fn = 6);
        }
        // aligerar entre los postes
        for (a = [0 : 60 : 300]) rotate([0, 0, a + 30]) translate([62, 0, plato_z - 1]) cylinder(d = 16, h = 10, $fn = 32);
    }
}

POS_PIZERO = [-32.5, -18];
function agujeros_pizero() = [for (p = [[3.5, 3.5], [61.5, 3.5], [3.5, 26.5], [61.5, 26.5]]) POS_PIZERO + p];

module soporte_tof() {
    // placa vertical que cuelga del plato con un bolsillo para el módulo
    q = p_ojo2 - (cab_pared + 2.2) * n_ojo2;
    h = unitario([q[0], q[1]]) * (plato_r - 22);
    intersection() {
    hueco_interior();
    difference() {
        hull() {
            translate(q) apuntar(n_ojo2) translate([0, 0, -1.5 - tof[2]]) cube([tof[1] + 6, tof[0] + 6, 3], center = true);
            translate([h[0] - 10, h[1] - 10, plato_z]) cube([20, 20, 1]);
        }
        translate(q) apuntar(n_ojo2) {
            translate([-tof[1] / 2 - 0.3, -tof[0] / 2 - 0.3, -tof[2] - 0.2]) cube([tof[1] + 0.6, tof[0] + 0.6, 10]);
            translate([-6, -6, -20]) cube([12, 12, 21]);            // conector y cables por detrás
        }
    }
    }
}

// Volumen libre dentro del casco (con 0.6 mm de holgura): ningún soporte se sale de aquí.
module hueco_interior() rotate_extrude($fn = 180) perfil(cab_pared + 0.6);

// ───────────── Soporte del ojo: mica Ø50 + anillo WS2812 + OV5647 ─────────────
// Todo en el marco del ojo: z sobre n_ojo, origen en la cara interior del casco.
mica_sag = pow(ojo_mica_d / 2, 2) / (2 * (Rd - cab_pared));
z_mica = -mica_sag - 3;                 // espalda de la mica de 3 mm
z_anillo = z_mica - 0.4 - 1.6 - anillo[2];   // espalda del PCB del anillo
z_disco = z_anillo - 2.5;
O_OJO = [0, 0, zc] + (Rd - cab_pared) * n_ojo;
function agujeros_soporte_ojo() = [[-14, O_OJO[1] - 40], [14, O_OJO[1] - 40]];

module en_ojo() translate(O_OJO) apuntar(n_ojo) rotate([0, 0, 90]) children();

module soporte_ojo() intersection() {
    hueco_interior();
    difference() {
        union() {
            en_ojo() translate([0, 0, z_disco]) cylinder(d = ojo_mica_d + 2, h = z_anillo - z_disco, $fn = 96);
            // pestañas que aprietan la mica contra el casco
            en_ojo() for (k = [0 : 3]) rotate([0, 0, 45 + 90 * k]) translate([anillo[0] / 2 + 0.6, -4, z_anillo]) cube([2.4, 8, z_mica - z_anillo]);
            // columna al plato
            hull() {
                en_ojo() translate([0, -ojo_mica_d / 2 + 4, z_disco]) rotate([0, 0, 0]) cylinder(d = 24, h = 2.5);
                translate([-20, O_OJO[1] - 48, plato_sup]) cube([40, 16, 4]);
            }
        }
        // la lente pasa por el disco; la cámara va por detrás en su bolsillo
        en_ojo() {
            translate([-camara_lente / 2 - 0.4, -camara_lente / 2 - 0.4, z_disco - 1]) cube([camara_lente + 0.8, camara_lente + 0.8, 10]);
            translate([-camara[0] / 2 - 0.3, -camara[1] / 2 + 2.5 - 0.3, z_disco - 8]) cube([camara[0] + 0.6, camara[1] + 0.6, 8 + 0.01]);
            // cables del anillo
            translate([0, anillo[1] / 2 - 1, z_disco - 1]) cylinder(d = 6, h = 10, $fn = 16);
        }
        for (p = agujeros_soporte_ojo()) translate([p[0], p[1], plato_sup - 1]) cylinder(d = 3.4, h = 20, $fn = 16);
        translate([0, 0, plato_sup - 50]) cylinder(r = 200, h = 50);   // nada debajo del plato
    }
}

// ───────────── Soporte de la bocina ─────────────
O_BOC = [0, 0, zc] + (Rd - cab_pared - 2) * n_boc;
function agujeros_soporte_bocina() = [[-14, O_BOC[1] + 34], [14, O_BOC[1] + 34]];
module en_bocina() translate(O_BOC) apuntar(n_boc) children();

module soporte_bocina() intersection() {
    hueco_interior();
    difference() {
        union() {
            en_bocina() translate([0, 0, -3]) cylinder(d = bocina_d + 8, h = 3, $fn = 96);
            hull() {
                en_bocina() translate([0, bocina_d / 2, -3]) cube([30, 6, 3], center = true);
                translate([-20, O_BOC[1] + 26, plato_sup]) cube([40, 16, 4]);
            }
        }
        en_bocina() {
            translate([0, 0, -10]) cylinder(d = bocina_d - 4, h = 20, $fn = 96);
            translate([0, 0, -bocina_prof - 40]) cylinder(d = bocina_d + 1, h = bocina_prof + 37, $fn = 96);
            for (k = [0 : 3]) rotate([0, 0, 45 + 90 * k]) translate([bocina_d / 2 + 1.5, 0, -10]) cylinder(d = 2.2, h = 20, $fn = 12);
        }
        for (p = agujeros_soporte_bocina()) translate([p[0], p[1], plato_sup - 1]) cylinder(d = 3.4, h = 20, $fn = 16);
        translate([0, 0, plato_sup - 50]) cylinder(r = 200, h = 50);
    }
}

// Tapón a presión que retiene cada bola desde arriba.
module tapon() difference() {
    cylinder(d = bolita_d + 0.2, h = 8, $fn = 48);
    translate([0, 0, -1]) cylinder(d = 3, h = 10, $fn = 12);    // grasa / sacarlo con un clavo
}

// ───────────── Vista ─────────────
module cabeza_completa(con_casco = true) {
    if (con_casco) color("white") casco();
    color("orange") plato();
    color("orange") soporte_ojo();
    color("orange") soporte_bocina();
    for (k = [0 : 3]) let(d = dir_iman_cab(k)) translate(cab(d * (Re + hueco_iman_ext))) apuntar(d) rotate([180, 0, 0]) iman();
    for (a = COPAS_AZ) let(d = dir_copa(a)) color("silver") translate(cab(d * (Re + bolita_d / 2))) sphere(d = bolita_d, $fn = 32);
    translate([0, -66, plato_z - 14]) power_bank();
    translate([POS_PIZERO[0], POS_PIZERO[1], plato_sup + 5]) pi_zero();
    translate([28, -52, plato_sup + 1]) pam8403();
    en_ojo() {
        translate([0, 0, z_anillo]) anillo_ws2812();
        translate([0, 0, z_disco]) camara_ov5647();
        color("#334", 0.6) translate([0, 0, z_mica]) cylinder(d = ojo_mica_d, h = 3);
    }
    translate(p_ojo2 - (cab_pared + 2.2) * n_ojo2) apuntar(n_ojo2) rotate([0, 0, 90]) vl53l0x();
    en_bocina() bocina();
}

if (parte == "vista") cabeza_completa();
else if (parte == "casco") casco();
else if (parte == "plato") rotate([180, 0, 0]) plato();
else if (parte == "tapon") tapon();
else if (parte == "soporte_ojo") soporte_ojo();
else if (parte == "soporte_bocina") soporte_bocina();
