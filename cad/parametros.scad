// BB-8 · parámetros compartidos de todas las piezas.
//
// Todas las medidas en mm. Marco del robot (ensamble en reposo):
//   origen = centro de la esfera, +z arriba, +y adelante, +x derecha (eje de las ruedas).
// Cambia un número aquí y vuelve a correr cad/exportar.sh: todo lo demás se recalcula.
// Las medidas marcadas MEDIR vienen de hojas de vendedores que no coinciden entre sí;
// mide tu pieza con vernier antes de imprimir lo que depende de ellas.

$fn = 48;          // exportar.sh lo sube para los STL finales
holg = 0.3;        // holgura general de impresión (FDM 0.4 mm)
holg_pared = 5;    // lo que no toca la esfera queda al menos a 5 mm de su pared interior
                   // (la esfera gira alrededor de la base; costuras, pegamento y resina adentro)
tuerca_m3 = 6.7;   // Ø entre puntas del hueco de tuerca M3 (la tuerca mide 6.35)
tuerca_m4 = 8.4;   // Ø entre puntas del hueco de tuerca M4 (la tuerca mide 8.08)

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

// ───────────────────────── Motores JGB37-520B (encoder, 319 RPM) ─────────────────────────
caja_d     = 37;     // diámetro de la caja de engranes
caja_L     = 22;     // medido (20 oct 2026)
motor_d    = 35;     // motor 520 + disco del encoder
motor_L    = 30;     // medido (20 oct 2026): 52 mm en total sin el eje, menos la caja
excentrico = 7;      // el eje de salida va 7 mm fuera del centro de la caja
eje_d      = 6;      // eje D de 6 mm (5.5 entre planos)
eje_L      = 15.5;
buje_d     = 12;     // buje de centrado de la caja
buje_alto  = 6;
caja_pcd   = 31;     // 6 × M3 en un círculo de 31 mm (centrado en la caja)

// ───────────────────────── Ruedas ─────────────────────────
rueda_d       = 65;
rueda_ancho   = 25;   // medida (65 × 25)
rueda_offset  = 20;    // cara de la caja → plano medio de la rueda: soporte 4 mm + buje + cople hex
hueco_motores = 44;    // separación entre los dos motores, al centro. Con motores de 30 mm las
                       // ruedas se quedan a 94 mm: más adentro el eje baja y la LiPo choca con el casco
chaflan_llanta = 2;    // el borde exterior de la llanta (forrada) es redondeado

// Plano medio de cada rueda (distancia al centro en x): los dos motores coaxiales
// llenan el ancho, así que no se elige, sale de las piezas.
a_rueda = rueda_offset + caja_L + motor_L + hueco_motores / 2;
// La rueda es vertical; toca la esfera con su borde exterior.
x_contacto = a_rueda + rueda_ancho / 2 - chaflan_llanta;
z_eje = -(sqrt(Ri * Ri - x_contacto * x_contacto) - rueda_d / 2);
angulo_contacto = asin(x_contacto / Ri);   // inclinación de la pared donde pisa la rueda

// ───────────────────────── Plataforma (MDF 5.5 mm) ─────────────────────────
plat_d      = 210;    // 20–22 cm
plat_e      = 5.5;    // MDF "de 6" (5.5 real) cortado en láser con plantillas/plataforma.dxf
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

// ───────────────────────── Ruedas locas RL4 (Electrozone) ─────────────────────────
// Domo de acero sobre una brida de 2 agujeros (medidas de Erick con la pieza en mano, 30 oct).
bola_d       = 19.05;  // 3/4″: se ven 18 mm por la boca; con 20 mm de salida y la espalda plana no cabe una de 1″
bt_y         = 110;    // a 90° de las ruedas, adelante y atrás
bt_brida     = [50, 32];   // punta a punta × ancho al centro (rombo: el domo de 30 + orilla)
bt_brida_e   = 1.5;
bt_agujero_d = 5;
bt_agujero_sep = 38;   // entre centros: 43 de orilla a orilla de los agujeros − 5
bt_ranura    = [35, 41];   // ranuras de ±3 mm alrededor de bt_agujero_sep
bt_domo_d    = 30;     // base del domo sobre la brida
bt_bola_fuera = 20 - bt_brida_e;  // cara de la brida → punta de la bola (20 desde la espalda)
// centro de la bola: toca el casco por dentro
bt_c_r = Ri - bola_d / 2;
bt_c_z = -sqrt(bt_c_r * bt_c_r - bt_y * bt_y);
bt_inclinacion = asin(bt_y / bt_c_r);    // el ball transfer apunta al centro de la esfera

