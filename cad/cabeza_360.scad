// BB-8 · cabeza que gira 360° (Parte 5): explicación, no pieza para imprimir.
//
// El MG996R pasa a rotación continua (sin potenciómetro) y un AS5600 (sensor
// magnético de ángulo absoluto) ocupa su lugar: un imán diametral en la punta del
// eje de salida y la placa fija debajo de la tapa del servo, centrada en el eje.
// El Arduino lee el ángulo por I2C (A4/A5, el mismo bus del MPU6050 y el VL53L0X)
// y mueve el servo hasta llegar al ángulo pedido.
//
// Las piezas internas del servo son aproximadas: al abrirlo, MIDE dónde queda la
// punta del eje y si la placa de control estorba. Si el imán queda a más de 3 mm
// del chip, el prolongador impreso lo baja.
//
//   openscad cabeza_360.scad                       (vista armada)
//   openscad -D explotar=1 cabeza_360.scad         (despiece con etiquetas)

include <parametros.scad>
use <componentes.scad>

explotar = 1;     // 0 armado, 1 despiece
etiquetas = true;

e = explotar;
z_porta = servo_alto + 6 + 14 * e;     // portaimanes sobre el cuerno
z_placa = -2.5 - 16 * e;               // AS5600 bajo la tapa
z_iman  = 0.6 - 7 * e;                 // imán en la punta del prolongador
z_tubo  = -6 - 22 * e;                 // tope del tubo del poste
z_ard   = -95 - 25 * e;

// ---------- piezas ----------
module servo_transparente() {
    // caja: solo las aristas, para ver adentro
    color("#444") translate([-servo_eje_x - servo[0] / 2, -servo[1] / 2, 0]) aristas([servo[0], servo[1], servo_caja_alto]);
    color("#444") translate([-servo_eje_x - servo_oreja_L / 2, -servo[1] / 2, servo_oreja_z]) aristas([servo_oreja_L, servo[1], servo_oreja_e]);
    // eje de salida y engrane
    color("silver") translate([0, 0, 20]) cylinder(d = 5.8, h = servo_alto - 20, $fn = 32);
    color("ivory") translate([0, 0, 26]) cylinder(d = 17, h = 4, $fn = 48);
    color("ivory") translate([-9, 0, 22]) cylinder(d = 12, h = 3, $fn = 48);
    // motor (en el otro extremo)
    color("#b0b0b0") translate([-servo_eje_x - servo[0] / 2 + 9, 0, 5]) cylinder(d = 15, h = 24, $fn = 40);
    // placa de control abajo, con hueco donde pasaba el potenciómetro
    color("darkgreen") translate([0, 0, 2]) difference() {
        translate([-servo_eje_x - servo[0] / 2 + 2, -servo[1] / 2 + 1.5, 0]) cube([servo[0] - 4, servo[1] - 3, 1.2]);
        translate([0, 0, -1]) cylinder(d = 7, h = 4, $fn = 32);
    }
}

module aristas(t, g = 0.7) for (a = [0 : 2]) for (u = [0, 1], v = [0, 1]) {
    o = a == 0 ? [0, u * t[1], v * t[2]] : a == 1 ? [u * t[0], 0, v * t[2]] : [u * t[0], v * t[1], 0];
    translate(o - [g, g, g] / 2) cube(a == 0 ? [t[0] + g, g, g] : a == 1 ? [g, t[1] + g, g] : [g, g, t[2] + g]);
}

module potenciometro() color("gold") {
    cylinder(d = 10, h = 7, $fn = 32);
    translate([0, 0, 7]) cylinder(d = 4, h = 4, $fn = 16);
    for (i = [-1 : 1]) translate([i * 2.5, -4, -5]) cylinder(d = 0.8, h = 5, $fn = 8);
}

module tache(t = 16) color("red") for (a = [45, -45]) rotate([0, a, 0]) cube([t, 1.4, 1.4], center = true);

module prolongador() color("white") {   // impreso: entra en el eje donde iba el potenciómetro
    translate([0, 0, 1.2]) cylinder(d = 4.5, h = 19, $fn = 24);
    translate([0, 0, 0]) cylinder(d = 7, h = 1.2, $fn = 32);
}

module iman_diametral() {   // 6 × 2.5 mm, magnetizado a lo ancho: mitad N, mitad S
    color("crimson") intersection() { cylinder(d = 6, h = 2.5, $fn = 40); translate([0, -5, -1]) cube([10, 10, 5]); }
    color("royalblue") intersection() { cylinder(d = 6, h = 2.5, $fn = 40); translate([-10, -5, -1]) cube([10, 10, 5]); }
}

module as5600() {
    color("#3949ab") translate([-11.5, -11.5, -1.6]) difference() {
        cube([23, 23, 1.6]);
        for (x = [2.5, 20.5], y = [2.5, 20.5]) translate([x, y, -1]) cylinder(d = 2.2, h = 4, $fn = 12);
    }
    color("#111") translate([-2.5, -2, 0]) cube([5, 4, 1.4]);                      // el chip, centrado en el eje
    for (i = [0 : 6]) color("gold") translate([-9 + i * 3, -11, -1.6 - 6]) cube([0.64, 0.64, 6]);   // 7 pines hacia abajo
}

module tubo() color("lightgray") translate([0, 0, z_tubo - 50]) difference() {   // cortado a la mitad
    cylinder(d = tubo_d, h = 50, $fn = 64);
    translate([0, 0, -1]) cylinder(d = tubo_d - 3, h = 52, $fn = 64);
    translate([-tubo_d, -2 * tubo_d, -1]) cube([2 * tubo_d, 2 * tubo_d, 52]);
}

