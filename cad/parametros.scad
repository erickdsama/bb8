// BB-8 · parámetros compartidos de todas las piezas.
//
// Todas las medidas en mm. Marco del robot (ensamble en reposo):
//   origen = centro de la esfera, +z arriba, +y adelante, +x derecha (eje de las ruedas).
// Cambia un número aquí y vuelve a correr cad/exportar.sh: todo lo demás se recalcula.
// Las medidas marcadas MEDIR vienen de hojas de vendedores que no coinciden entre sí;
// mide tu pieza con vernier antes de imprimir lo que depende de ellas.

$fn = 48;          // exportar.sh lo sube para los STL finales
holg = 0.3;        // holgura general de impresión (FDM 0.4 mm)

// ───────────────────────── Esfera ─────────────────────────
esfera_d   = 330;              // 30–35 cm
pared      = 4;                // casco PLA, todo perímetros
Re         = esfera_d / 2;     // radio exterior
Ri         = Re - pared;       // radio interior: por aquí ruedan ruedas y ball transfers
circulo_ang = 30;              // radio angular de los 6 círculos naranjas (grados)
rebaje     = 2;                // los círculos son tapas de 2 mm en un asiento de 2 mm
t_int      = 1.8;              // traslape: capa interior (mitad B)
traslape   = 10;               // ancho del traslape de la junta entre mitades
lengueta_n = 6;                // lengüetas de la bayoneta (múltiplo de 3 por simetría)
lengueta_alto = 0.6;           // cuánto sobresale cada lengüeta
lengueta_giro = 7;             // grados que se gira para cerrar
espiga_d   = 1.9;              // agujero para espiga de filamento 1.75 entre gajos
ceja_tapa  = 8;                // ceja que sostiene la tapa de carga (mm de arco)

// ───────────────────────── Motores JGB37-520B (encoder, 319 RPM) ─────────────────────────
caja_d     = 37;     // diámetro de la caja de engranes
caja_L     = 22;     // MEDIR: largo de la caja; 19–29 según la reducción
motor_d    = 35;     // motor 520 + disco del encoder
motor_L    = 50;     // MEDIR: motor + encoder, sin la caja
excentrico = 7;      // el eje de salida va 7 mm fuera del centro de la caja
eje_d      = 6;      // eje D de 6 mm (5.5 entre planos)
eje_L      = 15.5;
buje_d     = 12;     // buje de centrado de la caja
buje_alto  = 6;
caja_pcd   = 31;     // 6 × M3 en un círculo de 31 mm (centrado en la caja)

// ───────────────────────── Ruedas ─────────────────────────
rueda_d       = 65;
rueda_ancho   = 26;
rueda_offset  = 20;    // cara de la caja → plano medio de la rueda: soporte 4 mm + buje + cople hex
hueco_motores = 4;     // separación entre los dos motores, al centro
chaflan_llanta = 2;    // el borde exterior de la llanta (forrada) es redondeado

// Plano medio de cada rueda (distancia al centro en x): los dos motores coaxiales
// llenan el ancho, así que no se elige, sale de las piezas.
a_rueda = rueda_offset + caja_L + motor_L + hueco_motores / 2;
// La rueda es vertical; toca la esfera con su borde exterior.
x_contacto = a_rueda + rueda_ancho / 2 - chaflan_llanta;
z_eje = -(sqrt(Ri * Ri - x_contacto * x_contacto) - rueda_d / 2);
angulo_contacto = asin(x_contacto / Ri);   // inclinación de la pared donde pisa la rueda

// ───────────────────────── Plataforma (melamina) ─────────────────────────
plat_d      = 210;    // 20–22 cm
plat_e      = 15;     // melamina 15 mm
cuna_techo  = 3;      // material del soporte de motor encima de la caja
plat_z_inf  = z_eje + caja_d / 2 + cuna_techo;   // cara de abajo de la plataforma
plat_z_sup  = plat_z_inf + plat_e;               // cara de arriba

