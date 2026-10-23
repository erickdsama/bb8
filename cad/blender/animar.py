"""BB-8 en Blender: arma el robot completo con las piezas reales y le da vida.

Corre dentro de Blender 3.4+ (Scripting → Open → Run, o por el MCP de Blender). No
exporta nada: importa los STL de cad/stl (orientados para imprimir), los regresa a su
lugar en el robot y modela aquí lo que no se imprime (motores, ruedas, LiPo, tarjetas,
servo, imanes, power bank). Las medidas salen de cad/parametros.scad.

Marco: origen en el centro de la esfera, +y adelante, +x derecha, en mm (escala 0.001).

Animación de 12 s a 24 fps: rueda hacia adelante, frena y mira a los lados, la esfera se
vuelve transparente para ver la base por dentro, gira y regresa.
"""
import math
import os

import bpy
from mathutils import Matrix, Vector

CAD = os.environ.get("BB8_CAD") or os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STL = os.path.join(CAD, "stl")
MM = 0.001
FPS, FIN = 24, 288

# ── Medidas (cad/parametros.scad) ──
RE, PARED = 165.0, 4.0
RI = RE - PARED
CAJA_D, CAJA_L, MOTOR_D, MOTOR_L, EXC = 37.0, 22.0, 35.0, 30.0, 7.0   # caja y motor medidos
RUEDA_D, RUEDA_A = 65.0, 25.0
A_RUEDA = 20 + CAJA_L + MOTOR_L + 44 / 2                  # 94: los motores no se tocan al centro
X_CONTACTO = A_RUEDA + RUEDA_A / 2 - 2
Z_EJE = -(math.sqrt(RI**2 - X_CONTACTO**2) - RUEDA_D / 2)  # −89.6
X_CARA = A_RUEDA - 20
PLAT_D, PLAT_E = 210.0, 6.0   # MDF de 6 mm en láser
PLAT_INF = Z_EJE + CAJA_D / 2 + 3
PLAT_SUP = PLAT_INF + PLAT_E
LIPO = (47.0, 139.0, 25.0)
LIPO_INF = Z_EJE - MOTOR_D / 2 - 2 - LIPO[2]
BOLA_D, BT_Y = 25.4, 110.0
RL4_ALTO, RL4_DOMO, RL4_BRIDA = 25.4, 32.0, (50.0, 22.0)   # brida → punta de la bola
BT_R = RI - BOLA_D / 2
BT_Z = -math.sqrt(BT_R**2 - BT_Y**2)
TUBO_D = 25.0
R_PORTA = RI - 2.5
PORTA_INF = R_PORTA - 12
SERVO_INF = PORTA_INF + 2.5 - 42.9 - 1.5
TUBO_SUP = SERVO_INF - 6
IMAN_D, IMAN_E, IMAN_R = 20.0, 5.0, 27.0
SEPARADOR = 8.0
# carga inalámbrica (cad/cuna.scad): bobina receptora bajo la charola, emisora en la cuna
BOBINA_D, BOBINA_E, BOBINA_AIRE = 50.0, 4.0, 4.0
CUNA_D, CUNA_PLACA, CUNA_TAPA = 150.0, (45.0, 35.0, 12.0), 1.0
CUNA_Z_BOBINA = CUNA_PLACA[2] + 1
CUNA_Z_CENTRO = CUNA_Z_BOBINA + BOBINA_E + CUNA_TAPA + RE + 0.6   # centro de la esfera sentada
CUNA_POS = (0.0, -450.0)          # en el piso, detrás de donde arranca la animación
# cabeza (marco con origen en el polo de la esfera)
RC = 0.6 * 330 / 2
TAPA_R = 0.45 * RC
Z_TOPE = 2 + math.sqrt(RC**2 - TAPA_R**2)
ESPIGA_ANT = 2.4 + 3.5
PLATO_Z, PLATO_E = 16.0, 4.0
# tarjetas (esquina de abajo a la izquierda, en mm)
UNO = (5 - 34.3, -88.7)
L298 = (-71.5, -21.5)


def limpiar():
    for o in list(bpy.data.objects):
        bpy.data.objects.remove(o, do_unlink=True)
    for col in (bpy.data.meshes, bpy.data.materials, bpy.data.lights, bpy.data.cameras):
        for b in list(col):
            if b.users == 0:
                col.remove(b)


MATS = {}


def mat(nombre, color, rug=0.4, metal=0.0, emision=None, fuerza=0.0):
    if nombre in MATS:
        return MATS[nombre]
    m = bpy.data.materials.new(nombre)
    m.use_nodes = True
    p = m.node_tree.nodes["Principled BSDF"]
    p.inputs["Base Color"].default_value = (*color, 1)
    p.inputs["Roughness"].default_value = rug
    p.inputs["Metallic"].default_value = metal
    if emision:
        p.inputs["Emission"].default_value = (*emision, 1)
        p.inputs["Emission Strength"].default_value = fuerza
    MATS[nombre] = m
    return m


def mdf():
    """MDF: fibra café clara, mate, con grano fino de ruido."""
    m = mat("MDF", (0.62, 0.47, 0.32), 0.85)
    n = m.node_tree.nodes
    bsdf = n["Principled BSDF"]
    ruido = n.new("ShaderNodeTexNoise")
    ruido.inputs["Scale"].default_value = 900.0
    ruido.inputs["Detail"].default_value = 8.0
    rampa = n.new("ShaderNodeValToRGB")
    rampa.color_ramp.elements[0].color = (0.50, 0.36, 0.23, 1)
    rampa.color_ramp.elements[1].color = (0.70, 0.55, 0.38, 1)
    m.node_tree.links.new(ruido.outputs["Fac"], rampa.inputs["Fac"])
    m.node_tree.links.new(rampa.outputs["Color"], bsdf.inputs["Base Color"])
    return m


def plataforma_mdf(padre):
    """La plataforma tal cual se corta: contorno y agujeros de cad/plantillas/plataforma.svg."""
    import re
    import bmesh
    svg = open(os.path.join(CAD, "plantillas", "plataforma.svg")).read()
    d = re.search(r'd="([^"]+)"', svg).group(1)
    bm = bmesh.new()
    aristas = []
    for sub in re.findall(r"M([^Mz]+)z", d):
        pts = [tuple(map(float, p.split(","))) for p in re.findall(r"-?[\d.e-]+,-?[\d.e-]+", sub)]
        vs = [bm.verts.new((x * MM, -y * MM, 0)) for x, y in pts]     # el SVG trae y invertida
        aristas += [bm.edges.new((vs[i], vs[(i + 1) % len(vs)])) for i in range(len(vs))]
    bmesh.ops.triangle_fill(bm, use_beauty=True, use_dissolve=True, edges=aristas)
    caras = [f for f in bm.faces]
    ext = bmesh.ops.extrude_face_region(bm, geom=caras)
    bmesh.ops.translate(bm, vec=(0, 0, PLAT_E * MM), verts=[v for v in ext["geom"] if isinstance(v, bmesh.types.BMVert)])
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    me = bpy.data.meshes.new("Plataforma")
    bm.to_mesh(me)
    bm.free()
    o = bpy.data.objects.new("Plataforma", me)
    bpy.context.collection.objects.link(o)
    o.location = (0, 0, PLAT_INF * MM)
    me.materials.append(MATS["MDF"])
    me.materials.append(MATS["Canto MDF"])
    for f in me.polygons:
        f.material_index = 0 if abs(f.normal.z) > 0.5 else 1
    o.parent = padre
    return o


def materiales():
    mat("Blanco", (0.85, 0.84, 0.80), 0.35)
    mat("Naranja", (0.85, 0.32, 0.04), 0.4)
    mat("Impreso", (0.95, 0.45, 0.10), 0.55)
    mat("Melamina", (0.55, 0.40, 0.26), 0.7)   # madera
    mdf()
    mat("Canto MDF", (0.10, 0.055, 0.03), 0.8)   # orilla quemada del corte láser
    mat("Lastre", (0.35, 0.35, 0.37), 0.5, metal=0.7)
    mat("Metal", (0.75, 0.75, 0.78), 0.25, metal=1.0)
    mat("Motor", (0.25, 0.25, 0.27), 0.35, metal=0.8)
    mat("Hule", (0.03, 0.03, 0.03), 0.8)
    mat("Rin", (0.8, 0.65, 0.2), 0.3, metal=0.9)
    mat("LiPo", (0.15, 0.18, 0.35), 0.4)
    mat("PCB", (0.05, 0.35, 0.12), 0.5)
    mat("Uno", (0.0, 0.45, 0.5), 0.5)
    mat("L298N", (0.6, 0.05, 0.05), 0.5)
    mat("Buck", (0.05, 0.15, 0.6), 0.5)
    mat("Negro", (0.02, 0.02, 0.02), 0.5)
    mat("Tubo", (0.85, 0.85, 0.85), 0.5)
    mat("Iman", (0.55, 0.55, 0.58), 0.2, metal=1.0)
    mat("Gris", (0.45, 0.46, 0.48), 0.3, metal=0.8)
    mat("Mica", (0.01, 0.01, 0.015), 0.05)
    mat("Ojo", (0.0, 0.0, 0.0), 0.5, emision=(0.1, 0.75, 1.0), fuerza=8)
    mat("PowerBank", (0.15, 0.6, 0.35), 0.4)
    mat("Piso", (0.32, 0.30, 0.28), 0.6)
    for n in ("Blanco", "Naranja"):          # el casco se puede volver transparente
        m = MATS[n]
        m.blend_method = "BLEND"
        m.shadow_method = "HASHED"
        m.show_transparent_back = False


def vacio(nombre, padre=None, z=0.0):
    o = bpy.data.objects.new(nombre, None)
    bpy.context.collection.objects.link(o)
    o.parent = padre
    o.location.z = z
    return o


def liso(o, angulo=35):
    o.data.polygons.foreach_set("use_smooth", [True] * len(o.data.polygons))
    o.data.use_auto_smooth = True
    o.data.auto_smooth_angle = math.radians(angulo)


def stl(ruta, nombre, material, padre, m=Matrix()):
    """Importa un STL (mm) y le aplica la matriz m (en mm) para dejarlo en su lugar."""
    bpy.ops.import_mesh.stl(filepath=os.path.join(STL, ruta))
    o = bpy.context.selected_objects[0]
    o.name = nombre
    o.data.transform(m)
    o.data.transform(Matrix.Scale(MM, 4))
    o.data.materials.append(MATS[material])
    liso(o)
    o.parent = padre
    o.matrix_parent_inverse = Matrix()
    return o


def prim(tipo, nombre, material, padre, loc=(0, 0, 0), rot=(0, 0, 0), dim=None, **kw):
    """Primitiva de Blender en mm, colgada de padre."""
    f = {"cubo": bpy.ops.mesh.primitive_cube_add, "cil": bpy.ops.mesh.primitive_cylinder_add,
         "esf": bpy.ops.mesh.primitive_uv_sphere_add}[tipo]
    f(**kw)
    o = bpy.context.object
    o.name = nombre
    if dim:
        o.dimensions = dim
        bpy.ops.object.transform_apply(scale=True)
    o.data.transform(Matrix.Scale(MM, 4))
    o.location = Vector(loc) * MM
    o.rotation_euler = rot
    o.data.materials.append(MATS[material])
    if tipo != "cubo":
        liso(o, 40)
    o.parent = padre
    return o


