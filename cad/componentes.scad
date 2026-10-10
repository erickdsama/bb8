// BB-8 · maquetas de los componentes comprados (no se imprimen).
// Sirven para el ensamble y para comprobar que todo cabe. Medidas de las hojas de los
// vendedores; las dudosas están marcadas MEDIR en parametros.scad.

include <parametros.scad>

// JGB37-520 con encoder. Eje de salida en el origen apuntando a +x; la cara de montaje
// de la caja en x = 0. La caja y el motor van desplazados `excentrico` en dir (unitario en yz).
module jgb37(dir = [0, 1, 0]) {
    e = excentrico * dir;
    color("silver") translate(e) rotate([0, -90, 0]) cylinder(d = caja_d, h = caja_L);
    color("dimgray") translate(e + [-caja_L, 0, 0]) rotate([0, -90, 0]) cylinder(d = motor_d - 2, h = motor_L - 17);
    color("black") translate(e + [-caja_L - motor_L + 17, 0, 0]) rotate([0, -90, 0]) cylinder(d = motor_d, h = 17);
    color("silver") rotate([0, 90, 0]) cylinder(d = buje_d, h = buje_alto);
    color("silver") rotate([0, 90, 0]) difference() {
        cylinder(d = eje_d, h = eje_L);
        translate([eje_d / 2 - 0.5, -5, 3]) cube([2, 10, eje_L]);
    }
}

// Rueda de 65 mm con cople hexagonal; centrada en el origen, eje x.
module rueda() {
    color("#222") rotate([0, 90, 0]) difference() {
        hull() {
            cylinder(d = rueda_d - 2 * chaflan_llanta, h = rueda_ancho, center = true);
            cylinder(d = rueda_d, h = rueda_ancho - 2 * chaflan_llanta, center = true);
        }
        cylinder(d = rueda_d - 14, h = rueda_ancho + 1, center = true);
    }
    color("gold") rotate([0, 90, 0]) cylinder(d = rueda_d - 14, h = rueda_ancho - 6, center = true);
    color("silver") translate([-rueda_ancho / 2 - 3, 0, 0]) rotate([0, 90, 0]) cylinder(d = 12, h = 10, $fn = 6);
}

module pcb(x, y, c = "green") color(c) cube([x, y, 1.6]);

module arduino_uno() {
    pcb(68.6, 53.3, "teal");
    color("silver") translate([-6, 38, 1.6]) cube([16, 12, 11]);   // USB B
    color("black") translate([-2, 3, 1.6]) cube([14, 9, 11]);      // jack
    color("black") translate([27, 49, 1.6]) cube([36, 2.5, 8]);    // headers
    color("black") translate([18, 1, 1.6]) cube([46, 2.5, 8]);
}

module raspberry_pi4() {
    pcb(85, 56, "green");
    color("silver") translate([67, 2, 1.6]) cube([21, 16, 13.5]);    // ethernet
    color("silver") translate([70, 21, 1.6]) cube([17, 13, 16]);     // USB
    color("silver") translate([70, 39, 1.6]) cube([17, 13, 16]);
    color("black") translate([7, 50, 1.6]) cube([51, 5, 8.5]);       // GPIO
}

module l298n() {
    pcb(43, 43, "red");
    color("black") translate([12, 26, 1.6]) cube([23, 16, 25]);      // disipador
    color("blue") translate([0, 2, 1.6]) cube([8, 15, 10]);          // borneras
    color("blue") translate([35, 2, 1.6]) cube([8, 15, 10]);
    color("blue") translate([12, 0, 1.6]) cube([15, 8, 10]);
}

module lm2596() {
    pcb(43, 21, "blue");
    color("dimgray") translate([16, 5, 1.6]) cube([12, 12, 7]);
    color("gold") translate([30, 3, 1.6]) cube([9.5, 4.5, 10]);
}

module mpu6050() pcb(21, 16, "navy");

module lipo() color("#335") translate([-lipo[0] / 2, -lipo[1] / 2, 0]) cube(lipo);

