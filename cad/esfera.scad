// BB-8 · esfera (Parte 6).
//
// El casco se arma como el del BB-8: 6 círculos naranjas centrados en las caras de un
// cubo y 8 gajos blancos, uno por octante. La junta de las dos mitades es el gran círculo
// perpendicular a la diagonal (1,1,1) del cubo: pasa entre los círculos (a 5° de cada
// uno), así que ningún círculo queda partido.
//
// Por dentro el casco es liso: ruedas y ball transfers pasan por todas partes, así que
// nada sobresale hacia adentro. Por eso:
//   - los círculos son tapas de 2 mm pegadas en un asiento de 2 mm (piso de 2 mm debajo);
//   - la junta entre mitades es un traslape a media pared con bayoneta de 6 lengüetas
//     que viven dentro del grosor de la pared (se cierra girando 7°);
//   - los gajos de una misma mitad se pegan a tope con espigas de filamento de 1.75 mm.
// Un círculo (el del eje −z del patrón) es la tapa de carga: el piso tiene un hueco y la
// tapa se atornilla a la ceja con 4 tornillos M2 × 4 avellanados.
//
// Marco del patrón: ejes del cubo. La mitad A es la que queda del lado +(1,1,1).
// Uso: openscad -D 'parte="A_ppp"' -o A_ppp.stl esfera.scad
//   partes: A_ppp A_ppm A_pmp A_mpp A_pmm A_mpm A_mmp
//           B_ppm B_pmp B_mpp B_pmm B_mpm B_mmp B_mmm
//           circulo  tapa_carga  vista

include <parametros.scad>

parte = "vista";

ALFA = acos(1 / sqrt(3));   // ángulo entre (1,1,1) y cualquier eje
B = Re + 10;                 // "infinito" para cortes
EJES = [[1, 0, 0], [0, 1, 0], [0, 0, 1], [-1, 0, 0], [0, -1, 0], [0, 0, -1]];
EJE_CARGA = [0, 0, -1];
ang_ceja = ceja_tapa / Re * 180 / PI;
ang_holg = holg / Re * 180 / PI;

// Del marco de la junta (eje z = (1,1,1)) al marco del patrón.
module marco_junta() rotate(a = -ALFA, v = [1, -1, 0]) children();

// Anillo esférico entre radios r1–r2 y alturas z0–z1, de un ángulo dado.
module anillo_esf(r1, r2, z0, z1, ang = 360) {
    n = 16;
    rotate_extrude(angle = ang, $fn = $fn * 3)
        polygon(concat(
            [for (i = [0 : n]) let(z = z0 + (z1 - z0) * i / n) [sqrt(r2 * r2 - z * z), z]],
            [for (i = [n : -1 : 0]) let(z = z0 + (z1 - z0) * i / n) [sqrt(r1 * r1 - z * z), z]]));
}

module losa(z0, z1) translate([-B, -B, z0]) cube([2 * B, 2 * B, z1 - z0]);

module octante(s) scale(s) cube(B);

// ── Gajo blanco completo (todavía sin partir) ──
// ¿El punto p cae en el octante s (o en su borde)? Sirve para no restar de más.
function en_octante(p, s) = s == undef || (p[0] * s[0] >= -1 && p[1] * s[1] >= -1 && p[2] * s[2] >= -1);

module espigas(s) {
    r = (Ri + Re - rebaje) / 2;
    for (f = [45, 125, 145, 225, 305, 325]) {
        c = cos(f); n = sin(f);
        if (en_octante(r * [0, c, n], s))
            translate(r * [0, c, n]) rotate([0, 90, 0]) cylinder(d = espiga_d, h = 14, center = true, $fn = 12);
        if (en_octante(r * [n, 0, c], s))
            translate(r * [n, 0, c]) rotate([90, 0, 0]) cylinder(d = espiga_d, h = 14, center = true, $fn = 12);
        if (en_octante(r * [c, n, 0], s))
            translate(r * [c, n, 0]) cylinder(d = espiga_d, h = 14, center = true, $fn = 12);
    }
}

// Agujeros piloto ciegos en la ceja para los tornillos de la tapa de carga.
module tornillos_tapa() {
    a = circulo_ang - ang_ceja / 2;
    apuntar(EJE_CARGA) for (k = [0 : 3]) rotate([0, 0, 45 + 90 * k]) rotate([0, a, 0])
        translate([0, 0, Ri + 0.4]) cylinder(d = 1.6, h = Re - Ri, $fn = 12);
}

