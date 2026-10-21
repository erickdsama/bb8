"""BB-8 en Blender: arma la escena con las piezas del CAD y le da vida.

Corre dentro de Blender (3.4+): Scripting → Open → Run, o por el MCP de Blender.
Usa el casco de la cabeza de cad/stl y las piezas armadas de cad/blender/out
(esfera_naranja, antenas, ojo_mica, ojo_anillo; ver piezas.scad). La esfera blanca se
hace aquí con una esfera de Blender, porque exportarla de OpenSCAD pide mucha memoria.

Animación de 10 s a 24 fps: BB-8 rueda hacia adelante, frena, mira a los lados con el
ojo parpadeando, gira y regresa.
"""
import math
import os

import bpy

CAD = os.path.dirname(os.path.dirname(os.path.abspath(bpy.path.abspath("//") or __file__)))
if not os.path.isdir(os.path.join(CAD, "stl")):
    CAD = os.environ.get("BB8_CAD", os.path.expanduser("~/projects/novaminds/bb8/cad"))
MM = 0.001
RE = 165 * MM          # radio exterior de la esfera
FPS, FIN = 24, 240


def limpiar():
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete()
    for col in (bpy.data.meshes, bpy.data.materials, bpy.data.lights, bpy.data.cameras):
        for b in list(col):
            if b.users == 0:
                col.remove(b)


def material(nombre, color, rugosidad=0.4, metal=0.0, emision=None, fuerza=0.0):
    m = bpy.data.materials.new(nombre)
    m.use_nodes = True
    p = m.node_tree.nodes["Principled BSDF"]
    p.inputs["Base Color"].default_value = (*color, 1)
    p.inputs["Roughness"].default_value = rugosidad
    p.inputs["Metallic"].default_value = metal
    if emision:
        p.inputs["Emission"].default_value = (*emision, 1)
        p.inputs["Emission Strength"].default_value = fuerza
    return m


def importar(ruta, nombre, mat, padre):
    bpy.ops.import_mesh.stl(filepath=ruta, global_scale=MM)
    o = bpy.context.selected_objects[0]
    o.name = nombre
    o.data.materials.append(mat)
    bpy.ops.object.shade_smooth()
    o.data.use_auto_smooth = True
    o.data.auto_smooth_angle = math.radians(35)
    o.parent = padre
    return o


def vacio(nombre, padre=None, z=0.0):
    o = bpy.data.objects.new(nombre, None)
    bpy.context.collection.objects.link(o)
    o.parent = padre
    o.location.z = z
    return o


def escena():
    limpiar()
    blanco = material("Blanco", (0.85, 0.84, 0.80), 0.35)
    naranja = material("Naranja", (0.85, 0.32, 0.04), 0.4)
    gris = material("Gris", (0.45, 0.46, 0.48), 0.3, metal=0.8)
    mica = material("Mica", (0.01, 0.01, 0.015), 0.05)
    anillo = material("Ojo", (0.0, 0.0, 0.0), 0.5, emision=(0.1, 0.75, 1.0), fuerza=8)
    piso = material("Piso", (0.32, 0.30, 0.28), 0.6)

    raiz = vacio("BB8", z=RE)
    cuerpo = vacio("Cuerpo", raiz)
    cabeza = vacio("Cabeza", raiz, z=RE)

    bpy.ops.mesh.primitive_uv_sphere_add(segments=96, ring_count=48, radius=RE - 0.3 * MM)
    esf = bpy.context.object
    esf.name = "Esfera"
    esf.data.materials.append(blanco)
    bpy.ops.object.shade_smooth()
    esf.parent = cuerpo
    out = os.path.join(CAD, "blender", "out")
    importar(os.path.join(out, "esfera_naranja.stl"), "Circulos", naranja, cuerpo)

    importar(os.path.join(CAD, "stl", "parte5-cabeza", "cabeza_casco.stl"), "Casco", blanco, cabeza)
    importar(os.path.join(out, "antenas.stl"), "Antenas", gris, cabeza)
    importar(os.path.join(out, "ojo_mica.stl"), "Mica", mica, cabeza)
    importar(os.path.join(out, "ojo_anillo.stl"), "Anillo", anillo, cabeza)

    bpy.ops.mesh.primitive_plane_add(size=12, location=(0, 1, 0))
    bpy.context.object.name = "Piso"
    bpy.context.object.data.materials.append(piso)

    bpy.ops.object.light_add(type="SUN", location=(2, -2, 4))
    sol = bpy.context.object
    sol.data.energy = 3
    sol.rotation_euler = (math.radians(50), 0, math.radians(30))
    bpy.ops.object.light_add(type="AREA", location=(-1.5, 1.5, 1.5))
    bpy.context.object.data.energy = 150
    bpy.context.object.data.size = 2

    bpy.ops.object.camera_add(location=(1.6, 2.4, 0.75))
    cam = bpy.context.object
    cam.data.lens = 40
    seguir = cam.constraints.new("TRACK_TO")
    seguir.target = raiz
    seguir.track_axis = "TRACK_NEGATIVE_Z"
    seguir.up_axis = "UP_Y"
    bpy.context.scene.camera = cam

    w = bpy.context.scene.world or bpy.data.worlds.new("Mundo")
    bpy.context.scene.world = w
    w.use_nodes = True
    w.node_tree.nodes["Background"].inputs["Color"].default_value = (0.55, 0.6, 0.68, 1)
    w.node_tree.nodes["Background"].inputs["Strength"].default_value = 0.6
    return raiz, cuerpo, cabeza, anillo


