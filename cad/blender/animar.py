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
CAJA_D, CAJA_L, MOTOR_D, MOTOR_L, EXC = 37.0, 22.0, 35.0, 50.0, 7.0
RUEDA_D, RUEDA_A = 65.0, 26.0
A_RUEDA = 20 + CAJA_L + MOTOR_L + 2                       # 94
X_CONTACTO = A_RUEDA + RUEDA_A / 2 - 2
Z_EJE = -(math.sqrt(RI**2 - X_CONTACTO**2) - RUEDA_D / 2)  # −89.6
X_CARA = A_RUEDA - 20
PLAT_D, PLAT_E = 210.0, 15.0
PLAT_INF = Z_EJE + CAJA_D / 2 + 3
PLAT_SUP = PLAT_INF + PLAT_E
LIPO = (47.0, 139.0, 25.0)
LIPO_INF = Z_EJE - MOTOR_D / 2 - 2 - LIPO[2]
BOLA_D, BT_Y = 25.4, 110.0
BT_R = RI - BOLA_D / 2
BT_Z = -math.sqrt(BT_R**2 - BT_Y**2)
TUBO_D = 25.0
R_PORTA = RI - 2.5
PORTA_INF = R_PORTA - 12
SERVO_INF = PORTA_INF + 2.5 - 42.9 - 1.5
TUBO_SUP = SERVO_INF - 6
IMAN_D, IMAN_E, IMAN_R = 20.0, 5.0, 27.0
SEPARADOR = 8.0
# cabeza (marco con origen en el polo de la esfera)
RC = 0.6 * 330 / 2
TAPA_R = 0.45 * RC
Z_TOPE = 2 + math.sqrt(RC**2 - TAPA_R**2)
ESPIGA_ANT = 2.4 + 3.5
PLATO_Z, PLATO_E = 16.0, 4.0


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


def materiales():
    mat("Blanco", (0.85, 0.84, 0.80), 0.35)
    mat("Naranja", (0.85, 0.32, 0.04), 0.4)
    mat("Impreso", (0.95, 0.45, 0.10), 0.55)
    mat("Melamina", (0.75, 0.62, 0.45), 0.6)
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
        pieza = "tapa_carga" if e == (0, 0, -1) else "circulo"
        stl(f"parte6-esfera/esfera_{pieza}.stl", f"Circulo {i + 1}", "Naranja", padre, R_ESFERA @ apuntar(e))


# ── Base ──
def tarjeta(nombre, material, padre, x, y, z, dx, dy, piezas=()):
    caja(nombre, material, padre, x, y, z, dx, dy, 1.6)
    for k, (px, py, sx, sy, sz, mt) in enumerate(piezas):
        caja(f"{nombre} {k + 1}", mt, padre, x + px, y + py, z + 1.6, sx, sy, sz)