// s = octante [±1, ±1, ±1] para recortar desde el principio (CGAL va mucho más rápido),
// o undef para la esfera entera.
module blanco(s) {
    difference() {
        intersection() {
            difference() { sphere(Re, $fn = $fn * 3); sphere(Ri, $fn = $fn * 3); }
            if (s != undef) octante(s);
        }
        // asientos de los círculos
        for (e = EJES) if (en_octante(e, s)) apuntar(e) casquete(Re - rebaje, Re + 1, circulo_ang);
        // hueco de la tapa de carga (deja la ceja)
        if (en_octante(EJE_CARGA, s)) {
            apuntar(EJE_CARGA) cono(circulo_ang - ang_ceja, Re + 2);
            tornillos_tapa();
        }
        espigas(s);
    }
}

// ── Mitades con traslape y bayoneta (en el marco de la junta) ──
ang_leng = 8 / Ri * 180 / PI;    // lengüetas de 8 mm de ancho
zl = -traslape / 2;              // altura de las lengüetas
r_lap = Ri + t_int;              // radio de la superficie del traslape

module lenguetas() {
    for (k = [0 : lengueta_n - 1]) rotate([0, 0, 30 + 360 / lengueta_n * k])
        anillo_esf(r_lap - 0.2, r_lap + lengueta_alto, zl - 1.5, zl + 1.5, ang_leng);
}

module ranuras() {
    rr = r_lap + lengueta_alto + 0.25;
    for (k = [0 : lengueta_n - 1]) rotate([0, 0, 30 + 360 / lengueta_n * k]) {
        // entrada desde el borde de la mitad A
        rotate([0, 0, -0.6]) anillo_esf(r_lap - 0.1, rr, -traslape - 1, zl + 1.8, ang_leng + 1.2);
        // canal de giro
        rotate([0, 0, -0.6]) anillo_esf(r_lap - 0.1, rr, zl - 1.8, zl + 1.8, ang_leng + lengueta_giro + 1.2);
    }
}

module mitad_A(s) {
    difference() {
        intersection() {
            blanco(s);
            marco_junta() union() {
                losa(0, B);
                anillo_esf(r_lap + holg / 2, Re + 1, -traslape, 0);
            }
        }
        marco_junta() ranuras();
    }
}

module mitad_B(s) {
    intersection() {
        blanco(s);
        marco_junta() union() {
            losa(-B, -traslape - 0.2);
            anillo_esf(Ri - 1, r_lap, -traslape - 0.2, -0.2);
        }
    }
    intersection() {
        marco_junta() lenguetas();
        if (s != undef) octante(s);
        // no pongas lengüetas debajo de los asientos de los círculos
        difference() { sphere(Re, $fn = $fn * 3); for (e = EJES) apuntar(e) cono(circulo_ang + 2, Re + 2); }
    }
}

// ── Círculos naranjas ──
module circulo() casquete(Re - rebaje + 0.15, Re, circulo_ang - ang_holg);

module tapa_carga() {
    difference() {
        circulo();
        // tornillos avellanados (cabeza 3.8 mm) y una muesca para la uña
        a = circulo_ang - ang_ceja / 2;
        for (k = [0 : 3]) rotate([0, 0, 45 + 90 * k]) rotate([0, a, 0]) {
            translate([0, 0, Re - rebaje - 1]) cylinder(d = 2.3, h = rebaje + 2, $fn = 12);
            translate([0, 0, Re - 1.1]) cylinder(d1 = 2.3, d2 = 4.3, h = 1.11, $fn = 16);
        }
        rotate([0, circulo_ang - 1.2, 0]) translate([0, 0, Re]) cube([6, 12, 2.5], center = true);
    }
}

// Pieza en la orientación del patrón: octante s de la mitad m.
module pieza(m, s) {
    if (m == "A") mitad_A(s); else mitad_B(s);
}

function signos(t) = [for (i = [0 : 2]) t[i] == "p" ? 1 : -1];

// Para imprimir: la cara del corte por un plano del patrón va en la cama y el gajo sube
// como domo (165 × 165 × 165 mm, cabe en una cama de 220). Los soportes quedan por dentro.
module para_imprimir(m, s) {
    if (s[2] > 0) pieza(m, s); else rotate([180, 0, 0]) pieza(m, s);
}

module esfera_completa(explosion = 0) {
    color("white") mitad_A();
    color("white") mitad_B();
    for (e = EJES) color("darkorange") apuntar(e) translate([0, 0, explosion])
        if (e == EJE_CARGA) tapa_carga(); else circulo();
}

if (parte == "vista") esfera_completa();
else if (parte == "circulo") circulo();
else if (parte == "tapa_carga") tapa_carga();
else {
    m = parte[0];
    s = signos([parte[2], parte[3], parte[4]]);
    para_imprimir(m, s);
}