// ───────────────────────── Batería y lastre ─────────────────────────
lipo   = [47, 139, 25];   // LiPo 3S 5200 mAh XT60 (Hilldow): x, y, z (va a lo largo de y)
charola_piso = 3;
charola_pared = 4;
lastre_ancho = 34;        // compartimiento de lastre a cada lado de la LiPo (x)
lastre_largo = 84;        // (y)
z_motor_inf = z_eje - motor_d / 2;
lipo_z_sup  = z_motor_inf - 2;          // la LiPo va justo debajo de los motores
lipo_z_inf  = lipo_z_sup - lipo[2];

// ───────────────────────── Ball transfers 1″ ─────────────────────────
bola_d       = 25.4;
bt_y         = 110;    // a 90° de las ruedas, adelante y atrás (fuera de la charola)
bt_brida_d   = 55;     // MEDIR: brida redonda de 3 agujeros (CY-25A típico)
bt_brida_e   = 3;
bt_pcd       = 44;     // MEDIR
bt_cuerpo_d  = 36;     // MEDIR: cuerpo detrás de la brida
bt_cuerpo_L  = 16;     // MEDIR
bt_bola_fuera = 17;    // MEDIR: cuánto sale la bola de la cara de la brida
// centro de la bola: toca el casco por dentro
bt_c_r = Ri - bola_d / 2;
bt_c_z = -sqrt(bt_c_r * bt_c_r - bt_y * bt_y);
bt_inclinacion = asin(bt_y / bt_c_r);    // el ball transfer apunta al centro de la esfera

// ───────────────────────── Poste e imanes ─────────────────────────
tubo_d      = 25;       // tubo PVC conduit 25 mm (el hidráulico de 1/2″ mide 21.3: cámbialo)
iman_d      = 20;       // N42 20 × 5 avellanado M4
iman_e      = 5;
iman_r      = 27;       // radio del círculo de los 4 imanes (poste y cabeza)
hueco_iman_int = 2.5;   // imán del poste ↔ pared interior (2–3 mm)
hueco_iman_ext = 2;     // pared exterior ↔ imán de la cabeza
porta_d     = 74;       // portaimanes del poste
porta_e     = 12;       // grosor al centro (deja lugar a la tuerca M4 bajo cada imán)
// MG996R (cuerpo 40.7 × 19.7 × 42.9 confirmado; orejas y cuerno MEDIR)
servo   = [40.7, 19.7]; // largo, ancho
servo_caja_alto = 36.5; // base → tapa de la caja
servo_alto = 42.9;      // base → punta del estriado
servo_oreja_z = 26.5;   // base → cara de abajo de las orejas
servo_oreja_L = 54.5;
servo_oreja_e = 2.5;
servo_agujeros = [49.5, 10];
servo_eje_x = 40.7 / 2 - 10.2;   // el eje va a 10.2 mm de un extremo
cuerno_d = 25;          // cuerno redondo
cuerno_e = 2.5;
cuerno_sobre = 1.5;     // cuánto queda la cara del cuerno arriba del estriado

porta_z_sup = Ri - hueco_iman_int;           // cara de arriba del portaimanes, al centro
porta_z_inf = porta_z_sup - porta_e;
servo_z_inf = porta_z_inf + cuerno_e - servo_alto - cuerno_sobre;   // cuerno metido 2.5 en el portaimanes
servo_soporte_z = servo_z_inf + servo_oreja_z;   // donde apoyan las orejas
tubo_z_sup  = servo_z_inf - 6;
pie_socket  = 25;                            // cuánto entra el tubo en el pie
tubo_L      = tubo_z_sup - plat_z_sup;       // del plato al tope del tubo (ver echo)