def corte(o, punto, normal):
    """Deja solo la mitad de o que queda del lado de normal (para el domo de la RL4)."""
    bpy.ops.mesh.primitive_cube_add()
    h = bpy.context.object
    h.dimensions = (0.2, 0.2, 0.2)
    h.location = (punto - normal * 100) * MM
    h.rotation_euler = apuntar(normal).to_euler()
    b = o.modifiers.new("Corte", "BOOLEAN")
    b.object = h
    bpy.context.view_layer.objects.active = o
    bpy.ops.object.modifier_apply(modifier=b.name)
    bpy.data.objects.remove(h, do_unlink=True)


def cil_x(nombre, material, padre, x0, x1, y, z, d, **kw):
    """Cilindro a lo largo de x, de x0 a x1."""
    return prim("cil", nombre, material, padre, ((x0 + x1) / 2, y, z), (0, math.pi / 2, 0),
                (d, d, abs(x1 - x0)), vertices=48, **kw)


def apuntar(d):
    """Igual que apuntar() de parametros.scad: lleva +z a la dirección d."""
    u = Vector(d).normalized()
    b = math.acos(max(-1, min(1, u.z)))
    a = math.atan2(u.y, u.x)
    return Matrix.Rotation(a, 4, "Z") @ Matrix.Rotation(b, 4, "Y")


def caja(nombre, material, padre, x, y, z, dx, dy, dz):
    """Caja con la esquina de abajo en (x, y, z), como cube() de OpenSCAD."""
    return prim("cubo", nombre, material, padre, (x + dx / 2, y + dy / 2, z + dz / 2), dim=(dx, dy, dz))


# ── Esfera: 14 gajos + 6 círculos ──
ALFA = math.acos(1 / math.sqrt(3))
R_ESFERA = Matrix.Rotation(ALFA, 4, Vector((1, -1, 0)).normalized())
RX180 = Matrix.Rotation(math.pi, 4, "X")
GAJOS = ["A_ppp", "A_ppm", "A_pmp", "A_mpp", "A_pmm", "A_mpm", "A_mmp",
         "B_ppm", "B_pmp", "B_mpp", "B_pmm", "B_mpm", "B_mmp", "B_mmm"]


def esfera(padre):
    for g in GAJOS:
        m = R_ESFERA @ (RX180 if g[4] == "m" else Matrix())
        stl(f"parte6-esfera/esfera_gajo_{g}.stl", f"Gajo {g}", "Blanco", padre, m)
    for i, e in enumerate([(1, 0, 0), (0, 1, 0), (0, 0, 1), (-1, 0, 0), (0, -1, 0), (0, 0, -1)]):
        stl("parte6-esfera/esfera_circulo.stl", f"Circulo {i + 1}", "Naranja", padre, R_ESFERA @ apuntar(e))


# ── Cuna de carga (en el piso) ──
def cuna(padre):
    """La cuna con su bobina emisora, la placa del emisor y el cable del eliminador de 24 V."""
    c = vacio("Cuna de carga", padre)
    c.location = (CUNA_POS[0] * MM, CUNA_POS[1] * MM, 0)
    stl("parte6-esfera/esfera_cuna.stl", "Cuna", "Impreso", c)
    prim("cil", "Bobina emisora", "Negro", c, (0, 0, CUNA_Z_BOBINA + BOBINA_E / 2), dim=(BOBINA_D, BOBINA_D, BOBINA_E), vertices=64)
    caja("Emisor placa", "PCB", c, -CUNA_PLACA[0] / 2 + 1, -CUNA_PLACA[1] / 2 + 1, 1, CUNA_PLACA[0] - 2, CUNA_PLACA[1] - 2, 1.6)
    cil_x("Jack 24 V", "Negro", c, CUNA_D / 2 - 10, CUNA_D / 2 + 2, 0, 6, 8)
    cable("Cuna cable eliminador 24 V", [(CUNA_D / 2 + 2, 0, 6), (CUNA_D / 2 + 30, 0, 4), (CUNA_D / 2 + 90, 40, 2), (CUNA_D / 2 + 160, 60, 2)], "negro", 1.6, c)
    caja("Eliminador 24 V", "Negro", c, CUNA_D / 2 + 160, 45, 0, 60, 30, 25)
    for n in ("+", "-"):
        cable(f"Cuna cable emisor {n}", [(CUNA_D / 2 - 10, 0, 6), (20, (1 if n == "+" else -1) * 4, 5), (0, (1 if n == "+" else -1) * 4, CUNA_Z_BOBINA)],
              "rojo" if n == "+" else "negro", 0.5, c)
    return c


# ── Base ──
def tarjeta(nombre, material, padre, x, y, z, dx, dy, piezas=()):
    caja(nombre, material, padre, x, y, z, dx, dy, 1.6)
    for k, (px, py, sx, sy, sz, mt) in enumerate(piezas):
        caja(f"{nombre} {k + 1}", mt, padre, x + px, y + py, z + 1.6, sx, sy, sz)


def base(padre):
    plataforma_mdf(padre)

    # La base (Parte 2), todo impreso en PETG: los motores cuelgan de la plataforma de MDF en
    # sus soportes, la LiPo va en la charola con lastre a los lados y adelante y atrás hay una
    # rueda loca RL4 en su soporte inclinado.

    for lado in (-1, 1):
        s = "der" if lado > 0 else "izq"
        x = lado * X_CARA
        cil_x(f"Caja engranes {s}", "Metal", padre, x, x - lado * CAJA_L, EXC, Z_EJE, CAJA_D)
        cil_x(f"Motor {s}", "Motor", padre, x - lado * CAJA_L, x - lado * (CAJA_L + MOTOR_L - 17), EXC, Z_EJE, MOTOR_D - 2)
        cil_x(f"Encoder {s}", "Negro", padre, x - lado * (CAJA_L + MOTOR_L - 17), x - lado * (CAJA_L + MOTOR_L), EXC, Z_EJE, MOTOR_D)
        cil_x(f"Eje {s}", "Metal", padre, x, lado * A_RUEDA, 0, Z_EJE, 6)
        cil_x(f"Llanta {s}", "Hule", padre, lado * (A_RUEDA - RUEDA_A / 2), lado * (A_RUEDA + RUEDA_A / 2), 0, Z_EJE, RUEDA_D)
        cil_x(f"Rin {s}", "Rin", padre, lado * (A_RUEDA - RUEDA_A / 2 + 3), lado * (A_RUEDA + RUEDA_A / 2 + 0.5), 0, Z_EJE, RUEDA_D - 14)
        # soporte impreso en ABS (cad/stl/parte2-base), atornillado a la plataforma
        stl(f"parte2-base/base_soporte_motor{'' if lado > 0 else '_izq'}.stl", f"Soporte motor {s}", "Impreso", padre, RX180)

    stl("parte2-base/base_charola.stl", "Charola", "Impreso", padre)
    caja("LiPo", "LiPo", padre, -LIPO[0] / 2, -LIPO[1] / 2, LIPO_INF, *LIPO)
    for s in (-1, 1):       # lastre en los compartimentos a los lados de la LiPo
        x0 = LIPO[0] / 2 + 0.5 + 4 if s > 0 else -(LIPO[0] / 2 + 0.5 + 4 + 34)
        caja(f"Lastre {'der' if s > 0 else 'izq'}", "Lastre", padre, x0 + 1, -41, LIPO_INF + 1, 32, 82, 18)
    for k, yv in enumerate((-35, 35)):          # 2 correas de velcro por las ranuras de la charola
        piso = LIPO_INF - 3
        caja(f"Velcro {k + 1}", "Negro", padre, -LIPO[0] / 2 - 2.7, yv - 10, piso - 1.2, LIPO[0] + 5.4, 20, 1.2)
        caja(f"Velcro {k + 1}b", "Negro", padre, -LIPO[0] / 2 - 2.7, yv - 10, LIPO_INF + LIPO[2], LIPO[0] + 5.4, 20, 1.2)
        for x in (-LIPO[0] / 2 - 2.7, LIPO[0] / 2 + 1.5):
            caja(f"Velcro {k + 1}{'c' if x < 0 else 'd'}", "Negro", padre, x, yv - 10, piso - 1.2, 1.2, 20, LIPO[2] + 4.4)
    for s in (-1, 1):
        nombre = "adelante" if s > 0 else "atras"
        m = RX180 if s > 0 else Matrix.Scale(-1, 4, (0, 1, 0)) @ RX180
        stl("parte2-base/base_soporte_bt.stl", f"Soporte RL4 {nombre}", "Impreso", padre, m)
        c = Vector((0, s * BT_Y, BT_Z))
        u = c.normalized()
        rot = apuntar(u).to_euler()
        cara = c - u * (RL4_ALTO - BOLA_D / 2)          # cara de la brida
        prim("esf", f"RL4 {nombre} bola", "Metal", padre, c, dim=(BOLA_D,) * 3, segments=32, ring_count=16)
        # domo: falda cilíndrica + media esfera de Ø32; su punta queda 3 mm abajo de la de la bola
        falda = RL4_ALTO - 3 - RL4_DOMO / 2
        prim("cil", f"RL4 {nombre} falda", "Metal", padre, cara + u * falda / 2, rot, dim=(RL4_DOMO, RL4_DOMO, falda), vertices=48)
        d = prim("esf", f"RL4 {nombre} domo", "Metal", padre, cara + u * falda, rot, dim=(RL4_DOMO,) * 3, segments=48, ring_count=24)
        corte(d, cara + u * falda, u)
        # brida de 2 agujeros: 50 × 22 con las puntas redondas (en el marco de apuntar, el largo va en y)
        prim("cubo", f"RL4 {nombre} brida", "Metal", padre, cara - u * 0.75, rot, dim=(RL4_BRIDA[1], RL4_BRIDA[0] - RL4_BRIDA[1], 1.5))
        for k in (-1, 1):
            q = apuntar(u) @ Vector((0, k * (RL4_BRIDA[0] - RL4_BRIDA[1]) / 2, 0))
            prim("cil", f"RL4 {nombre} brida {k}", "Metal", padre, cara - u * 0.75 + q, rot, dim=(RL4_BRIDA[1], RL4_BRIDA[1], 1.5), vertices=32)
            q2 = apuntar(u) @ Vector((0, k * 17.5, 0))
            prim("cil", f"RL4 {nombre} tornillo {k}", "Negro", padre, cara + q2 + u * 1.0, rot, dim=(7, 7, 2.5), vertices=6)
    stl("parte2-base/base_panel_interruptor.stl", "Panel del interruptor", "Impreso", padre)
    # bobina receptora, pegada en el portabobina de la charola a 4 mm de la pared
    prim("cil", "Bobina receptora", "Negro", padre, (0, 0, -(RI - BOBINA_AIRE) + BOBINA_E / 2), dim=(BOBINA_D, BOBINA_D, BOBINA_E), vertices=64)

    z = PLAT_SUP + SEPARADOR
    tarjeta("Raspberry Pi 3", "PCB", padre, -42.5, 34, z, 85, 56,
            [(67, 2, 21, 16, 13.5, "Metal"), (70, 21, 17, 13, 16, "Metal"), (70, 39, 17, 13, 16, "Metal"), (7, 50, 51, 5, 8.5, "Negro"),
             (6.6, -1, 8, 5, 3, "Metal")])                       # micro-USB de alimentación, a 10.6 mm de la orilla
    # Uno R3: conectores hembra en sus posiciones reales (pines cada 2.54; D7–D8 a 4.06)
    tarjeta("Arduino Uno", "Uno", padre, *UNO, z, 68.6, 53.3,
            [(-6, 38, 16, 12, 11, "Metal"), (-2, 3, 14, 9, 11, "Negro"),
             (17.53, 49.53, 25.4, 2.54, 8.5, "Negro"), (44.45, 49.53, 20.32, 2.54, 8.5, "Negro"),
             (26.67, 1.27, 20.32, 2.54, 8.5, "Negro"), (49.53, 1.27, 15.24, 2.54, 8.5, "Negro")])
    # L298N: OUT1/OUT2 a la izquierda y OUT3/OUT4 a la derecha junto al disipador; al frente la
    # bornera de poder (+12V, GND, +5V) y los 6 pines ENA IN1 IN2 IN3 IN4 ENB (sin jumpers ENA/ENB)
    tarjeta("L298N", "L298N", padre, *L298, z, 43, 43,
            [(12, 26, 23, 16, 25, "Negro"), (0, 22, 8, 15, 10, "Buck"), (35, 22, 8, 15, 10, "Buck"),
             (6, 1, 15, 7, 10, "Buck"), (24.2, 3.3, 15.24, 2, 1, "Negro")])
    for k in range(6):
        caja(f"L298N pin {k + 1}", "Rin", padre, L298[0] + 25.27 + 2.54 * k - 0.32, L298[1] + 4.27 - 0.32, z + 1.6, 0.64, 0.64, 6)
    for k, (x, y) in enumerate([(28.5, 0.5), (28.5, -21.5)]):
        tarjeta(f"Buck {k + 1}", "Buck", padre, x, y, PLAT_SUP + 1, 43, 21, [(16, 5, 12, 12, 7, "Gris"), (30, 3, 9.5, 4.5, 10, "Rin")])
    tarjeta("MPU6050", "Buck", padre, -75, 30, PLAT_SUP + 1, 21, 16)