// Ball transfer 1″: origen al centro de la cara de la brida, la bola sale hacia +z.
module ball_transfer() {
    color("silver") translate([0, 0, -bt_brida_e]) difference() {
        cylinder(d = bt_brida_d, h = bt_brida_e);
        for (k = [0 : 2]) rotate([0, 0, 90 + 120 * k]) translate([bt_pcd / 2, 0, -1]) cylinder(d = 5, h = 5);
    }
    color("silver") translate([0, 0, -bt_brida_e - bt_cuerpo_L]) cylinder(d = bt_cuerpo_d, h = bt_cuerpo_L);
    color("silver") cylinder(d1 = bola_d + 6, d2 = bola_d + 1, h = bt_bola_fuera - bola_d / 2);
    color("lightsteelblue") translate([0, 0, bt_bola_fuera - bola_d / 2]) sphere(d = bola_d);
}

// MG996R: origen en el eje de salida, a la altura de la base del servo.
module mg996r() {
    translate([-servo_eje_x, 0, 0]) {
        color("#111") translate([-servo[0] / 2, -servo[1] / 2, 0]) cube([servo[0], servo[1], servo_caja_alto]);
        color("#111") translate([-servo_oreja_L / 2, -servo[1] / 2, servo_oreja_z]) difference() {
            cube([servo_oreja_L, servo[1], servo_oreja_e]);
            for (sx = [-1, 1], sy = [-1, 1])
                translate([servo_oreja_L / 2 + sx * servo_agujeros[0] / 2, servo[1] / 2 + sy * servo_agujeros[1] / 2, -1])
                    cylinder(d = 4.2, h = 5);
        }
    }
    color("#111") cylinder(d = 13, h = servo_alto - 4);
    color("silver") cylinder(d = 5.8, h = servo_alto);
}

module cuerno() color("white") cylinder(d = cuerno_d, h = cuerno_e);

// Imán N42 avellanado; origen al centro de la cara avellanada (+z).
module iman() color("gray") translate([0, 0, -iman_e]) difference() {
    cylinder(d = iman_d, h = iman_e);
    translate([0, 0, -0.1]) cylinder(d = 4.5, h = iman_e + 1);
    translate([0, 0, iman_e - 2.2]) cylinder(d1 = 4.5, d2 = 8.6, h = 2.21);
}

module pi_zero() {
    pcb(65, 30, "green");
    color("silver") translate([10, -1, 1.6]) cube([8, 6, 3]);
    color("silver") translate([41, -1, 1.6]) cube([8, 6, 3]);
    color("silver") translate([54, -1, 1.6]) cube([8, 6, 3]);
}

// Cámara OV5647 tipo v1: origen al centro del lente, mirando a +z, PCB detrás.
module camara_ov5647() {
    color("green") translate([-camara[0] / 2, -camara[1] / 2 + 2.5, -camara[2]]) cube(camara);
    color("black") translate([-camara_lente / 2, -camara_lente / 2, 0]) cube([camara_lente, camara_lente, 4]);
    color("black") cylinder(d = 7, h = 6);
}

// Anillo WS2812 de 16 LED; origen al centro, LED mirando a +z.
module anillo_ws2812() {
    color("black") difference() {
        cylinder(d = anillo[0], h = anillo[2]);
        translate([0, 0, -1]) cylinder(d = anillo[1], h = 4);
    }
    for (k = [0 : 15]) rotate([0, 0, k * 360 / 16]) translate([(anillo[0] + anillo[1]) / 4, 0, anillo[2] + 0.8])
        color("white") cube([5, 5, 1.6], center = true);
}

module vl53l0x() {
    color("purple") translate([-tof[0] / 2, -tof[1] / 2, -tof[2]]) cube(tof);
    color("black") translate([-2.5, -1.5, 0]) cube([5, 3, 1.2]);
}

// Bocina: origen al centro del cono, sonando hacia +z.
module bocina() {
    color("#333") translate([0, 0, -bocina_prof]) cylinder(d1 = bocina_d * 0.6, d2 = bocina_d, h = bocina_prof);
    color("#555") translate([0, 0, -1]) cylinder(d = bocina_d - 4, h = 1);
}

module power_bank() color("#2a6") rotate([0, 90, 0]) cylinder(d = powerbank[0], h = powerbank[1], center = true);

module pam8403() {
    pcb(30, 21, "darkred");
    color("blue") translate([22, 5, 1.6]) cylinder(d = 9, h = 7);
}