def base(padre):
    p = prim("cil", "Plataforma", "Melamina", padre, (0, 0, PLAT_INF + PLAT_E / 2), dim=(PLAT_D, PLAT_D, PLAT_E), vertices=128)
    for lado in (-1, 1):      # muescas de las ruedas
        bpy.ops.mesh.primitive_cube_add()
        h = bpy.context.object
        h.dimensions = (PLAT_D * MM, 62 * MM, 40 * MM)
        h.location = (lado * (A_RUEDA - RUEDA_A / 2 - 4 + PLAT_D / 2) * MM, 0, (PLAT_INF + PLAT_E / 2) * MM)
        b = p.modifiers.new("Muesca", "BOOLEAN")
        b.object = h
        bpy.context.view_layer.objects.active = p
        bpy.ops.object.modifier_apply(modifier=b.name)
        bpy.data.objects.remove(h, do_unlink=True)

    # La base real (Parte 2): los motores cuelgan de la melamina con 2 abrazaderas sinfín
    # cada uno, la LiPo va abajo con velcro y adelante y atrás hay una rueda loca RL4 sobre
    # una cuña de madera. Nada impreso.

    for lado in (-1, 1):
        s = "der" if lado > 0 else "izq"
        x = lado * X_CARA
        cil_x(f"Caja engranes {s}", "Metal", padre, x, x - lado * CAJA_L, EXC, Z_EJE, CAJA_D)
        cil_x(f"Motor {s}", "Motor", padre, x - lado * CAJA_L, x - lado * (CAJA_L + MOTOR_L - 17), EXC, Z_EJE, MOTOR_D - 2)
        cil_x(f"Encoder {s}", "Negro", padre, x - lado * (CAJA_L + MOTOR_L - 17), x - lado * (CAJA_L + MOTOR_L), EXC, Z_EJE, MOTOR_D)
        cil_x(f"Eje {s}", "Metal", padre, x, lado * A_RUEDA, 0, Z_EJE, 6)
        cil_x(f"Llanta {s}", "Hule", padre, lado * (A_RUEDA - RUEDA_A / 2), lado * (A_RUEDA + RUEDA_A / 2), 0, Z_EJE, RUEDA_D)
        cil_x(f"Rin {s}", "Rin", padre, lado * (A_RUEDA - RUEDA_A / 2 + 3), lado * (A_RUEDA + RUEDA_A / 2 + 0.5), 0, Z_EJE, RUEDA_D - 14)
        for k, xa in enumerate((x - lado * 6, x - lado * 16)):          # abrazaderas en la caja de engranes
            cil_x(f"Abrazadera {s} {k + 1}", "Metal", padre, xa - 4, xa + 4, EXC, Z_EJE, CAJA_D + 2.5)
            caja(f"Tornillo abrazadera {s} {k + 1}", "Metal", padre, xa - 4, EXC - 5, Z_EJE + CAJA_D / 2, 8, 10, PLAT_INF - Z_EJE - CAJA_D / 2)

    caja("LiPo", "LiPo", padre, -LIPO[0] / 2, -LIPO[1] / 2, LIPO_INF, *LIPO)
    for k, yv in enumerate((-40, 40)):          # 2 tiras de velcro que la cuelgan de la melamina
        caja(f"Velcro {k + 1}", "Negro", padre, -LIPO[0] / 2 - 1.5, yv - 10, LIPO_INF - 1.5, 1.5, 20, PLAT_INF - LIPO_INF + 1.5)
        caja(f"Velcro {k + 1}b", "Negro", padre, LIPO[0] / 2, yv - 10, LIPO_INF - 1.5, 1.5, 20, PLAT_INF - LIPO_INF + 1.5)
        caja(f"Velcro {k + 1}c", "Negro", padre, -LIPO[0] / 2 - 1.5, yv - 10, LIPO_INF - 1.5, LIPO[0] + 3, 20, 1.5)
    for s in (-1, 1):
        nombre = "adelante" if s > 0 else "atras"
        c = Vector((0, s * BT_Y, BT_Z))
        prim("esf", f"Rueda loca {nombre}", "Metal", padre, c, dim=(BOLA_D,) * 3, segments=32, ring_count=16)
        u = c.normalized()
        prim("cil", f"RL4 {nombre}", "Metal", padre, c - u * (BOLA_D / 2 + 3), apuntar(u).to_euler(), dim=(36, 36, 10), vertices=32)
        # cuña de madera entre la rueda loca y la melamina
        arriba = c - u * (BOLA_D / 2 + 8)
        caja(f"Cuna {nombre}", "Melamina", padre, -20, min(arriba.y, s * 80) if s > 0 else arriba.y - 0,
             arriba.z, 40, abs(arriba.y - s * 80) + 0.01, PLAT_INF - arriba.z)

    z = PLAT_SUP + SEPARADOR
    tarjeta("Raspberry Pi 3", "PCB", padre, -42.5, 34, z, 85, 56,
            [(67, 2, 21, 16, 13.5, "Metal"), (70, 21, 17, 13, 16, "Metal"), (70, 39, 17, 13, 16, "Metal"), (7, 50, 51, 5, 8.5, "Negro")])
    tarjeta("Arduino Uno", "Uno", padre, 5 - 34.3, -88.7, z, 68.6, 53.3,
            [(-6, 38, 16, 12, 11, "Metal"), (-2, 3, 14, 9, 11, "Negro"), (27, 49, 36, 2.5, 8, "Negro"), (18, 1, 46, 2.5, 8, "Negro")])
    tarjeta("L298N", "L298N", padre, -71.5, -21.5, z, 43, 43,
            [(12, 26, 23, 16, 25, "Negro"), (0, 2, 8, 15, 10, "Buck"), (35, 2, 8, 15, 10, "Buck")])
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
    for f, nombre, m in (("ojo_mica.stl", "Mica", "Mica"), ("ojo_anillo.stl", "Anillo del ojo", "Ojo")):
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
    animar(raiz, cuerpo, base_, cabeza_)
    render()
    bpy.context.scene.frame_set(60)


main()