# ── Poste ──
def iman(nombre, padre, pos, d):
    prim("cil", nombre, "Iman", padre, pos - d.normalized() * IMAN_E / 2, apuntar(d).to_euler(), dim=(IMAN_D, IMAN_D, IMAN_E), vertices=32)


def poste(padre):
    stl("parte5-poste/poste_pie_poste.stl", "Pie del poste", "Impreso", padre)
    stl("parte5-poste/poste_soporte_servo.stl", "Soporte servo", "Impreso", padre)
    stl("parte5-poste/poste_portaimanes.stl", "Portaimanes", "Impreso", padre)
    prim("cil", "Tubo", "Tubo", padre, (0, 0, (PLAT_SUP + 2 + TUBO_SUP) / 2), dim=(TUBO_D, TUBO_D, TUBO_SUP - PLAT_SUP - 2), vertices=48)
    ex = 40.7 / 2 - 10.2
    caja("Servo", "Negro", padre, -ex - 40.7 / 2, -19.7 / 2, SERVO_INF, 40.7, 19.7, 36.5)
    caja("Servo orejas", "Negro", padre, -ex - 54.5 / 2, -19.7 / 2, SERVO_INF + 26.5, 54.5, 19.7, 2.5)
    prim("cil", "Cuerno", "Blanco", padre, (0, 0, PORTA_INF + 1.25), dim=(25, 25, 2.5), vertices=32)
    for k in range(4):
        a = math.radians(45 + 90 * k)
        d = Vector((IMAN_R * math.cos(a), IMAN_R * math.sin(a), math.sqrt(R_PORTA**2 - IMAN_R**2))).normalized()
        iman(f"Iman poste {k + 1}", padre, d * (R_PORTA - 0.5), d)


# ── Cabeza (origen en el polo de la esfera) ──
def cabeza(padre):
    stl("parte5-cabeza/cabeza_casco.stl", "Casco cabeza", "Blanco", padre)
    stl("parte5-cabeza/cabeza_plato.stl", "Plato cabeza", "Impreso", padre, RX180)
    stl("parte5-cabeza/cabeza_soporte_ojo.stl", "Soporte ojo", "Impreso", padre)
    stl("parte5-cabeza/cabeza_soporte_bocina.stl", "Soporte bocina", "Impreso", padre)
    for nombre, (x, y) in (("corta", (0, -0.45)), ("larga", (9, -0.72))):
        stl(f"parte5-cabeza/cabeza_antena_{nombre}.stl", f"Antena {nombre}", "Gris", padre,
            Matrix.Translation((x, y * TAPA_R, Z_TOPE - ESPIGA_ANT)))
    out = os.path.join(CAD, "blender", "out")
    for f, nombre, m in (("ojo_domo.stl", "Domo del ojo", "Mica"), ("ojo_anillo.stl", "Anillo del ojo", "Ojo")):
        if os.path.exists(os.path.join(out, f)):
            stl(os.path.join(out, f), nombre, m, padre)
    sup = PLATO_Z + PLATO_E
    tarjeta("Pi Zero W", "PCB", padre, -32.5, -18, sup + 5, 65, 30, [(10, -1, 8, 6, 3, "Metal"), (41, -1, 8, 6, 3, "Metal")])
    caja("ReSpeaker", "Negro", padre, -32.5, -18, sup + 18, 65, 30, 1.6)
    tarjeta("PAM8403", "L298N", padre, 28, -52, sup + 1, 30, 21)
    cil_x("Power bank", "PowerBank", padre, -48.5, 48.5, -66, PLATO_Z - 14, 26)
    for a in (90, 210, 330):
        r = math.radians(a)
        d = Vector((60 * math.cos(r), 60 * math.sin(r), math.sqrt((RE + 6.35) ** 2 - 3600))).normalized()
        prim("esf", f"Bola cabeza {a}", "Metal", padre, d * (RE + 6.35) - Vector((0, 0, RE)), dim=(12.7,) * 3, segments=24, ring_count=12)
    for k in range(4):
        a = math.radians(45 + 90 * k)
        d = Vector((IMAN_R * math.cos(a), IMAN_R * math.sin(a), math.sqrt((RE + 2) ** 2 - IMAN_R**2))).normalized()
        iman(f"Iman cabeza {k + 1}", padre, d * (RE + 2 + IMAN_E) - Vector((0, 0, RE)), d)


# ── Cableado ──
# Cada cable va de pin a pin, con los colores y calibres reales, según wiki/Parte-1-Protoboard.md
# (pines del Arduino), wiki/Parte-6-Esfera-y-baterias.md (LiPo, fusible, rocker, bucks, VIN,
# divisor en A0, MOSFET en A3), wiki/Materiales.md (pines de la Zero W con el ReSpeaker, JST del
# motor) y cad/cabeza_360.scad (AS5600, en prueba). Las etiquetas dicen el pin en cada punta.
COLOR = {
    "rojo": (0.75, 0.02, 0.02), "negro": (0.015, 0.015, 0.015), "blanco": (0.8, 0.8, 0.78),
    "morado": (0.3, 0.05, 0.5), "verde": (0.03, 0.5, 0.08), "cian": (0.0, 0.5, 0.65),
    "azul": (0.02, 0.12, 0.75), "amarillo": (0.85, 0.65, 0.0), "naranja": (0.95, 0.3, 0.0),
    "cafe": (0.25, 0.12, 0.04), "rosa": (0.9, 0.35, 0.55), "gris": (0.3, 0.3, 0.32),
    "turquesa": (0.05, 0.7, 0.5), "magenta": (0.75, 0.0, 0.55), "lima": (0.45, 0.75, 0.05),
    "beige": (0.75, 0.62, 0.42), "celeste": (0.45, 0.7, 0.95),
}
# radio del cable con forro (mm): 12 AWG de la LiPo, 18 AWG de poder, 22 AWG, Dupont, USB
R12, R18, R22, RDU, RUSB = 1.65, 1.05, 0.8, 0.65, 2.0
SUBE = (0, 0, 1)


def _mat_cable(color):
    return mat(f"Cable {color}", COLOR[color], 0.45)


def cable(nombre, pts, color, r, padre):
    """Curva bezier por los puntos (mm), con grosor r."""
    cu = bpy.data.curves.new(nombre, "CURVE")
    cu.dimensions = "3D"
    cu.bevel_depth = r * MM
    cu.bevel_resolution = 3
    cu.use_fill_caps = True
    sp = cu.splines.new("BEZIER")
    sp.bezier_points.add(len(pts) - 1)
    v = [Vector(p) * MM for p in pts]
    for i, bp in enumerate(sp.bezier_points):
        # tangente de Catmull-Rom con brazos cortos: el cable dobla sin hacer gazas
        t = (v[min(i + 1, len(v) - 1)] - v[max(i - 1, 0)]).normalized()
        bp.co = v[i]
        bp.handle_left_type = bp.handle_right_type = "FREE"
        bp.handle_left = v[i] - t * (v[i] - v[i - 1]).length * 0.28 if i else v[i]
        bp.handle_right = v[i] + t * (v[i + 1] - v[i]).length * 0.28 if i < len(v) - 1 else v[i]
    o = bpy.data.objects.new(nombre, cu)
    bpy.context.collection.objects.link(o)
    o.data.materials.append(_mat_cable(color))
    o.parent = padre
    return o


def cinta(nombre, pts, ancho, padre):
    """Cable plano (la cinta CSI de la cámara)."""
    o = cable(nombre, pts, "blanco", 0, padre)
    o.data.bevel_depth = 0
    o.data.extrude = ancho / 2 * MM
    o.data.twist_mode = "MINIMUM"
    o.data.materials[0] = mat("Cinta CSI", (0.85, 0.75, 0.5), 0.5)
    return o


FUENTE = None


def etiqueta(texto, p, padre, tam=2.2):
    """Texto que siempre mira a la cámara, con un fondo oscuro para leerlo sobre cualquier cosa."""
    cu = bpy.data.curves.new(f"Etiqueta {texto}", "FONT")
    cu.body = texto
    cu.size = tam * 1.35 * MM
    cu.align_x, cu.align_y = "CENTER", "CENTER"
    if FUENTE:
        cu.font = FUENTE
    o = bpy.data.objects.new(f"Etiqueta {texto}", cu)
    bpy.context.collection.objects.link(o)
    o.data.materials.append(mat("Etiqueta", (1, 1, 1), 0.5, emision=(1, 1, 0.92), fuerza=1.5))
    o.parent = padre
    o.location = Vector(p) * MM
    t = o.constraints.new("TRACK_TO")
    t.target = bpy.context.scene.camera
    t.track_axis, t.up_axis = "TRACK_Z", "UP_Y"
    bpy.context.view_layer.update()
    w, h = o.dimensions.x / MM, tam * 1.35 * 1.15
    bpy.ops.mesh.primitive_plane_add(size=1)
    f = bpy.context.object
    f.name = f"Etiqueta fondo {texto}"
    f.scale = ((w + 1.2) * MM, (h + 0.6) * MM, 1)
    f.location = (0, 0, -0.05 * MM)
    f.data.materials.append(mat("Fondo etiqueta", (0.01, 0.01, 0.012), 1.0))
    f.parent = o
    f.matrix_parent_inverse = Matrix()
    return o