// ───────────────────────── Poste e imanes ─────────────────────────
tubo_d      = 29.6;     // tubo PVC conduit 1″ (nominal 25 mm): exterior 29.4–29.6 según la ficha
iman_d      = 20;       // N42 20 × 5 avellanado M4
iman_e      = 5;
iman_r      = 27;       // radio del círculo de los 4 imanes (poste y cabeza)
hueco_iman_int = 2.5;   // imán del poste ↔ pared interior (2–3 mm)
hueco_iman_ext = 2;     // pared exterior ↔ imán de la cabeza
porta_d     = 74;       // portaimanes del poste
porta_e     = 12;       // grosor al centro (deja lugar a la tuerca M4 bajo cada imán)
// MG995/MG996R (same body; cuerpo 40.7 × 19.7 × 42.9 confirmado; orejas del plano MG995 y medidas)
servo   = [40.7, 19.7]; // largo, ancho
servo_caja_alto = 36.5; // base → tapa de la caja
servo_alto = 42.9;      // base → punta del estriado
servo_oreja_z = 26.5;   // base → cara de abajo de las orejas
servo_oreja_L = 52.8;
servo_oreja_e = 2.5;
servo_agujeros = [47.8, 10.2];
servo_eje_x = 40.7 / 2 - 10.2;   // el eje va a 10.2 mm de un extremo
cuerno_d = 20;          // cuerno redondo (medido)
cuerno_r_torn = 7;      // centro → agujeros de afuera del cuerno (MEDIR)
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
// Ojo principal: media esfera de acrílico transparente pintada por dentro con Tamiya TS-71
// (humo); apagado se ve negro, el anillo WS2812 de 16 LED la ilumina desde su base.
ojo_elev    = 38;             // elevación del ojo vista desde el centro del domo
ojo_domo_d  = 50;             // MEDIR: esfera de acrílico de 5 cm, una mitad
ojo_sobresale = 8;            // cuánto sale el domo de la cara del casco
ojo_hueco_d = ojo_domo_d + 0.6;
anillo      = [44.5, 31.8, 1.6];  // diámetro ext., int., grosor del PCB (MEDIR: UNIT dice 45)
// Ojo chico: la cámara OV5647 (módulo para Pi Zero, 72.4°) ve por un agujero en cono;
// desde fuera solo se ve el lente dentro de un aro de 16 mm.
ojo2_azim   = 28;             // a la derecha del ojo principal
ojo2_z      = 4;              // altura del lente (debajo del plato)
ojo2_inclin = 5;              // la cámara mira 5° hacia abajo
ojo2_d      = 16;             // aro decorativo, 0.8 mm de hondo (se pinta negro)
camara      = [25, 24, 1];    // tarjeta; el lente queda 2.5 mm arriba de su centro
camara_lente = 8.5;           // bloque cuadrado del lente
camara_lente_alto = 6;        // MEDIR: de la cara de la tarjeta a la punta del lente
camara_fov  = 72.4;
// Sensor de distancia VL53L0X detrás del círculo chico bajo el ojo principal
tof_azim    = 6;
tof_z       = 4;
tof_inclin  = 10;             // mira 10° hacia abajo
tof_hueco   = 8;
tof         = [25, 13, 1.6];  // MEDIR: GY-530 / CJMCU-530
// Antenas impresas en la tapa, hacia atrás: [x, y, Ø del agujero]
antena_corta_pos = [0, -0.45, 6];   // y en fracciones de cab_tapa_r
antena_larga_pos = [9, -0.72, 4];
antena_corta = [9, 28];       // Ø y alto sobre la tapa
antena_larga = [5, 90];       // Ø de la varilla y alto total; base Ø10 × 14
alambre_d   = 1.8;            // canal para un alambre de acero (clip o cuerda de piano de 1.5 mm)
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

// ───────────── Carga inalámbrica ─────────────
// La base siempre cuelga abajo y gira alrededor del centro de la esfera (ruedas y ball
// transfers tocan la pared), así que un punto de la base queda siempre a la misma
// distancia de la pared: la bobina receptora va ahí, bajo la charola, y la emisora en el
// plato de carga (plato.scad). Bobinas de ~50 mm: MEDIR las que lleguen y ajustar.
bobina_d     = 50;     // Ø de la bobina (receptora y emisora)
bobina_e     = 4;      // grosor con su ferrita
bobina_aire  = 7;      // aire entre la bobina y la pared en el centro (en la orilla ~5 mm)
piel_bobina  = 1;      // plástico del plato entre la bobina emisora y la esfera
// distancia bobina-bobina: aire + pared + piel del plato (el módulo debe dar ≥ 10 mm)
bobina_dist  = bobina_aire + pared + piel_bobina;

module reportar() {
    echo(str("a_rueda (plano medio) = ", a_rueda, " mm"));
    echo(str("eje de ruedas z = ", z_eje, " mm; pared donde pisa la rueda a ", angulo_contacto, "°"));
    echo(str("plataforma z = ", plat_z_inf, " … ", plat_z_sup,
             "; radio libre abajo = ", sqrt(Ri * Ri - plat_z_inf * plat_z_inf)));
    echo(str("LiPo z = ", lipo_z_inf, " … ", lipo_z_sup));
    echo(str("ball transfers: centro bola (0, ±", bt_y, ", ", bt_c_z, "), inclinación ", bt_inclinacion, "°"));
    echo(str("tubo del poste: cortar a ", tubo_L - 2, " mm (del fondo del pie a la manga del servo)"));
    echo(str("cabeza: Ø", cabeza_d, ", alto sobre el polo ", cab_z_tope));
    echo(str("carga: bobina a ", bobina_aire, " mm de la pared; bobina a bobina ", bobina_dist, " mm"));
}