// ───────────────────────── Cabeza ─────────────────────────
cabeza_d   = 0.6 * esfera_d;   // proporción del BB-8 de las películas
Rc         = cabeza_d / 2;
cab_pared  = 2.4;
// Perfil (marco de la cabeza: origen en el polo de la esfera, z = z_robot − Re)
cab_z_ancho = 2;              // altura del punto más ancho
cab_domo_r  = Rc;             // esfera del domo (media esfera truncada)
cab_domo_zc = cab_z_ancho - sqrt(cab_domo_r * cab_domo_r - Rc * Rc);
cab_tapa_r  = 0.45 * Rc;      // la tapa plana de arriba
cab_z_tope  = cab_domo_zc + sqrt(cab_domo_r * cab_domo_r - cab_tapa_r * cab_tapa_r);
cab_r_borde = 0.95 * Rc;      // radio del borde de abajo (se ensancha hacia arriba)
cab_hueco_borde = 4;          // el borde flota a 4 mm del casco
plato_z     = 16;             // cara de abajo del plato interior (el power bank va debajo)
plato_e     = 4;
bolita_d    = 12.7;           // 3 bolas de acero 1/2″ en copas (o mini ball transfers)
bolita_r    = 60;             // radio del círculo de las 3 copas
// Ojo principal: anillo WS2812 16 LED + cámara OV5647 detrás de su centro
ojo_elev    = 38;             // elevación del ojo vista desde el centro del domo
ojo_hueco_d = 40;
ojo_mica_d  = 50;             // mica de acrílico ahumado 3 mm
anillo      = [44.5, 31.8, 1.6];  // diámetro ext., int., grosor del PCB (MEDIR: UNIT dice 45)
camara      = [25, 24, 1];        // OV5647 tipo cámara v1; agujeros M2 21 × 12.5
camara_lente = 8.5;
// Ojo chico (holoproyector): VL53L0X
ojo2_azim   = 28;             // a la derecha del ojo principal
ojo2_z      = 4;              // altura del ojo chico (debajo del plato)
ojo2_inclin = 10;             // el ToF mira 10° hacia abajo
ojo2_d      = 16;
tof         = [25, 13, 1.6];  // MEDIR: GY-530 / CJMCU-530
// Bocina 4 Ω 5 W y power bank
bocina_d    = 40;             // MEDIR: la de UNIT puede ser de 40 o 50 mm
bocina_prof = 20;
bocina_elev = 45;             // atrás y hacia arriba
powerbank   = [26, 97];       // diámetro, largo
pizero      = [65, 30];       // agujeros M2.5 a 58 × 23

// ───────────────────────── Ayudas ─────────────────────────
function norma(v) = sqrt(v * v);
function unitario(v) = v / norma(v);

// Orienta el eje z de los hijos hacia la dirección d.
module apuntar(d) {
    u = unitario(d);
    b = acos(u[2]);
    a = atan2(u[1], u[0]);
    rotate([0, b, a]) children();
}

// Casquete esférico entre dos radios, dentro de un cono de semiángulo ang alrededor de +z.
module casquete(r1, r2, ang) {
    rotate_extrude($fn = $fn * 2)
        polygon(concat([[0, 0]],
            [for (t = [0 : ang / 24 : ang]) [r2 * sin(t), r2 * cos(t)]],
            [for (t = [ang : -ang / 24 : 0]) [r1 * sin(t), r1 * cos(t)]]));
}

// Cono macizo con vértice en el origen, alrededor de +z.
module cono(ang, h) {
    cylinder(h = h, r1 = 0, r2 = h * tan(ang));
}

module reportar() {
    echo(str("a_rueda (plano medio) = ", a_rueda, " mm"));
    echo(str("eje de ruedas z = ", z_eje, " mm; pared donde pisa la rueda a ", angulo_contacto, "°"));
    echo(str("plataforma z = ", plat_z_inf, " … ", plat_z_sup,
             "; radio libre abajo = ", sqrt(Ri * Ri - plat_z_inf * plat_z_inf)));
    echo(str("LiPo z = ", lipo_z_inf, " … ", lipo_z_sup));
    echo(str("ball transfers: centro bola (0, ±", bt_y, ", ", bt_c_z, "), inclinación ", bt_inclinacion, "°"));
    echo(str("tubo del poste: cortar a ", tubo_L - 2, " mm (del fondo del pie a la manga del servo)"));
    echo(str("cabeza: Ø", cabeza_d, ", alto sobre el polo ", cab_z_tope));
}