class Red:
    """Pines con nombre: posición (mm, en el marco del padre), dirección por la que sale el cable
    y su etiqueta. Los cables se piden por nombre de pin."""

    def __init__(self, padre):
        self.padre, self.pin = padre, {}

    def p(self, nombre, pos, sale=SUBE, texto=None, off=(0, 0, 3.5)):
        self.pin[nombre] = (Vector(pos), Vector(sale).normalized())
        if texto:
            etiqueta(texto, Vector(pos) + Vector(off), self.padre)

    def _punta(self, n):
        return self.pin[n]

    def c(self, nombre, a, b, color, r=RDU, via=(), largo=4.0):
        pa, sa = self._punta(a)
        pb, sb = self._punta(b)
        pts = [pa, pa + sa * largo] + [Vector(v) for v in via] + [pb + sb * largo, pb]
        return cable(f"Cable {nombre}", pts, color, r, self.padre)


def dupont(nombre, pos, padre, alto=12.0):
    """Carcasa de un Dupont sobre un pin; regresa la punta de arriba, donde sale el cable."""
    caja(f"Conector {nombre}", "Negro", padre, pos[0] - 1.27, pos[1] - 1.27, pos[2], 2.54, 2.54, alto)
    return (pos[0], pos[1], pos[2] + alto)


# Uno R3 (USB a la izquierda): x de cada pin desde la orilla del USB; arriba en y = 50.8, abajo en 2.54
UNO_ARRIBA = {"SCL": 18.80, "SDA": 21.34, "AREF": 23.88, "GND ": 26.42, "D13": 28.96, "D12": 31.50,
              "D11": 34.04, "D10": 36.58, "D9": 39.12, "D8": 41.66,
              **{f"D{n}": 45.72 + 2.54 * (7 - n) for n in range(8)}}
UNO_ABAJO = {"IOREF": 30.48, "RESET": 33.02, "3.3V": 35.56, "5V": 38.10, "GND": 40.64, "GND2": 43.18,
             "VIN": 45.72, **{f"A{n}": 50.80 + 2.54 * n for n in range(6)}}