module portaimanes_simple() {
    color("orange") difference() {
        cylinder(d = porta_d, h = porta_e, $fn = 96);
        translate([0, 0, -1]) cylinder(d = 7, h = porta_e + 2, $fn = 24);
    }
    for (k = [0 : 3]) rotate([0, 0, 45 + 90 * k]) translate([iman_r, 0, porta_e]) color("gray") cylinder(d = iman_d, h = 1, $fn = 48);
}

// flecha circular: gira sin tope
module flecha_360(r = 46, z = 0) color("orangered") translate([0, 0, z]) {
    rotate_extrude(angle = 300, $fn = 96) translate([r, 0]) circle(d = 2.2, $fn = 12);
    rotate([0, 0, 300]) translate([r, 0, 0]) rotate([-90, 0, 0]) cylinder(d1 = 6, d2 = 0, h = 9, $fn = 20);
}

// cable entre dos puntos (polilínea)
module cable(pts, c) color(c) for (i = [0 : len(pts) - 2]) hull() {
    translate(pts[i]) sphere(d = 1.6, $fn = 10);
    translate(pts[i + 1]) sphere(d = 1.6, $fn = 10);
}

module arduino() {
    color("#00838f") translate([-34, -27, -2]) cube([68, 54, 2]);
    color("#222") translate([-30, 20, 0]) cube([60, 4, 8]);   // tira de pines de abajo (A0…A5, 5V, GND)
}

// ---------- etiquetas (en el plano XZ, de frente a la cámara) ----------
module etiqueta(txt, p, desde, s = 3.6) if (etiquetas) {
    color("#222") translate(p) rotate([90, 0, 0]) linear_extrude(0.5) text(txt, size = s, font = "Liberation Sans:style=Bold", valign = "center",
        halign = p[0] < 0 ? "right" : "left");
    color("#888") hull() { translate(desde) sphere(d = 0.8, $fn = 8); translate(p + [p[0] < 0 ? 2 : -2, 0, 0]) sphere(d = 0.8, $fn = 8); }
}

// ---------- escena ----------
translate([0, 0, z_porta]) portaimanes_simple();
flecha_360(z = z_porta + porta_e + 4);
color("white") translate([0, 0, servo_alto - 2]) cylinder(d = cuerno_d, h = cuerno_e, $fn = 48);
servo_transparente();
// potenciómetro: se quita (en el despiece, afuera con un tache)
translate([e * 52, 0, 9]) { potenciometro(); if (e > 0) translate([0, 0, 4]) tache(); }
translate([0, 0, z_iman + 2.5 + 0.1]) prolongador();
translate([0, 0, z_iman]) iman_diametral();
translate([0, 0, z_placa]) as5600();
tubo();
translate([0, 0, z_ard]) arduino();

// los 4 cables (+ DIR a GND) bajan por el tubo hasta la tira de pines del Arduino
pin = function(i) [-9 + i * 3 + 0.3, -10.7, z_placa - 7.6];
cables = [[0, "red", 12], [1, "black", 6], [2, "#7b1fa2", 6], [3, "gold", -8], [4, "limegreen", -14]];  // VCC GND DIR SCL SDA
for (cb = cables) let(p = pin(cb[0]))
    cable([p, [p[0] * 0.4, -4, z_tubo - 10], [p[0] * 0.4, -4, z_ard + 30], [cb[2], 22, z_ard + 9]], cb[1]);
// DIR se une a GND en la placa
cable([pin(2), pin(2) + [0, 0, -3], pin(1) + [0, 0, -3], pin(1)], "#7b1fa2");

if (e > 0) {
    x = 44;
    etiqueta("Portaimanes (arrastra la cabeza)", [x, 0, z_porta + 6], [porta_d / 2, 0, z_porta + 6]);
    etiqueta("360° sin tope", [-x - 6, 0, z_porta + porta_e + 4], [-46, 0, z_porta + porta_e + 4]);
    etiqueta("MG996R, rotación continua", [x, 0, 30], [servo[0] / 2 - servo_eje_x, 0, 30]);
        etiqueta("Potenciómetro: se quita", [62, 0, 2], [52, 0, 6]);
    etiqueta("Prolongador impreso", [-x, 0, z_iman + 12], [-2.5, 0, z_iman + 12]);
    etiqueta("Imán 6 × 2.5 (gira)", [-x, 0, z_iman + 1], [-3, 0, z_iman + 1]);
    etiqueta("AS5600 (fijo, centrado)", [x, 0, z_placa - 1], [11.5, 0, z_placa - 1]);
    etiqueta("imán a 0.5–3 mm del chip", [x, 0, z_placa - 6], [x, 0, z_placa - 6], 3);
    etiqueta("Cables por el tubo", [-x, 0, z_tubo - 20], [-tubo_d / 2, 0, z_tubo - 20]);
    for (i = [0 : 4]) let(l = ["VCC → 5V", "GND → GND", "DIR → GND", "SCL → A5", "SDA → A4"][i], c = ["red", "black", "#7b1fa2", "gold", "limegreen"][i])
        translate([-x, 0, z_ard + 34 - 6 * i]) { color(c) translate([-1, 0, 0]) rotate([90, 0, 0]) cylinder(d = 3, h = 0.6, $fn = 16);
            etiqueta(l, [-4, 0, 0], [-4, 0, 0], 3.4); }
    etiqueta("Arduino Uno (servo en D9)", [x, 0, z_ard], [34, 0, z_ard]);
}
