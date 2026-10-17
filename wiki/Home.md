# Wiki del BB-8

Un BB-8 de 30–35 cm que rueda, escucha y ve, manejado por Claude a través de MCP o por
su propio agente (OpenRouter). Dos binarios de Go: `bb8-body` en la Pi 3 y `bb8-head` en
la Pi Zero W, unidos por un enlace TLS mutuo. Esta wiki explica cómo está hecho y cómo correr cada pieza, del
protoboard a la esfera cerrada.

## Empieza aquí

1. [Arquitectura](Arquitectura.md): las tres capas (reflejos, cuerpo, agente), qué corre en cada placa, la configuración en vivo y por qué el modelo nunca toca los motores.
2. [Simulador](Simulador.md): corre todo en tu PC sin hardware y pruébalo de punta a punta.
3. [Conectar con Claude](Conectar-con-Claude.md): Claude Code, Claude Desktop, el agente propio (OpenRouter, clima, calendario) y la página de configuración.

## Las seis partes, en orden

Cada parte termina con algo tangible. Nada de esfera hasta que la base ruede sola y
obedezca al agente; hasta la Parte 6 todo va con fuente de pared de 12 V.

| # | Parte | Corre en | Lista cuando |
| --- | --- | --- | --- |
| 1 | [Componentes en protoboard](Parte-1-Protoboard.md) | Arduino | Motores, servo, IMU y ToF responden; `I` contesta `OK BB8 fw=0.1` |
| 2 | [Base de melamina](Parte-2-Base-de-melamina.md) | — | Se levanta con una mano y nada se mueve |
| 3 | [Base andando desde la Pi](Parte-3-Base-andando.md) | Pi + Arduino | Avanza, gira 90° y vuelve con deriva < 5° |
| 4 | [Agente controla el movimiento](Parte-4-Agente-controla-el-movimiento.md) | Pi | "Oye Jarvis… ven acá" y se mueve |
| 5 | [Agente controla la cabeza](Parte-5-Agente-controla-la-cabeza.md) | Pi Zero + Pi | Gira la cabeza hacia ti y describe lo que ve |
| 6 | [Esfera y baterías](Parte-6-Esfera-y-baterias.md) | Pi + Arduino | Rueda sin cable, se duerme solo y despierta al moverlo |

## Referencia

- [Protocolo](Protocolo.md) (en inglés): serial Pi ↔ Arduino, herramientas, API local, API de configuración, enlace cabeza ↔ cuerpo, plano, pinout.
- [Instalación en la Pi](Instalacion-en-la-Pi.md): binarios, emparejar la cabeza, servicios systemd, `/etc/bb8.env` y las variables `BB8_*`.
- [Wake word "oye BB-8"](Wake-word-oye-BB-8.md): entrenar el modelo propio en Colab y ponerlo en el robot.
- [Materiales](Materiales.md): lo que ya se compró (pedido UNIT 377466) y lo que falta.
- [Diseño 3D](Diseno-3D.md): esfera, base, poste y cabeza en OpenSCAD paramétrico, con STL listos para imprimir.
- [Artefactos](Artefactos.md): documento técnico, diagramas, tracker de avance, mecánica y animación.
- [Pendientes](Pendientes.md): lo que falta probar o construir.

## Mapa del repo

```
bb8/
├── partes/
│   ├── 1-protoboard/arduino/       pruebas p1…p5 + bb8_firmware
│   ├── 2-base-melamina/            montaje (sin código)
│   ├── 3-base-andando/             calibrar/, mapas/ (planos); bb8/ (Python anterior)
│   ├── 4-agente-movimiento/        voz/ (micrófono → agente), wakeword/; bb8_mcp/ (Python anterior)
│   ├── 5-cabeza/                   head/ (pitidos y personalidad que usa voz; servidor Python anterior)
│   └── 6-esfera-baterias/          energia/ (reposo y batería)
├── go/
│   ├── cmd/bb8-body, cmd/bb8-head  los dos binarios
│   └── internal/                   motion, floorplan, link, head, config, agent, connectors…
├── cad/                            piezas imprimibles en OpenSCAD (STL, plantilla, renders)
├── simulador/                      dummy/ + lanzar_dummy.py
├── sistema/                        systemd e instaladores
├── Makefile                        make test · make release (arm64 y armv6) · make probar
├── docs/                           documento técnico y artefactos HTML
├── wiki/                           esta wiki
├── go.mod                          módulo de Go
├── pyproject.toml                  mapea cada paquete de Python a su carpeta (uv pip install -e .)
└── requirements.txt                voz, simulador y prueba
```