def cableado_base(padre):
    r = Red(padre)
    z = PLAT_SUP + SEPARADOR
    top = z + 1.6                     # cara de arriba de Pi, Uno y L298N
    piso = PLAT_SUP + 1 + 1.6         # cara de arriba de bucks, MPU y placas sobre la plataforma
    ZA = top + 28                     # nivel por encima de los Dupont para cruzar al Arduino
    # --- Arduino: Dupont macho en cada pin usado ---
    for n, px in UNO_ARRIBA.items():
        if n in ("D2", "D3", "D4", "D5", "D6", "D7", "D8", "D9", "D10", "D11", "D12", "SDA", "SCL", "GND "):
            k = list(UNO_ARRIBA).index(n)
            q = dupont(f"Uno {n.strip()}", (UNO[0] + px, UNO[1] + 50.8, top + 8.5), padre)
            r.p("uno GND arriba" if n == "GND " else f"uno {n}", q, texto=n.strip(), off=(0, 2.0 + 3 * (k % 3), 4.5 + 3 * (k % 3)))
    for n, px in UNO_ABAJO.items():
        if n in ("3.3V", "5V", "GND", "VIN", "A0", "A2", "A3", "A4", "A5"):
            k = list(UNO_ABAJO).index(n)
            q = dupont(f"Uno {n}", (UNO[0] + px, UNO[1] + 2.54, top + 8.5), padre)
            r.p(f"uno {n}", q, texto=n, off=(0, -2.0 - 3 * (k % 3), 4.5 + 3 * (k % 3)))
    # clavija USB-B en el conector del Uno (entra por −x)
    usb_b = (UNO[0] - 6 - 14, UNO[1] + 44, top + 5.5)
    caja("Conector USB-B", "Gris", padre, usb_b[0], usb_b[1] - 6, usb_b[2] - 5.5, 14, 12, 11)
    r.p("uno USB", usb_b, (-1, 0, 0), "USB: serial 115200 y 5 V", (-10, -4, 16))

    # --- L298N ---
    lx, ly = L298
    for k, n in enumerate(("ENA", "IN1", "IN2", "IN3", "IN4", "ENB")):
        q = dupont(f"L298N {n}", (lx + 25.27 + 2.54 * k, ly + 4.27, top + 6), padre, 14)
        r.p(f"l298 {n}", q, texto=n, off=(0, -2.5 - 3 * (k % 3), 4.5 + 3 * (k % 3)))
    for k, n in enumerate(("+12V", "GND", "+5V")):   # +5V sin usar: el jumper 5V-EN queda puesto
        r.p(f"l298 {n}", (lx + 8.5 + 5 * k, ly + 1, top + 5), (0, -1, 0), n if k < 2 else "+5V (sin usar)", (0, -3 - 3 * k, 7 + 3 * k))
    for n, x, s, y in (("OUT1", 0, -1, 26.5), ("OUT2", 0, -1, 32.5), ("OUT3", 39, 0, 26.5), ("OUT4", 39, 0, 32.5)):
        if s:      # izquierda: los tornillos aprietan el cable que entra por el costado
            r.p(f"l298 {n}", (lx + x, ly + y, top + 5), (-1, 0, 0), n, (-4, 0, 6))
        else:      # derecha: el costado queda pegado al pie del poste (0.5 mm), el cable baja por arriba
            r.p(f"l298 {n}", (lx + x, ly + y, top + 10), SUBE, n, (0, 0, 9 + 3 * (y > 30)))
    etiqueta("L298N", (lx + 12, ly + 30, top + 40), padre, 3.2)

    # --- Bucks: IN a la izquierda, OUT a la derecha (pads para soldar) ---
    for k, (y0, nombre) in enumerate(((0.5, "Buck 1: XL4015 → Pi 5.1 V"), (-21.5, "Buck 2: LM2596 → servo 5.1 V"))):
        b = k + 1
        for n, x, y in (("IN-", 2, 3), ("IN+", 2, 18), ("OUT-", 41, 3), ("OUT+", 41, 18)):
            r.p(f"buck{b} {n}", (28.5 + x, y0 + y, piso), SUBE, n.replace("-", "−"), (0, 0, 7 + 3 * (y > 10)))
        etiqueta(nombre, (50, y0 + 10.5, piso + 17), padre, 2.4)

    # --- Placa de distribución: un solo riel de GND (Parte 1/2), 12 V tras el rocker, 5 V del Uno ---
    rx, ry = 42.0, -76.0
    caja("Placa distribucion", "PCB", padre, rx, ry, PLAT_SUP + 1, 24, 20, 1.6)
    filas = {"12V": 17, "5V": 8, "GND": 3}
    for f, yy in filas.items():
        caja(f"Placa distribucion bus {f}", "Rin", padre, rx + 1, ry + yy - 0.6, piso, 19.5 if f == "5V" else 22, 1.2, 0.3)
        etiqueta(f, (rx - 4, ry + yy, piso + 2), padre, 2.0)
    pad = lambda f, i: (rx + 2 + 2.54 * i, ry + filas[f], piso + 0.3)
    # divisor de la batería: 10 kΩ de 12V al nodo A0, 4.7 kΩ del nodo a GND
    nodo = (rx + 22.5, ry + 12.5, piso + 0.3)
    caja("Placa distribucion nodo A0", "Rin", padre, nodo[0] - 0.8, nodo[1] - 0.8, piso, 1.6, 1.6, 0.3)
    for nm, y1, y2, col in (("10k", 17, 12.5, "Rin"), ("4k7", 12.5, 3, "Rin")):
        prim("cil", f"Resistencia {nm}", "Impreso", padre, (rx + 22.5, ry + (y1 + y2) / 2, piso + 2),
             (math.pi / 2, 0, 0), (1.8, 1.8, abs(y1 - y2) - 2), vertices=12)
    etiqueta("10k / 4.7k → A0", (rx + 23, ry + 10, piso + 9), padre, 1.8)
    r.p("riel A0", nodo, SUBE, None)
    etiqueta("Placa de distribución", (rx + 12, ry + 21, piso + 14), padre, 2.4)
    for f, i, q in (("12V", 0, "rocker"), ("12V", 1, "l298"), ("12V", 2, "b1"), ("12V", 3, "b2"), ("12V", 4, "vin"),
                    ("5V", 0, "uno"), ("5V", 1, "encI"), ("5V", 2, "encD"), ("5V", 3, "mpu"),
                    ("GND", 0, "bat"), ("GND", 1, "l298"), ("GND", 2, "b1"), ("GND", 3, "b2"), ("GND", 4, "uno"),
                    ("GND", 5, "encI"), ("GND", 6, "encD"), ("GND", 7, "mpu"), ("GND", 8, "led")):
        r.p(f"riel {f} {q}", pad(f, i))

    # --- MOSFET AO3401 (corte de la Pi por A3), en placa adaptadora con 100 kΩ de G a GND ---
    mx, my = 75.0, 22.0
    caja("MOSFET placa", "PCB", padre, mx, my, PLAT_SUP + 1, 14, 10, 1.6)
    caja("MOSFET AO3401", "Negro", padre, mx + 5, my + 3.5, piso, 3, 3, 1.1)
    prim("cil", "Resistencia 100k", "Impreso", padre, (mx + 10.5, my + 5, piso + 1), (math.pi / 2, 0, 0), (1.6, 1.6, 4), vertices=12)
    for n, x, y in (("S", 2, 5), ("D", 12.5, 5), ("G", 7, 8.6), ("GND", 7, 1.4)):
        r.p(f"mos {n}", (mx + x, my + y, piso + 0.3), SUBE, n, (0, 0, 5 + 3 * (n in ("G", "D"))))
    etiqueta("MOSFET AO3401 (A3 alto = Pi apagada)", (mx + 7, my + 6, piso + 15), padre, 2.0)

    # --- Pi 3: micro-USB de poder y USB-A (puerto de abajo) al Arduino ---
    plug = (-42.5 + 10.6, 34 - 6, top + 1.5)
    caja("Conector micro-USB Pi", "Negro", padre, plug[0] - 4, plug[1] - 1, plug[2] - 1.8, 8, 7.5, 3.6)
    r.p("pi PWR", (plug[0], plug[1] - 1, plug[2]), (0, -1, 0), "PWR micro-USB", (0, -3, 7))
    usb_a = (-42.5 + 85, 34 + 27.5, top + 4)       # puerto USB de abajo
    caja("Conector USB-A Pi", "Gris", padre, usb_a[0], usb_a[1] - 6, usb_a[2] - 2.5, 14, 12, 5)
    r.p("pi USB", (usb_a[0] + 14, usb_a[1], usb_a[2]), (1, 0, 0), "USB", (3, 0, 6))
    etiqueta("Raspberry Pi 3", (0, 70, top + 26), padre, 3.2)
    etiqueta("Arduino Uno", (UNO[0] + 34, UNO[1] + 26, top + 26), padre, 3.2)

    # --- MPU6050 (GY-521): pines en la orilla de atrás, macho hacia arriba ---
    for k, n in enumerate(("VCC", "GND", "SCL", "SDA", "XDA", "XCL", "AD0", "INT")):
        x, y = -75 + 2.6 + 2.54 * k, 30 + 1.5
        caja(f"MPU pin {n}", "Rin", padre, x - 0.32, y - 0.32, piso, 0.64, 0.64, 6)
        if n in ("VCC", "GND", "SCL", "SDA", "INT"):
            q = dupont(f"MPU {n}", (x, y, piso + 6), padre)
            r.p(f"mpu {n}", q, texto=n, off=(0, -2.5 - 3 * (k % 3), 4.5 + 3 * (k % 3)))
    etiqueta("MPU6050 (0x68)", (-64.5, 44, piso + 20), padre, 2.4)

    # --- Panel del interruptor (atrás a la izquierda, 225°): rocker y LED de carga ---
    def panel(x, y, zz):    # x: hacia afuera desde la cara interior (r = 86), y: a lo largo del panel
        c, s = math.cos(math.radians(225)), math.sin(math.radians(225))
        return (c * (86 + x) - s * y, s * (86 + x) + c * y, PLAT_SUP + zz)
    rot = (0, 0, math.radians(225))
    mat("XT60", (0.9, 0.7, 0.05), 0.4)
    mat("LED", (0.1, 0.9, 0.2), 0.3, emision=(0.1, 1, 0.2), fuerza=2)
    prim("cubo", "Rocker cuerpo", "Negro", padre, panel(-10, -11, 18), rot, (20, 19, 13))
    prim("cil", "LED carga", "LED", padre, panel(-3, 14, 18), (0, math.pi / 2, math.radians(225)), (5, 5, 8), vertices=24)
    sale = Vector(panel(-1, 0, 0)) - Vector(panel(0, 0, 0))
    sale.z = 0
    for n, y in (("rocker 1", -15.5), ("rocker 2", -6.5)):
        r.p(n, panel(-21, y, 18), sale, None)
    r.p("led +", panel(-8, 13, 18), sale, None)
    r.p("led -", panel(-8, 15, 18), sale, None)
    etiqueta("Rocker", panel(-14, -11, 29), padre, 2.2)
    etiqueta("LED carga", panel(-10, 16, 27), padre, 2.0)

    # --- Carga inalámbrica: bobina receptora → receptor → cargador CC/CV → BMS 3S → LiPo ---
    # Las tres placas van pegadas bajo la plataforma, atrás a la derecha (posición tentativa).
    zb = PLAT_INF - 1
    caja("Receptor carga", "PCB", padre, 30, -76, zb - 1.6, 38, 28, 1.6)
    caja("Receptor carga chip", "Negro", padre, 40, -66, zb - 4, 10, 8, 2.4)
    r.p("rx bobina A", (32, -70, zb - 1.6), (0, 0, -1), None)
    r.p("rx bobina B", (32, -64, zb - 1.6), (0, 0, -1), None)
    r.p("rx OUT+", (66, -52, zb - 1.6), (0, 0, -1), "OUT+", (0, 2, -4))
    r.p("rx OUT-", (66, -56, zb - 1.6), (0, 0, -1), "OUT−", (0, -2, -4))
    etiqueta("Receptor inalámbrico 24 V", (49, -62, zb - 14), padre, 2.0)
    caja("Cargador CC/CV", "Buck", padre, 30, -106, zb - 1.6, 48, 26, 1.6)
    prim("cil", "Cargador CC/CV bobina", "Negro", padre, (50, -93, zb - 6), dim=(12, 12, 8), vertices=24)
    for n, x, y in (("IN+", 32, -82), ("IN-", 32, -86), ("OUT+", 76, -82), ("OUT-", 76, -86)):
        r.p(f"cc {n}", (x, y, zb - 1.6), (0, 0, -1), n.replace("-", "−"), (0, 0, -4))
    etiqueta("Cargador CC/CV 12.6 V · 1.5 A", (54, -93, zb - 18), padre, 2.0)
    caja("BMS 3S", "PCB", padre, 72, -92, zb - 1.6, 16, 45, 1.6)
    for k, n in enumerate(("B-", "B1", "B2", "B+", "P-")):
        r.p(f"bms {n}", (80, -90 + 9.5 * k, zb - 1.6), (0, 0, -1), n.replace("-", "−"), (4, 0, -3))
    etiqueta("BMS 3S con balance", (80, -70, zb - 16), padre, 2.0)
    # la bobina: su cable sube por la ranura del portabobina al compartimiento de lastre derecho
    for k, q in enumerate(("rx bobina A", "rx bobina B")):
        cable(f"Cable bobina {k + 1}", [(BOBINA_D / 2 - 1, -1 + 2 * k, -(RI - BOBINA_AIRE) + 2), (28, -1 + 2 * k, -(RI - BOBINA_AIRE) + 6),
                                       (28, -1 + 2 * k, LIPO_INF - 1), (30, -20 + k, LIPO_INF + 10), (34, -55 + k, PLAT_INF - 14),
                                       Vector(r.pin[q][0]) + Vector((0, 0, -4)), r.pin[q][0]], "naranja", 0.45, padre)
    etiqueta("Bobina receptora (4 mm de la pared)", (0, 0, -(RI - BOBINA_AIRE) - 8), padre, 2.2)
    r.c("receptor cargador +", "rx OUT+", "cc IN+", "rojo", R22, via=[(60, -60, zb - 8), (34, -78, zb - 8)])
    r.c("receptor cargador -", "rx OUT-", "cc IN-", "negro", R22, via=[(58, -62, zb - 9), (35, -80, zb - 9)])
    r.c("cargador BMS P-", "cc OUT-", "bms P-", "negro", R22, via=[(70, -86, zb - 7), (76, -52, zb - 7)])

    # --- LiPo: su XT60 macho sale atrás de la charola; fusible 5 A en línea bajo la plataforma ---
    xt = (-10.0, -82.0, LIPO_INF + 8)
    caja("XT60 LiPo", "XT60", padre, xt[0] - 8, xt[1] - 4, xt[2] - 4, 16, 8, 8)
    for n, dx in (("lipo +", -4), ("lipo -", 4)):
        cable(f"Cable {n}", [(dx, -LIPO[1] / 2, LIPO_INF + 12), (dx, -LIPO[1] / 2 - 6, LIPO_INF + 10),
                             (xt[0] + dx * 0.9, xt[1] + 2, xt[2]), (xt[0] + dx * 0.9, xt[1] + 4, xt[2])],
              "rojo" if "+" in n else "negro", R12, padre)
    etiqueta("LiPo 3S 11.1 V (XT60)", (0, -60, LIPO_INF + LIPO[2] + 6), padre, 2.6)
    r.p("xt60 lipo +", (xt[0] - 3.6, xt[1] - 4, xt[2]), (0, -1, 0), None)
    r.p("xt60 lipo -", (xt[0] + 3.6, xt[1] - 4, xt[2]), (0, -1, 0), None)
    f0, f1 = Vector((-26, -70, -95)), Vector((-44, -56, -84))
    d = (f1 - f0).normalized()
    prim("cil", "Fusible 5 A", "Negro", padre, (f0 + f1) / 2, apuntar(d).to_euler(), (9, 9, (f1 - f0).length), vertices=24)
    r.p("fus in", f0, -d, None)
    r.p("fus out", f1, d, None)
    etiqueta("Fusible 5 A", (f0 + f1) / 2 + Vector((0, -4, -8)), padre, 2.2)
    hole_i, hole_d = Vector((-55, -40, PLAT_SUP)), Vector((55, -40, PLAT_SUP))

    # poder: LiPo + → fusible → rocker → 12V;  LiPo − → BMS B−, BMS P− → GND.
    # Carga: cargador OUT+ al + de la LiPo (empalme en su XT60), OUT− a P−.
    r.c("lipo+ fusible", "xt60 lipo +", "fus in", "rojo", R12, via=[(-14, -92, -112), (-22, -80, -100)])
    r.c("fusible rocker", "fus out", "rocker 1", "rojo", R12,
        via=[(-52, -46, -76), hole_i + Vector((1, -1, -3)), hole_i + Vector((1, -1, 9)), (-50, -48, -52)])
    r.c("lipo- BMS B-", "xt60 lipo -", "bms B-", "negro", R12, via=[(4, -96, -112), (40, -100, -96), (78, -96, zb - 8)])
    r.c("cargador OUT+ LiPo+", "cc OUT+", "xt60 lipo +", "rojo", R18, via=[(70, -80, zb - 10), (20, -98, -100), (-8, -96, -108)])
    r.c("BMS P- GND", "bms P-", "riel GND bat", "negro", R18,
        via=[(84, -50, zb - 6), (90, -50, PLAT_SUP + 4), (60, -70, PLAT_SUP + 6)])
    # 12V a la placa de distribución: por detrás del Uno, sobre la plataforma
    r.c("rocker 12V", "rocker 2", "riel 12V rocker", "rojo", R18,
        via=[(-40, -32, -48), (-26, -31, PLAT_SUP + 4), (38, -32, PLAT_SUP + 4), (44, -54, PLAT_SUP + 6)])
    # balanceo: GND, celda 1, celda 2, + de la LiPo a B−, B1, B2, B+ del BMS
    for k, (col, q) in enumerate((("negro", "bms B-"), ("rojo", "bms B1"), ("rojo", "bms B2"), ("rojo", "bms B+"))):
        cable(f"Cable balanceo {k + 1}", [Vector((-8 + 1.2 * k, -LIPO[1] / 2, LIPO_INF + 20)), Vector((-12 + 1.2 * k, -LIPO[1] / 2 - 8, LIPO_INF + 20)),
                                          Vector((20 + k, -90, -100)), Vector((60 + k, -98 + k, zb - 12)),
                                          Vector(r.pin[q][0]) + Vector((0, 0, -5)), r.pin[q][0]], col, 0.55, padre)
    etiqueta("Balanceo JST-XH → BMS", (30, -92, -104), padre, 2.0)
    # LED de carga: OUT+ del receptor → 2.2 kΩ → LED → GND
    prim("cil", "Resistencia 2k2", "Impreso", padre, (10, -60, PLAT_SUP + 4), (0, math.pi / 2, 0), (1.8, 1.8, 6), vertices=12)
    etiqueta("2.2 kΩ", (10, -60, PLAT_SUP + 9), padre, 1.8)
    r.c("LED +", "rx OUT+", "led +", "rosa", R22, via=[(62, -48, zb - 4), (40, -46, PLAT_SUP + 4), (13, -60, PLAT_SUP + 4), (7, -60, PLAT_SUP + 4), (-40, -60, PLAT_SUP + 6)])
    r.c("LED -", "led -", "riel GND led", "negro", R22, via=[(-40, -64, PLAT_SUP + 8), (30, -66, PLAT_SUP + 8), (40, -76, PLAT_SUP + 6)])

    # 12 V y GND a cada carga
    r.c("12V L298N", "riel 12V l298", "l298 +12V", "rojo", R18,
        via=[(44, -50, PLAT_SUP + 8), (36, -30, PLAT_SUP + 7), (-40, -29, PLAT_SUP + 7), (lx + 8.5, ly - 9, top + 2)])
    r.c("GND L298N", "riel GND l298", "l298 GND", "negro", R18,
        via=[(46, -52, PLAT_SUP + 6), (35, -29.5, PLAT_SUP + 5), (-40, -27.5, PLAT_SUP + 5), (lx + 13.5, ly - 8, top + 2)])
    for b, i in ((1, "b1"), (2, "b2")):
        r.c(f"12V buck{b}", f"riel 12V {i}", f"buck{b} IN+", "rojo", R18, via=[(47 + 2 * b, -50, piso + 10), (33, -25, piso + 16)])
        r.c(f"GND buck{b}", f"riel GND {i}", f"buck{b} IN-", "negro", R18, via=[(49 + 2 * b, -52, piso + 8), (35, -24, piso + 14)])
    r.c("12V VIN", "riel 12V vin", "uno VIN", "rojo", RDU, via=[(48, -66, top + 10), (UNO[0] + 45.72, -80, ZA - 4)])
    r.c("GND Uno", "riel GND uno", "uno GND", "negro", RDU, via=[(46, -78, top + 6), (UNO[0] + 40.64, -80, ZA - 6)])
    r.c("5V Uno", "uno 5V", "riel 5V uno", "rojo", RDU, via=[(UNO[0] + 38.1, -80, ZA - 8), (44, -72, top + 4)])
    r.c("A0 divisor", "uno A0", "riel A0", "lima", RDU, via=[(UNO[0] + 50.8, -80, ZA - 2), (rx + 22.5, ry + 12.5, top + 2)])

    # Pi: buck 1 → MOSFET → micro-USB; gate en A3
    r.c("buck1 OUT+ MOSFET S", "buck1 OUT+", "mos S", "rojo", R22, via=[(72, 20, piso + 8)])
    r.c("buck1 OUT- MOSFET GND", "buck1 OUT-", "mos GND", "negro", R22, via=[(73, 8, piso + 6), (80, 19, piso + 5)])
    r.c("Pi 5V", "mos D", "pi PWR", "rojo", R22, via=[(90, 30, piso + 9), (60, 32, top), (-20, 31.5, top), (plug[0] + 0.8, plug[1] - 9, plug[2])])
    r.c("Pi GND", "mos GND", "pi PWR", "negro", R22, via=[(84, 20, piso + 6), (60, 31, top - 1.5), (-20, 30.5, top - 1.5), (plug[0] - 0.8, plug[1] - 9, plug[2])])
    r.c("A3 gate", "uno A3", "mos G", "magenta", RDU,
        via=[(UNO[0] + 58.42, -80, ZA), (70, -62, ZA - 4), (90, -40, ZA - 6), (92, 10, ZA - 10), (mx + 7, my + 14, piso + 12)])
    r.c("USB Pi-Uno", "pi USB", "uno USB", "gris", RUSB,
        via=[(64, 58, top + 10), (62, 36, top + 22), (40, -26, ZA + 6), (-20, -26, ZA + 4), (-46, -40, top + 8)], largo=6)

    # Lógica del L298N (Dupont): ENA D5, IN1 D7, IN2 D8, IN3 D11, IN4 D12, ENB D6
    for k, (n, d, col) in enumerate((("ENA", "D5", "beige"), ("IN1", "D7", "gris"), ("IN2", "D8", "celeste"),
                                     ("IN3", "D11", "gris"), ("IN4", "D12", "celeste"), ("ENB", "D6", "beige"))):
        x0 = lx + 25.27 + 2.54 * k
        xd = UNO[0] + UNO_ARRIBA[d]
        r.c(f"{d} {n}", f"uno {d}", f"l298 {n}", col, RDU,
            via=[(xd, -33, ZA + 2 + k * 1.4), (x0 + 12, -28.5 - k * 0.3, ZA + 2 + k * 1.4), (x0, -24, top + 26)])

    # Motores: el JST PH de 6 pines del encoder (M1 GND C2 C1 VCC M2) en la cara de adentro
    for lado in (-1, 1):
        s = "izq" if lado < 0 else "der"
        jx = lado * (X_CARA - CAJA_L - MOTOR_L) - lado * 0.5
        caja(f"JST motor {s}", "Blanco", padre, jx - (3 if lado > 0 else 0), EXC - 6.5, Z_EJE + 6, 3, 13, 6)
        for k, n in enumerate(("M1", "GND", "C2", "C1", "VCC", "M2")):
            r.p(f"mot{s} {n}", (jx - lado * 3, EXC - 5 + 2 * k, Z_EJE + 9), (-lado, 0, 0),
                n, (-lado * 9, 0, 6 + 3.2 * (k % 3)))
        etiqueta(f"Motor {s} · encoder", (lado * 40, EXC, Z_EJE + 30), padre, 2.4)
    hole = {"izq": hole_i, "der": hole_d}
    for s, sg in (("izq", -1), ("der", 1)):
        h = hole[s]
        base_v = lambda k: [(sg * 14, -6 + k, Z_EJE + 12), (sg * 30, -33, PLAT_INF - 8), h + Vector((-sg * 4 + sg * 1.4 * k, 2, -6)),
                            h + Vector((-sg * 4 + sg * 1.4 * k, 2, 8))]
        if s == "izq":
            r.c("motor izq M1", "motizq M1", "l298 OUT1", "blanco", R22, via=base_v(0) + [(lx - 6, ly + 8, top + 5), (lx - 6, ly + 26.5, top + 5)])
            r.c("motor izq M2", "motizq M2", "l298 OUT2", "morado", R22, via=base_v(5) + [(lx - 8, ly + 8, top + 7), (lx - 8, ly + 32.5, top + 5)])
            enc = (("C1", "D2", "verde"), ("C2", "D4", "cian"))
        else:
            r.c("motor der M1", "motder M1", "l298 OUT3", "blanco", R22,
                via=base_v(0) + [(40, -26, ZA + 12), (-24, -26, ZA + 14), (lx + 39, ly + 20, top + 24)])
            r.c("motor der M2", "motder M2", "l298 OUT4", "morado", R22,
                via=base_v(5) + [(40, -24, ZA + 14), (-24, -24, ZA + 16), (lx + 39, ly + 27, top + 26)])
            enc = (("C1", "D3", "verde"), ("C2", "D10", "cian"))
        for j, (n, d, col) in enumerate(enc):
            k = {"C2": 2, "C1": 3}[n]
            xd = UNO[0] + UNO_ARRIBA[d]
            r.c(f"encoder {s} {n} {d}", f"mot{s} {n}", f"uno {d}", col, R22,
                via=base_v(k) + [(h.x - sg * 6, -30 - j, ZA + 8 + j), (xd, -30.5 - j, ZA + 8 + j), (xd, -34, ZA)])
        e = "encI" if s == "izq" else "encD"
        r.c(f"encoder {s} VCC", f"mot{s} VCC", f"riel 5V {e}", "rojo", R22,
            via=base_v(4) + [(h.x - sg * 4, -30, PLAT_SUP + 6), (40, -31, PLAT_SUP + 6), (45 + (s == "der") * 3, -60, piso + 6)])
        r.c(f"encoder {s} GND", f"mot{s} GND", f"riel GND {e}", "negro", R22,
            via=base_v(1) + [(h.x - sg * 4, -32, PLAT_SUP + 4), (40, -33, PLAT_SUP + 4), (52 + (s == "der") * 3, -62, piso + 4)])

    # MPU6050: VCC/GND a la placa, SCL A5, SDA A4, INT A2; rodea al L298N por la izquierda
    for k, (n, dest, col) in enumerate((("VCC", "riel 5V mpu", "rojo"), ("GND", "riel GND mpu", "negro"),
                                        ("SCL", "uno A5", "amarillo"), ("SDA", "uno A4", "azul"), ("INT", "uno A2", "turquesa"))):
        o = 1.4 * k
        fin = [(UNO[0] + UNO_ABAJO[dest.split()[1]], -42, ZA + 4 + o), (UNO[0] + UNO_ABAJO[dest.split()[1]], -80, ZA - 2 + o)] \
            if dest.startswith("uno") else [(44, -40, ZA + o), (50 + k, -64, piso + 8)]
        r.c(f"MPU {n}", f"mpu {n}", dest, col, RDU,
            via=[(-66 + o, 24, ZA - 2), (-82 + o, 10, ZA - 4), (-82 + o, -26, ZA - 4), (-60, -31 - o, ZA + 4 + o), (20, -31 - o, ZA + 4 + o)] + fin)

    # 1000 µF a la salida del buck 2, donde se conecta el servo (Parte 1)
    cx = (76.0, -11.0, PLAT_SUP + 5.5)
    cil_x("Condensador 1000uF", "Negro", padre, cx[0] + 4, cx[0] + 20, cx[1], cx[2], 10)
    cil_x("Condensador franja", "Gris", padre, cx[0] + 4.5, cx[0] + 19.5, cx[1] - 2.2, cx[2] + 3.2, 2.5)
    r.p("cap +", (cx[0] + 1, cx[1] + 2.5, cx[2] - 1), (-1, 0, 0), "+", (0, 2, 5))
    r.p("cap -", (cx[0] + 1, cx[1] - 2.5, cx[2] - 1), (-1, 0, 0), "−", (0, -2, 5))
    etiqueta("1000 µF 16 V", (cx[0] + 12, cx[1], cx[2] + 10), padre, 2.0)
    r.c("buck2 OUT+ cap", "buck2 OUT+", "cap +", "rojo", R22, via=[(72, -5, piso + 5)])
    r.c("buck2 OUT- cap", "buck2 OUT-", "cap -", "negro", R22, via=[(72, -17, piso + 5)])

    # Servo MG996R (D9) y AS5600 suben por el tubo y salen por el pie: van en su propio grupo
    # para esconderlos junto con el poste
    padre = r.padre = vacio("Cableado poste", padre)
    ex = 40.7 / 2 - 10.2
    sal = Vector((-ex - 40.7 / 2 - 0.5, 0, SERVO_INF + 4))
    pie = Vector((22, 22, PLAT_SUP + 8))
    for k, (n, col) in enumerate((("señal", "naranja"), ("5V", "rojo"), ("GND", "cafe"))):
        o = Vector((0, 1.3 * (k - 1), 0))
        r.p(f"servo {n}", sal + o, (-1, 0, 0))
        tubo = [sal + o + Vector((-6, 0, -6)), Vector((-6, 1.3 * (k - 1), TUBO_SUP - 4)), Vector((-6, 1.3 * (k - 1), PLAT_SUP + 20)),
                Vector((6 + k, 6 + k, PLAT_SUP + 9)), pie + Vector((k, -k, 0)), pie + Vector((8 + k, 6 - k, 2))]
        if n == "señal":
            r.c("D9 servo", "servo señal", "uno D9", col, R22,
                via=tubo + [(70, 34, ZA - 2), (92, 20, ZA - 6), (92, -26, ZA - 2), (40, -29, ZA + 10), (UNO[0] + 39.12, -31, ZA + 10)])
        else:
            r.c(f"servo {n}", f"servo {n}", "cap +" if n == "5V" else "cap -", col, R22,
                via=tubo + [(70, 34 + k, ZA - 8), (94, 18, ZA - 12), (94, -3 - 4 * k, piso + 14), (cx[0] - 4, cx[1] + (2.5 if n == "5V" else -2.5), cx[2] + 4)])
    etiqueta("Servo MG996R: café GND · rojo 5.1 V · naranja D9", sal + Vector((-8, 0, 16)), padre, 2.2)

    # AS5600 (cabeza 360°, en prueba): placa bajo la tapa del servo, imán en el eje
    az = SERVO_INF - 2.5
    caja("AS5600", "Buck", padre, -11.5, -11.5, az - 1.6, 23, 23, 1.6)
    caja("AS5600 chip", "Negro", padre, -2.5, -2, az, 5, 4, 1.4)
    for k, n in enumerate(("VCC", "OUT", "GND", "DIR", "SCL", "SDA", "GPO")):
        p = Vector((-9 + 3 * k, -10, az - 1.6))
        caja(f"AS5600 pin {n}", "Rin", padre, p.x - 0.32, p.y - 0.32, p.z - 6, 0.64, 0.64, 6)
        r.p(f"as {n}", p - Vector((0, 0, 6)), (0, 0, -1), n if n not in ("OUT", "GPO") else None, (0, -3, -2 - 2.5 * (k % 2)))
    cable("Cable AS5600 DIR-GND", [r.pin["as DIR"][0], r.pin["as DIR"][0] + Vector((0, 0, -2)),
                                   r.pin["as GND"][0] + Vector((0, 0, -2)), r.pin["as GND"][0]], "morado", 0.4, padre)
    etiqueta("AS5600 (0x36, 3.3 V) · en prueba", (0, -16, az - 14), padre, 2.2)
    for k, (n, dest, col) in enumerate((("VCC", "uno 3.3V", "rosa"), ("GND", "uno GND arriba", "negro"),
                                        ("SCL", "uno SCL", "amarillo"), ("SDA", "uno SDA", "azul"))):
        tubo = [Vector((-4 + 1.3 * k, -6, TUBO_SUP - 6)), Vector((-4 + 1.3 * k, -6, PLAT_SUP + 20)),
                Vector((4 + k, 4, PLAT_SUP + 9)), pie + Vector((k - 2, 2 - k, 1)), pie + Vector((8 + k, 4 - k, 4))]
        destino = {"3.3V": None, "GND arriba": "GND ", "SCL": "SCL", "SDA": "SDA"}[dest.split(" ", 1)[1]]
        if destino is None:
            fin = [(34, 0, ZA + 18), (34, -40, ZA + 8), (UNO[0] + UNO_ABAJO["3.3V"], -78, ZA)]
        else:
            fin = [(34, 0, ZA + 18 + k), (30, -27 - k * 0.5, ZA + 8 + k), (UNO[0] + UNO_ARRIBA[destino], -31, ZA + 6)]
        r.c(f"AS5600 {n}", f"as {n}", dest, col, R22 * 0.8, via=tubo + fin)
    return r