def suave(t):
    """0→1 con arranque y frenado suaves."""
    t = min(1.0, max(0.0, t))
    return t * t * (3 - 2 * t)


def animar(raiz, cuerpo, cabeza, anillo):
    sc = bpy.context.scene
    sc.render.fps = FPS
    sc.frame_start, sc.frame_end = 1, FIN
    brillo = anillo.node_tree.nodes["Principled BSDF"].inputs["Emission Strength"]
    x = y = 0.0
    rumbo = 0.0
    giro_bola = 0.0
    prev_v = 0.0
    for f in range(1, FIN + 1):
        s = f / FPS
        # velocidad (m/s) y rumbo por tramos: avanza, frena y mira, gira, regresa
        if s < 3:
            v = 0.45 * suave(s / 0.8) * (1 - suave((s - 2.2) / 0.8))
        elif s < 5.5:
            v = 0.0
        elif s < 6.5:
            v = 0.0
            rumbo = math.pi * suave((s - 5.5) / 1.0)
        else:
            v = 0.45 * suave((s - 6.5) / 0.8) * (1 - suave((s - 9.2) / 0.8))
        dt = 1 / FPS
        x += -math.sin(rumbo) * v * dt
        y += math.cos(rumbo) * v * dt
        giro_bola += v * dt / RE
        acel = (v - prev_v) / dt
        prev_v = v

        raiz.location = (x, y, RE)
        raiz.rotation_euler = (0, 0, rumbo)
        cuerpo.rotation_euler = (-giro_bola, 0, 0)
        # la cabeza se inclina con la aceleración y busca a los lados cuando está parado
        mira = math.radians(35) * math.sin((s - 3) / 2.5 * 2 * math.pi) if 3 <= s < 5.5 else 0.0
        cabeza.rotation_euler = (-0.12 * acel, 0.04 * math.sin(s * 3) * (v > 0.05), mira)
        # el ojo respira, y parpadea dos veces al detenerse
        parpadeo = 0.0 if (3.2 < s < 3.3 or 3.45 < s < 3.55) else 1.0
        brillo.default_value = (5 + 3 * math.sin(s * 2.5)) * parpadeo
        for o, props in ((raiz, ("location", "rotation_euler")), (cuerpo, ("rotation_euler",)), (cabeza, ("rotation_euler",))):
            for p in props:
                o.keyframe_insert(p, frame=f)
        brillo.keyframe_insert("default_value", frame=f)


def render():
    sc = bpy.context.scene
    sc.render.engine = "BLENDER_EEVEE"
    sc.eevee.use_soft_shadows = True
    sc.eevee.use_ssr = True
    sc.eevee.use_bloom = True
    sc.render.resolution_x, sc.render.resolution_y = 1280, 720


raiz, cuerpo, cabeza, anillo = escena()
animar(raiz, cuerpo, cabeza, anillo)
render()
bpy.context.scene.frame_set(60)
