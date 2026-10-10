#!/usr/bin/env bash
# Exporta los STL (cad/stl), la plantilla de la plataforma (cad/plantillas) y los
# renders (cad/png). Requiere OpenSCAD (2021.01 o más nuevo) y, para los PNG sin
# pantalla, xvfb-run.
#
#   bash cad/exportar.sh            # todo
#   bash cad/exportar.sh png        # solo los renders (segundos)
#   FN=96 bash cad/exportar.sh stl  # STL más finos (tarda más)
set -euo pipefail
cd "$(dirname "$0")"
QUE=${1:-todo}
export FN=${FN:-48}
JOBS=${JOBS:-$(nproc)}
mkdir -p stl png plantillas

pieza() {   # archivo.scad  nombre  parte
    openscad -q -D "\$fn=$FN" -D "parte=\"$3\"" -o "stl/$2.stl" "$1" && echo "  stl/$2.stl"
}
export -f pieza

if [[ $QUE == todo || $QUE == stl ]]; then
    echo "STL (FN=$FN, $JOBS a la vez; los gajos de la esfera tardan ~2 min cada uno):"
    {
        for p in A_ppp A_ppm A_pmp A_mpp A_pmm A_mpm A_mmp B_ppm B_pmp B_mpp B_pmm B_mpm B_mmp B_mmm; do
            echo "esfera.scad esfera_gajo_$p $p"
        done
        echo "esfera.scad esfera_circulo circulo"
        echo "esfera.scad esfera_tapa_carga tapa_carga"
        for p in soporte_motor soporte_motor_izq charola soporte_bt panel_carga; do echo "base.scad base_$p $p"; done
        for p in pie_poste soporte_servo portaimanes; do echo "poste.scad poste_$p $p"; done
        for p in casco plato tapon soporte_ojo soporte_bocina; do echo "cabeza.scad cabeza_$p $p"; done
    } | xargs -P "$JOBS" -L 1 bash -c 'pieza "$0" "$1" "$2"'
    python3 herramientas/stl_binario.py stl/*.stl > /dev/null
    openscad -q -D 'parte="plataforma_2d"' -o plantillas/plataforma.svg base.scad
    openscad -q -D 'parte="plataforma_2d"' -o plantillas/plataforma.dxf base.scad
fi

if [[ $QUE == todo || $QUE == png ]]; then
    echo "PNG:"
    png() {   # nombre  cámara  archivo  [-D ...]
        local n=$1 cam=$2 f=$3; shift 3
        xvfb-run -a openscad -q --imgsize=1400,1400 --projection=o --colorscheme=Tomorrow \
            --camera="$cam" "$@" -o "png/$n.png" "$f" && echo "  png/$n.png"
    }
    png bb8 0,0,40,78,0,205,1350 ensamble.scad -D corte=false
    png bb8_corte 0,0,20,70,0,305,1350 ensamble.scad -D corte=true
    png bb8_corte_lateral 0,0,30,90,0,270,1100 ensamble.scad -D corte=true
    png bb8_explosion 0,0,90,72,0,205,1700 ensamble.scad -D corte=false -D explotar=1
    png base 0,0,-90,62,0,215,560 base.scad
    png base_abajo 0,0,-90,130,0,215,560 base.scad
    png poste 0,0,50,75,0,30,500 poste.scad
    png cabeza 0,0,30,75,0,200,420 cabeza.scad
    png esfera 0,0,0,62,0,25,750 esfera.scad
fi