def cableado_cabeza(padre):
    """Cabeza: nada va al cuerpo (el enlace es por Wi-Fi). Power bank → Zero, ojo, PAM8403."""
    r = Red(padre)
    sup = PLATO_Z + PLATO_E
    zt = sup + 5 + 1.6                  # cara de arriba de la Zero
    hat = sup + 18 + 1.6                # cara de arriba del ReSpeaker
    # Zero W: PWR micro-USB a 54 mm de la orilla de la SD; CSI en la orilla derecha
    caja("Conector micro-USB Zero", "Negro", padre, -32.5 + 54 - 4, -18 - 9, zt - 0.5, 8, 9, 3.4)
    r.p("zero PWR", (-32.5 + 54, -27, zt + 1.2), (0, -1, 0), "PWR", (0, -2, 6))
    caja("Conector CSI Zero", "Negro", padre, 32.5 - 3, -9, zt, 3, 12, 1.4)
    r.p("zero CSI", (32.5 - 1.5, -3, zt + 1.4), SUBE, "CSI (cinta 22 → 15)", (8, 0, 6))
    # ReSpeaker 2-Mic v1: puertos Grove (GPIO12 y I2C) y jack de audio (posición aproximada)
    for n, x in (("GPIO12", -26.0), ("I2C", 26.0)):
        caja(f"Grove {n}", "Blanco", padre, x - 5, -12, hat, 10, 8, 5)
    for k, (n, col) in enumerate((("amarillo", "amarillo"), ("blanco", "blanco"), ("rojo", "rojo"), ("negro", "negro"))):
        r.p(f"grove12 {n}", (-26 - 3 + 2 * k, -8, hat + 5), SUBE)
        r.p(f"grovei2c {n}", (26 - 3 + 2 * k, -8, hat + 5), SUBE)
    etiqueta("Grove GPIO12 (amarillo = GPIO12)", (-26, -10, hat + 22), padre, 2.0)
    etiqueta("Grove I2C (amarillo SCL · blanco SDA · 3.3 V)", (26, -10, hat + 28), padre, 2.0)
    caja("Jack audio HAT", "Negro", padre, 2, -18 - 2, hat - 4, 6, 8, 5)
    r.p("hat jack", (5, -22, hat - 1.5), (0, -1, 0), None)
    etiqueta("ReSpeaker 2-Mic v1 sobre la Zero W", (0, 0, hat + 40), padre, 2.8)

    # Power bank (bajo el plato): dos salidas USB-A en la punta +x
    for k, zz in enumerate((PLATO_Z - 14 + 5, PLATO_Z - 14 - 5)):
        caja(f"Conector USB-A bank {k + 1}", "Gris", padre, 48.5, -66 - 6, zz - 2.5, 12, 12, 5)
    r.p("bank 1", (60.5, -66, PLATO_Z - 9), (1, 0, 0), "USB 1 → Zero", (4, 0, 7))
    r.p("bank 2 +", (60.5, -64.5, PLATO_Z - 19), (1, 0, 0), "USB 2 → 5 V", (6, 0, -5))
    r.p("bank 2 -", (60.5, -67.5, PLATO_Z - 19), (1, 0, 0), None)
    paso = Vector((0, -45, sup))            # pasacables del plato (14 × 9)
    r.c("USB bank-Zero", "bank 1", "zero PWR", "gris", 1.8,
        via=[(66, -60, PLATO_Z - 6), (30, -52, 8), paso + Vector((3, 0, -10)), paso + Vector((3, 0, 6)), (16, -38, zt + 4)], largo=5)

    # PAM8403: entrada L/⊥/R, poder +/−, salida L+/L− a la bocina de 4 Ω
    px, py = 28.0, -52.0
    for n, x, y in (("L", 2, 14), ("G", 2, 11), ("R", 2, 8), ("+", 2, 4), ("-", 2, 1.5), ("L+", 28, 16), ("L-", 28, 13)):
        r.p(f"pam {n}", (px + x, py + y, sup + 2.6), SUBE, {"G": "GND", "-": "−", "L-": "L−"}.get(n, n), (-3 if x < 10 else 3, 0, 4 + 3 * (y % 2 > 0.5)))
    etiqueta("PAM8403 (5 V)", (px + 15, py + 10, sup + 18), padre, 2.4)
    r.c("5V bank-PAM", "bank 2 +", "pam +", "rojo", R22, via=[(68, -62, PLATO_Z - 16), (20, -54, 6), paso + Vector((-3, 2, -10)), paso + Vector((-3, 2, 6)), (24, -48, sup + 8)])
    r.c("GND bank-PAM", "bank 2 -", "pam -", "negro", R22, via=[(68, -66, PLATO_Z - 18), (20, -56, 4), paso + Vector((-5, -2, -10)), paso + Vector((-5, -2, 6)), (24, -50.5, sup + 6)])
    for n, col in (("L", "blanco"), ("G", "negro"), ("R", "rojo")):
        r.c(f"audio {n}", "hat jack", f"pam {n}", col, 0.55, via=[(5, -30, hat - 6), (22, -40 + 3 * (n == "R"), sup + 10)])
    etiqueta("Audio: jack del HAT → entrada del PAM8403", (12, -32, hat + 8), padre, 2.0)

    # Bocina de 4 Ω en su soporte (atrás y arriba)
    n_boc = Vector((0, -math.cos(math.radians(45)), math.sin(math.radians(45))))
    o_boc = Vector((0, -66.8923, 68.8923))
    prim("cil", "Bocina 4 ohm", "Negro", padre, o_boc - n_boc * 12, apuntar(n_boc).to_euler(), (40, 40, 20), vertices=48)
    for n, dx in (("+", 5), ("-", -5)):
        r.p(f"boc {n}", o_boc - n_boc * 22 + Vector((dx, 0, 0)), -n_boc, {"-": "−"}.get(n, n), (dx, 2, -3))
    etiqueta("Bocina 4 Ω", o_boc - n_boc * 22 + Vector((0, 8, 6)), padre, 2.4)
    r.c("bocina +", "pam L+", "boc +", "rojo", 0.6, via=[(44, -34, sup + 12), (14, -42, 50)])
    r.c("bocina -", "pam L-", "boc -", "negro", 0.6, via=[(42, -38, sup + 12), (4, -45, 48)])

    # Ojo: anillo WS2812 de 16 LED; DIN ← 330 Ω ← GPIO12; 5 V y GND del power bank (vía el PAM8403)
    n_ojo = Vector((0, math.cos(math.radians(38)), math.sin(math.radians(38))))
    c = Vector((0, 76.1218, 61.4729)) + (-14.6) * n_ojo
    abajo = (Vector((0, 0, -1)) - Vector((0, 0, -1)).dot(n_ojo) * n_ojo).normalized()
    for n, dx in (("DIN", -3), ("5V", 0), ("GND", 3)):
        r.p(f"ojo {n}", c + abajo * 18.5 - n_ojo * 0.8 + Vector((dx, 0, 0)), -n_ojo, n, (dx * 2.5, -4, -4 - 2 * (dx == 0)))
    etiqueta("Anillo WS2812 (16 LED)", c + abajo * 4 - n_ojo * 10, padre, 2.4)
    ry = (Vector(r.pin["ojo DIN"][0]) + Vector((-6, -16, -4)))
    cil_x("Resistencia 330", "Impreso", padre, ry.x - 2.5, ry.x + 2.5, ry.y, ry.z, 1.8)
    etiqueta("330 Ω", ry + Vector((-6, 0, 3)), padre, 1.8)
    r.c("GPIO12 DIN", "grove12 amarillo", "ojo DIN", "amarillo", 0.55,
        via=[(-30, -2, hat + 14), (-36, 20, hat + 6), (ry.x - 6, ry.y - 2, ry.z), (ry.x + 6, ry.y + 2, ry.z)])
    r.c("Grove GND ojo", "grove12 negro", "ojo GND", "negro", 0.55, via=[(-24, -2, hat + 12), (-30, 22, hat + 4), (4, ry.y + 4, ry.z - 2)])
    for n in ("blanco", "rojo"):     # sin usar: se cortan y se aíslan con termorretráctil
        p = r.pin[f"grove12 {n}"][0]
        cable(f"Cable Grove {n} sin usar", [p, p + Vector((0, 0, 4)), p + Vector((-4, 3, 7))], n, 0.55, padre)
        prim("cil", f"Termo Grove {n}", "Negro", padre, p + Vector((-4.8, 3.6, 7.4)), (0, math.radians(-50), 0), (1.8, 1.8, 3), vertices=12)
    etiqueta("blanco y rojo: sin usar", r.pin["grove12 blanco"][0] + Vector((-12, 6, 10)), padre, 1.8)
    cap = Vector((12, 52, sup + 6))
    cil_x("Condensador ojo 1000uF", "Negro", padre, cap.x, cap.x + 16, cap.y, cap.z, 10)
    r.p("capo +", (cap.x - 1, cap.y + 2.5, cap.z), (-1, 0, 0), None)
    r.p("capo -", (cap.x - 1, cap.y - 2.5, cap.z), (-1, 0, 0), None)
    etiqueta("1000 µF", cap + Vector((8, 0, 8)), padre, 1.8)
    r.c("5V PAM-ojo", "pam +", "capo +", "rojo", R22 * 0.8, via=[(24, -46, sup + 10), (-40, -26, sup + 12), (-42, 30, sup + 10), (4, 52, sup + 8)])
    r.c("GND PAM-ojo", "pam -", "capo -", "negro", R22 * 0.8, via=[(24, -49, sup + 8), (-42, -28, sup + 10), (-44, 30, sup + 8), (4, 48, sup + 6)])
    r.c("5V cap-ojo", "capo +", "ojo 5V", "rojo", R22 * 0.8, via=[(2, 58, cap.z + 8)])
    r.c("GND cap-ojo", "capo -", "ojo GND", "negro", R22 * 0.8, via=[(5, 56, cap.z + 10)])

    # Cámara OV5647 (ojo chico): cinta CSI por el aligerado del plato a 30°
    q_cam = Vector((46.6525, 76.5237, 4.98135))
    a30 = Vector((62 * math.cos(math.radians(30)), 62 * math.sin(math.radians(30)), 0))
    cinta("Cable CSI", [q_cam + Vector((0, -2, -14)), q_cam + Vector((2, -12, -10)), a30 + Vector((0, 0, 4)),
                        a30 + Vector((0, 0, sup + 4)), Vector((40, 2, zt + 10)), r.pin["zero CSI"][0] + Vector((0, 0, 5)),
                        r.pin["zero CSI"][0]], 12, padre)
    etiqueta("Cámara OV5647 (CSI)", q_cam + Vector((0, -6, 12)), padre, 2.2)

    # VL53L0X bajo el ojo: al Grove I2C del HAT (yellow SCL, white SDA, red 3.3 V, black GND)
    q_tof = Vector((10.3872, 92.6935, 5.08167))
    a90 = Vector((0, 62, 0))
    for k, (n, col) in enumerate((("SCL", "amarillo"), ("SDA", "blanco"), ("VIN", "rojo"), ("GND", "negro"))):
        r.p(f"tof {n}", q_tof + Vector((-4.5 + 3 * k, -6, -8)), (0, 0, -1), n, (-4.5 + 3 * k, -4, -6 - 2.5 * (k % 2)))
        g = {"SCL": "amarillo", "SDA": "blanco", "VIN": "rojo", "GND": "negro"}[n]
        r.c(f"ToF {n}", f"tof {n}", f"grovei2c {g}", col, 0.55,
            via=[(q_tof.x - 3 + 1.2 * k, 80, -6), a90 + Vector((-2 + 1.3 * k, 0, 2)), a90 + Vector((-2 + 1.3 * k, 0, sup + 4)),
                 (26 - 3 + 2 * k, 14, hat + 14)])
    etiqueta("VL53L0X (0x29)", q_tof + Vector((0, -4, -20)), padre, 2.2)
    etiqueta("Sin cables al cuerpo: enlace Wi-Fi con TLS mutuo", (0, -10, hat + 52), padre, 2.4)
    return r


