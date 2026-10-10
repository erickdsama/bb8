# Diseño 3D

Todo lo que se imprime del BB-8, en OpenSCAD paramétrico: la esfera, la base motriz, el
poste con los imanes y la cabeza. Está en
[`cad/`](https://github.com/erickdsama/bb8/tree/main/cad); las medidas viven en
[`cad/parametros.scad`](https://github.com/erickdsama/bb8/blob/main/cad/parametros.scad)
y todo lo demás se calcula a partir de ellas.

![BB-8 completo](https://github.com/erickdsama/bb8/blob/main/cad/png/bb8.png?raw=true)
![Corte](https://github.com/erickdsama/bb8/blob/main/cad/png/bb8_corte.png?raw=true)

```bash
sudo apt install openscad xvfb
bash cad/exportar.sh          # STL en cad/stl, plantilla de la plataforma en cad/plantillas, renders en cad/png
bash cad/exportar.sh png      # solo renders
openscad cad/ensamble.scad    # el ensamble, con corte (corte=true) o explotado (explotar=1)
```

**Nada de esto se ha impreso todavía.** La geometría se revisó en el ensamble: nada de
la base, el poste ni la cabeza toca el casco salvo las ruedas y las bolas (intersección
CGAL vacía con 1 mm de margen). Antes de imprimir, mide lo marcado **MEDIR** en
`parametros.scad` (ver [Antes de imprimir](#antes-de-imprimir)).

## Cómo está pensado

Marco del robot: origen en el centro de la esfera, +z arriba, +y adelante, +x hacia la
rueda derecha. Esfera de 330 mm con pared de 4 mm (radio interior 161).

### Base

![Base](https://github.com/erickdsama/bb8/blob/main/cad/png/base.png?raw=true)
![Base por abajo](https://github.com/erickdsama/bb8/blob/main/cad/png/base_abajo.png?raw=true)

- **Las ruedas salen de las piezas, no se eligen.** Los dos JGB37 van coaxiales y llenan el ancho: cople + caja de 22 mm + motor con encoder de 50 mm + 2 mm al centro ponen cada rueda a 94 mm del centro. Ahí la pared interior está inclinada 41°, así que la rueda (vertical) toca con el **borde exterior de la llanta**: fórrala con cinta de silicona y redondéale el canto. Inclinar los motores para que la llanta pise plana no cabe: el extremo del motor chocaría con el casco.
- **La plataforma va arriba y lo pesado cuelga.** El eje de las ruedas queda a z = −89.5 y la plataforma de melamina de 15 mm entre −68 y −53; a esa altura el casco deja 146 mm de radio libre, así que el disco de 210 mm entra holgado. Los motores cuelgan en soportes impresos con la placa frontal atornillada a la cara de la caja (M3 avellanados: la rueda pasa a 3 mm).
- **La LiPo va justo debajo de los motores** (z −134 a −109), en una charola colgada de la plataforma con dos paredes en los extremos. A cada lado de la LiPo, bajo los motores, hay un compartimento de lastre de 34 × 84 mm: caben ~0.5 kg de acero o ~0.75 kg de plomo por lado (la esfera recorta las esquinas), que es el lastre de 1–1.5 kg que pide la Parte 6, en el punto más bajo posible.
- **Ball transfers a ±110 mm en y**, fuera de la charola, inclinados 48° para que apunten al centro de la esfera. Sus soportes se nivelan con arandelas entre la brida y el soporte: las 4 patas (2 ruedas y 2 bolas) deben tocar el casco a la vez; si las bolas tocan primero, la esfera no rueda.
- **Plataforma:** `cad/plantillas/plataforma.svg` y `.dxf` a escala 1:1 con las muescas de las ruedas, todos los agujeros (soportes, charola, ball transfers, pie del poste, Pi 4, Uno, L298N, panel de carga) y dos pasacables de 22 mm. Imprímela en papel a 100 %, pégala en la melamina y taladra.
- Acomodo sobre la plataforma: Pi 4 adelante, Uno atrás, L298N a la izquierda, los dos LM2596 a la derecha, MPU6050 adelante a la izquierda con cinta doble cara (chip arriba) y el panel de carga atrás a la izquierda.

### Poste e imanes

![Poste](https://github.com/erickdsama/bb8/blob/main/cad/png/poste.png?raw=true)

Tubo PVC de 25 mm fijo en un pie impreso; arriba, el MG996R en un soporte que deja su eje
justo en el centro del tubo, y sobre el cuerno el portaimanes con 4 imanes N42 de 20 × 5 a
27 mm del eje. La cara de arriba del portaimanes es un casquete de la esfera interior
menos 2.5 mm: los imanes quedan a 2.5 mm del casco en toda su cara. `reportar()` en
`parametros.scad` da el largo exacto del tubo (con estas medidas, 150 mm: del fondo del pie
al tope dentro de la manga del soporte del servo). Recórtalo después de probar la base dentro de la
esfera, como pide la Parte 6.

### Cabeza

![Cabeza](https://github.com/erickdsama/bb8/blob/main/cad/png/cabeza.png?raw=true)

- Ø 198 mm (0.6 de la esfera, la proporción del BB-8 de las películas), media esfera truncada arriba y una banda que baja siguiendo la curva del cuerpo a 4 mm.
- Se apoya en **3 bolas de acero de 1/2″** en copas a 60 mm del eje. Las ruedecitas del documento técnico solo ruedan en una dirección, y la esfera pasa debajo de la cabeza en todas. Las bolas entran por arriba y las retiene un tapón a presión (con grasa de litio o PTFE).
- 4 imanes en postes del plato, a 2 mm de la pintura: entre las caras de los imanes del poste y de la cabeza hay 8.5 mm (2.5 + 4 de pared + 2).
- Todo cuelga de un plato interior: el **power bank va debajo del plato**, atrás, para bajar el centro de gravedad; arriba van la Pi Zero 2 W (separadores M2.5), el PAM8403, el soporte del ojo y el de la bocina.
- **Ojo principal** a 38° de elevación: mica ahumada de 50 × 3 mm, el anillo WS2812 de 16 LED detrás y la OV5647 en un bolsillo del mismo soporte, con la lente en el centro del anillo.
- **Ojo chico** a 28° a la derecha: el VL53L0X, mirando 10° hacia abajo. Con su campo de 25°, el piso aparece hasta ~0.8 m, lejos de los 25 cm del freno.
- Bocina atrás y hacia arriba (45°) con rejilla hexagonal; dos agujeros de antena en la tapa; ranura del USB del power bank atrás, abajo.
- El casco baja sobre el plato y se fija con 3 tornillos M3 avellanados radiales a tuercas atrapadas en el plato.

### Esfera

![Esfera](https://github.com/erickdsama/bb8/blob/main/cad/png/esfera.png?raw=true)

- El patrón es el del BB-8: **6 círculos naranjas en las caras de un cubo** (30° de radio angular) y **8 gajos blancos**, uno por octante, como dice el documento técnico.
- **La junta de las dos mitades** es el gran círculo perpendicular a la diagonal (1,1,1) del cubo. Pasa entre los círculos a 5° de cada uno, así que ningún círculo queda partido; corta 6 gajos en dos, por eso son 14 piezas blancas.
- **Por dentro no sobresale nada**, porque ruedas y ball transfers pasan por todo el casco:
  - los círculos son tapas de 2 mm pegadas en un asiento de 2 mm (debajo queda un piso de 2 mm);
  - la junta es un traslape a media pared (10 mm) con **bayoneta de 6 lengüetas** de 0.6 mm que viven dentro de la pared: se encajan las mitades y se giran 7° para cerrar;
  - los gajos de una misma mitad se pegan a tope con espigas de filamento de 1.75 mm (18 agujeros), y luego se refuerzan por dentro con fibra de vidrio fina, lijada a ras.
- **Tapa de carga:** el círculo del eje −z del patrón tiene el piso abierto (queda una ceja de 8 mm) y se atornilla con 4 M2 × 4 avellanados. Para cargar, se gira la esfera con la mano (la base siempre cuelga) hasta ver el panel de carga por la tapa.

## Cambios respecto al documento técnico

| Documento técnico | Diseño 3D | Por qué |
| --- | --- | --- |
| XT60, JST-XH y rocker en un soporte detrás de la tapa (en el casco), con extensiones a la LiPo | Panel de carga en la **base** | La esfera gira alrededor de la base: un cable de la base al casco se enreda en la primera vuelta |
| Cabeza con 3 ruedecitas | 3 bolas de acero de 1/2″ en copas | Una rueda solo rueda en un sentido; la esfera pasa en todos |
| Cúpula de 12–14 cm | Cabeza de 198 mm (paramétrica) | El power bank mide 97 mm de largo y la proporción del BB-8 es ~0.6 del cuerpo. Cambia `cabeza_d` si prefieres otra |
| VL53L0X por I2C del Arduino | VL53L0X en la cabeza (ojo chico) | Dentro de la esfera solo vería el casco; en la cabeza lo lee la Zero (`/tof`) y el firmware va con `USAR_TOF_LOCAL 0` |
| Ball transfers "a la misma altura que las ruedas" | A ±110 mm, inclinados 48°, nivelados con arandelas | Es lo que hace que las 4 patas toquen la esfera a la vez |
| Base con LiPo "centrada abajo" | LiPo bajo los motores y lastre a sus lados en la charola | Es el punto más bajo que deja la geometría |

## Piezas a imprimir

PLA, boquilla 0.4. Los STL ya salen orientados. Cama mínima 220 × 220 × 170 mm (cada
gajo mide hasta 165 mm por lado).

| STL | Copias | Cómo |
| --- | --- | --- |
| `esfera_gajo_A_ppp` … `esfera_gajo_B_mmm` (14) | 1 de cada | 0.2 mm, 4 perímetros (pared maciza), soportes de árbol solo por dentro. El nombre dice la mitad (A/B) y el octante (p = +, m = −) |
| `esfera_circulo` | 5 | 0.2 mm, en naranja, cara convexa arriba, sin soportes |
| `esfera_tapa_carga` | 1 | Igual que los círculos |
| `base_soporte_motor`, `base_soporte_motor_izq` | 1 y 1 | 30 % de relleno, 4 perímetros |
| `base_charola` | 1 | 30 %; carga 450 g de LiPo + el lastre |
| `base_soporte_bt` | 2 | 40 %, soportes en el hueco del cuerpo |
| `base_panel_carga` | 1 | |
| `poste_pie_poste`, `poste_soporte_servo` | 1 y 1 | 30 % |
| `poste_portaimanes` | 1 | 50 %; los imanes N42 rompen piezas flojas |
| `cabeza_casco` | 1 | 0.2 mm, de pie, soportes de árbol por dentro (la tapa plana es un puente) |
| `cabeza_plato` | 1 | De cabeza (ya viene así), 30 % |
| `cabeza_soporte_ojo`, `cabeza_soporte_bocina` | 1 y 1 | Con soportes |
| `cabeza_tapon` | 3 | |

Tamaño y peso (sólido, PLA 1.24 g/cm³; es un tope, con relleno las piezas gruesas pesan menos):

| STL | Caja (mm) | cm³ | g |
| --- | --- | --- | --- |
| `base_charola.stl` | 132 × 148 × 69 | 208 | 258 |
| `base_panel_carga.stl` | 53 × 53 × 36 | 8 | 10 |
| `base_soporte_bt.stl` | 57 × 44 × 48 | 35 | 43 |
| `base_soporte_motor.stl` | 70 × 52 × 45 | 44 | 54 |
| `base_soporte_motor_izq.stl` | 70 × 52 × 45 | 44 | 54 |
| `cabeza_casco.stl` | 198 × 198 × 115 | 175 | 218 |
| `cabeza_plato.stl` | 189 × 189 × 31 | 153 | 189 |
| `cabeza_soporte_bocina.stl` | 48 × 58 × 65 | 6 | 8 |
| `cabeza_soporte_ojo.stl` | 52 × 61 × 56 | 28 | 35 |
| `cabeza_tapon.stl` | 13 × 13 × 8 | 1 | 1 |
| `esfera_circulo.stl` | 164 × 164 × 24 | 42 | 52 |
| `esfera_gajo_A_mmp.stl` | 125 × 125 × 57 | 28 | 35 |
| `esfera_gajo_A_mpm.stl` | 125 × 57 × 125 | 28 | 35 |
| `esfera_gajo_A_mpp.stl` | 140 × 163 × 163 | 112 | 138 |
| `esfera_gajo_A_pmm.stl` | 57 × 125 × 125 | 28 | 35 |
| `esfera_gajo_A_pmp.stl` | 163 × 140 × 163 | 112 | 138 |
| `esfera_gajo_A_ppm.stl` | 163 × 163 × 140 | 112 | 139 |
| `esfera_gajo_A_ppp.stl` | 163 × 163 × 163 | 133 | 165 |
| `esfera_gajo_B_mmm.stl` | 163 × 163 × 145 | 124 | 154 |
| `esfera_gajo_B_mmp.stl` | 163 × 163 × 133 | 104 | 129 |
| `esfera_gajo_B_mpm.stl` | 163 × 133 × 145 | 95 | 118 |
| `esfera_gajo_B_mpp.stl` | 49 × 115 × 115 | 21 | 26 |
| `esfera_gajo_B_pmm.stl` | 133 × 163 × 145 | 95 | 118 |
| `esfera_gajo_B_pmp.stl` | 115 × 49 × 115 | 21 | 26 |
| `esfera_gajo_B_ppm.stl` | 115 × 115 × 31 | 12 | 15 |
| `esfera_tapa_carga.stl` | 164 × 164 × 24 | 41 | 51 |
| `poste_pie_poste.stl` | 56 × 56 × 25 | 17 | 21 |
| `poste_portaimanes.stl` | 74 × 74 × 12 | 33 | 41 |
| `poste_soporte_servo.stl` | 61 × 33 × 48 | 30 | 37 |

Una copia de cada STL suma ~2.3 kg; con los 5 círculos y los 3 tapones, la esfera sola
ronda 1.6 kg de PLA, dentro de los 2–3 kg de filamento que pide la Parte 6.

## Tornillería y extras

Además del [pedido UNIT](Materiales.md):

| Pieza | Cant. | Para |
| --- | --- | --- |
| Tornillo M3 × 30 + tuerca | 18 | Soportes de motor (8), charola (4), ball transfers (6), a través de la melamina |
| Tornillo M3 × 6–8 avellanado | 6–12 | Cara de la caja de cada motor (los que coincidan de los 6) |
| Tornillo M3 × 25 + tuerca | 6 | Pie del poste (4), panel de carga (2) |
| Tornillo M4 × 16 + tuerca | 6 | Ball transfers |
| Tornillo M4 × 10 avellanado + tuerca de nylon | 8 | Imanes (ya en la lista de materiales) |
| Autorroscante M3 × 12 | 4 | Orejas del MG996R |
| Autorroscante M2 × 6 | 4 | Cuerno del servo al portaimanes |
| Autorroscante M2 × 4 avellanado | 4 | Tapa de carga |
| Tornillo M3 × 12 avellanado + tuerca | 3 | Casco de la cabeza al plato |
| Tornillo M3 × 10 + tuerca | 4 | Soportes del ojo y de la bocina |
| Separadores M2.5 de latón | 8 | Pi 4 (8 mm) y Pi Zero (5 mm) |
| Bola de acero 1/2″ (12.7 mm) | 3 | Cabeza |
| Mica acrílica ahumada Ø 50 × 3 mm | 1 | Ojo |
| Correa de velcro 20 mm | 2 | LiPo |
| Lastre (acero, plomo, perdigones con resina) | 1–1.5 kg | Charola |
| Tubo PVC conduit 25 mm | 20 cm | Poste |

**Imanes:** el poste los quiere con la **cara N avellanada** (avellanado arriba, N hacia el
casco) y la cabeza con la **cara S avellanada** (avellanado abajo). Pídelos así, o compra
8 iguales y comprueba con una brújula antes de atornillar: si la polaridad sale al revés,
la cabeza se repele.

## Antes de imprimir

Mide y corrige en `parametros.scad`:

- `caja_L` y `motor_L` del JGB37: la caja mide 19–29 mm según la reducción; el motor con encoder, ~50. Cambian la posición de las ruedas y todo lo de abajo.
- `rueda_offset`: distancia real de la cara de la caja al centro de la rueda con tu cople.
- Ball transfer: `bt_brida_d`, `bt_pcd`, `bt_cuerpo_d`, `bt_cuerpo_L`, `bt_bola_fuera`.
- MG996R: orejas (`servo_oreja_z`, `servo_agujeros`) y el cuerno (`cuerno_d`, y el radio de sus agujeros en `portaimanes`).
- `tubo_d`: el conduit de 25 mm mide 25; el hidráulico de 1/2″, 21.3.
- Cabeza: `anillo`, `tof`, `bocina_d` (la de UNIT puede ser de 40 o 50 mm), `powerbank`.
- Agujeros del L298N (`L298N_AG` en `base.scad`).

Luego imprime primero **dos gajos vecinos de mitades distintas** (por ejemplo `A_ppm` y
`B_pmm`) y prueba la bayoneta antes de gastar los 2 kg de filamento.

## Armado

1. Plataforma: corta y taladra con la plantilla. Monta motores, ruedas, charola y soportes de ball transfer; nivela con arandelas sobre una mesa (Parte 2).
2. Electrónica sobre la plataforma y prueba la base andando sin esfera (Partes 3 y 4).
3. Pega los 7 gajos de cada mitad con las espigas, refuerza por dentro y lija a ras.
4. Mete la base en la mitad B, cierra con la A y gira 7°. Prueba que rueda **antes** de poner lastre e imanes.
5. Corta el tubo, monta servo y portaimanes, pon el lastre y prueba que no vuelca al frenar.
6. Cabeza: plato con imanes, bolas y power bank; electrónica; casco con 3 tornillos.
7. Pinta: pega los círculos naranjas al final (la tapa de carga se atornilla).
