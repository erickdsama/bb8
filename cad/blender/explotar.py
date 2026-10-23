"""BB-8 en Blender: vista explotada de la base, al final de la animación de animar.py.

Corre después de animar.py. Del segundo 12.5 al 14 la esfera desaparece, la cabeza sube
y sale de cuadro, y las piezas de la base se separan por capas: tarjetas arriba,
plataforma de MDF, soportes de motor, motores y llantas hacia los lados, ruedas locas en sus soportes, y la charola con la LiPo y el lastre abajo.
"""
import math

import bpy
from mathutils import Vector

FPS = 24
F0, F1, FIN = 300, 336, 372      # empieza, termina de separarse, fin del video
MM = 0.001


def desplazamiento(nombre):
    """Cuánto se separa cada pieza (mm, en el marco de la base)."""
    lado = 1 if nombre.endswith(("der", "der 1", "der 2")) else -1
    if nombre.startswith(("Raspberry", "Arduino", "L298N", "Buck", "MPU")):
        return (0, 0, 230)
    if nombre.startswith(("Pie del poste", "Soporte servo", "Portaimanes", "Tubo", "Servo", "Cuerno", "Iman poste")):
        return (0, 0, 450)   # el poste sale de cuadro
    if nombre.startswith("Plataforma"):
        return (0, 0, 160)
    if nombre.startswith("Soporte motor"):
        return (lado * 30, 0, 125)
    if nombre.startswith(("Caja engranes", "Motor", "Encoder")):
        return (lado * 30, 0, 80)
    if nombre.startswith(("Eje", "Llanta", "Rin")):
        return (lado * 90, 0, 80)
    if nombre.startswith("Soporte RL4"):
        s = 1 if "adelante" in nombre else -1
        return (0, s * 40, 120)
    if nombre.startswith("RL4"):
        s = 1 if "adelante" in nombre else -1
        return (0, s * 80, 60)
    if nombre.startswith("Panel"):
        return (-60, -60, 200)
    if nombre.startswith(("LiPo", "Velcro")):
        return (0, 0, 10)
    if nombre.startswith("Lastre"):
        return (lado * 60, 0, -10)
    if nombre.startswith("Charola"):
        return (0, 0, -40)
    return (0, 0, 0)


def main():
    sc = bpy.context.scene
    sc.frame_end = FIN
    base = bpy.data.objects["Base"]
    raiz = bpy.data.objects["BB8"]
    cabeza = bpy.data.objects["Cabeza"]

    # la base queda derecha mientras se separa
    for f in (F0 - 12, FIN):
        base.rotation_euler = (0, 0, 0)
        base.keyframe_insert("rotation_euler", frame=f)

    # esfera fuera: transparencia total (el material Blanco también es el del casco de la cabeza)
    for n in ("Blanco", "Naranja"):
        a = bpy.data.materials[n].node_tree.nodes["Principled BSDF"].inputs["Alpha"]
        sc.frame_set(F0 - 12)
        a.keyframe_insert("default_value", frame=F0 - 12)
        a.default_value = 0.0
        a.keyframe_insert("default_value", frame=F0 + 6)
    # la cabeza sube y sale de cuadro
    sc.frame_set(F0 - 12)
    z0 = cabeza.location.z
    cabeza.keyframe_insert("location", frame=F0 - 12)
    cabeza.location.z = z0 + 0.9
    cabeza.keyframe_insert("location", frame=F0 + 12)

    # el cableado no se separa: se esconde mientras la base se abre
    for o in bpy.data.objects:
        if o.name.startswith("Cableado"):
            for h in [o] + list(o.children_recursive):
                for f, oculto in ((F0 - 1, h.hide_render), (F0, True)):
                    h.hide_viewport = h.hide_render = oculto
                    h.keyframe_insert("hide_viewport", frame=f)
                    h.keyframe_insert("hide_render", frame=f)

    for o in base.children:
        if o.name.startswith("Cableado"):
            continue
        o.keyframe_insert("location", frame=F0)
        o.location = o.location + Vector(desplazamiento(o.name)) * MM
        o.keyframe_insert("location", frame=F1)

    # cámara más cerca para el final
    cam = sc.camera
    sc.frame_set(F0 - 12)
    cam.keyframe_insert("location", frame=F0 - 12)
    centro = raiz.matrix_world.translation
    cam.location = centro + Vector((0.55, 0.6, 0.18))
    cam.keyframe_insert("location", frame=F1)
    sc.frame_set(F1 + 12)


main()