def cableado(base_, cabeza_):
    """Arma todo el cableado; se puede correr de nuevo sobre una escena ya armada."""
    global FUENTE
    ttf = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"   # trae → µ Ω −; la de Blender no
    FUENTE = bpy.data.fonts.load(ttf, check_existing=True) if os.path.exists(ttf) else None
    for m in bpy.data.materials:
        MATS.setdefault(m.name, m)
    for o in list(bpy.data.objects):
        if o.name.startswith(("Cableado", "Cable ", "Etiqueta", "Conector ", "Placa distribucion", "MOSFET", "Resistencia",
                              "Rocker cuerpo", "XT60", "JST", "Fusible", "Condensador", "AS5600", "Grove", "Jack audio",
                              "LED carga", "Receptor carga", "Cargador CC/CV", "BMS 3S",
                              "Bocina 4", "Termo Grove", "MPU pin")):
            bpy.data.objects.remove(o, do_unlink=True)
    cb = vacio("Cableado base", base_)
    cc = vacio("Cableado cabeza", cabeza_)
    cableado_base(cb)
    cableado_cabeza(cc)
    c = bpy.data.objects.get("Cuna de carga")
    if c:
        etiqueta("Cuna de carga: bobina emisora bajo 1 mm", (0, 0, CUNA_Z_CENTRO - RE + 40), c, 3.0)
        etiqueta("Eliminador 24 V 2 A", (CUNA_D / 2 + 190, 60, 40), c, 2.6)
    # si el poste o la cabeza están escondidos, sus cables también
    for grupo, ref in ((bpy.data.objects.get("Cableado poste"), "Servo"), (cc, "Casco cabeza")):
        o = bpy.data.objects.get(ref)
        if grupo and o and o.hide_viewport:
            for h in [grupo] + list(grupo.children_recursive):
                h.hide_viewport = h.hide_render = True
    return cb, cc


def escenario(raiz):
    bpy.ops.mesh.primitive_plane_add(size=14, location=(0, 1, 0))
    bpy.context.object.name = "Piso"
    bpy.context.object.data.materials.append(MATS["Piso"])
    bpy.ops.object.light_add(type="SUN", location=(2, -2, 4))
    sol = bpy.context.object
    sol.data.energy = 3
    sol.rotation_euler = (math.radians(50), 0, math.radians(30))
    bpy.ops.object.light_add(type="AREA", location=(-1.5, 1.5, 1.5))
    bpy.context.object.data.energy = 150
    bpy.context.object.data.size = 2
    bpy.ops.object.camera_add(location=(1.2, 1.9, 0.6))
    cam = bpy.context.object
    cam.data.lens = 40
    t = cam.constraints.new("TRACK_TO")
    t.target = raiz
    t.track_axis = "TRACK_NEGATIVE_Z"
    t.up_axis = "UP_Y"
    bpy.context.scene.camera = cam
    w = bpy.context.scene.world or bpy.data.worlds.new("Mundo")
    bpy.context.scene.world = w
    w.use_nodes = True
    w.node_tree.nodes["Background"].inputs["Color"].default_value = (0.55, 0.6, 0.68, 1)
    w.node_tree.nodes["Background"].inputs["Strength"].default_value = 0.6


def suave(t):
    t = min(1.0, max(0.0, t))
    return t * t * (3 - 2 * t)


def animar(raiz, cuerpo, base_, cabeza_):
    sc = bpy.context.scene
    sc.render.fps = FPS
    sc.frame_start, sc.frame_end = 1, FIN
    brillo = MATS["Ojo"].node_tree.nodes["Principled BSDF"].inputs["Emission Strength"]
    alfas = [MATS[n].node_tree.nodes["Principled BSDF"].inputs["Alpha"] for n in ("Blanco", "Naranja")]
    x = y = rumbo = giro = prev_v = 0.0
    for f in range(1, FIN + 1):
        s = f / FPS
        if s < 3:
            v = 0.45 * suave(s / 0.8) * (1 - suave((s - 2.2) / 0.8))
        elif s < 7.5:
            v = 0.0
            rumbo = math.pi * suave((s - 6.5) / 1.0)
        else:
            v = 0.45 * suave((s - 8.5) / 0.8) * (1 - suave((s - 11.2) / 0.8))
        dt = 1 / FPS
        x += -math.sin(rumbo) * v * dt
        y += math.cos(rumbo) * v * dt
        giro += v * dt / (RE * MM)
        acel = (v - prev_v) / dt
        prev_v = v
        raiz.location = (x, y, RE * MM)
        raiz.rotation_euler = (0, 0, rumbo)
        cuerpo.rotation_euler = (-giro, 0, 0)
        base_.rotation_euler = (0.08 * acel, 0, 0)           # la base cuelga y se mece al frenar
        mira = math.radians(35) * math.sin((s - 3) / 2.5 * 2 * math.pi) if 3 <= s < 5.5 else 0.0
        cabeza_.rotation_euler = (-0.12 * acel, 0, mira)
        parpadeo = 0.0 if (3.2 < s < 3.3 or 3.45 < s < 3.55) else 1.0
        brillo.default_value = (5 + 3 * math.sin(s * 2.5)) * parpadeo
        # la esfera se vuelve transparente de 4.5 a 7.5 s para ver la base
        a = 1 - 0.85 * suave((s - 4.5) / 0.7) * (1 - suave((s - 6.8) / 0.7))
        for o, props in ((raiz, ("location", "rotation_euler")), (cuerpo, ("rotation_euler",)),
                         (base_, ("rotation_euler",)), (cabeza_, ("rotation_euler",))):
            for p in props:
                o.keyframe_insert(p, frame=f)
        brillo.keyframe_insert("default_value", frame=f)
        for al in alfas:
            al.default_value = a
            al.keyframe_insert("default_value", frame=f)


def render():
    sc = bpy.context.scene
    sc.render.engine = "BLENDER_EEVEE"
    sc.eevee.use_soft_shadows = True
    sc.eevee.use_ssr = True
    sc.eevee.use_bloom = True
    sc.eevee.use_gtao = True
    sc.render.resolution_x, sc.render.resolution_y = 1280, 720


def main():
    limpiar()
    materiales()
    raiz = vacio("BB8", z=RE * MM)
    cuerpo = vacio("Esfera", raiz)
    base_ = vacio("Base", raiz)
    cabeza_ = vacio("Cabeza", raiz, z=RE * MM)
    esfera(cuerpo)
    base(base_)
    poste(base_)
    cabeza(cabeza_)
    escenario(raiz)
    cuna(None)
    cableado(base_, cabeza_)      # después del escenario: las etiquetas miran a la cámara
    animar(raiz, cuerpo, base_, cabeza_)
    render()
    bpy.context.scene.frame_set(60)


main()
