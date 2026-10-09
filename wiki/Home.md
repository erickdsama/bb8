# Wiki del BB-8

Un BB-8 de 30–35 cm que rueda, escucha y ve, manejado por Claude a través de un
servidor MCP. Esta wiki explica cómo está hecho y cómo correr cada pieza, del
protoboard a la esfera cerrada.

## Empieza aquí

1. [Arquitectura](Arquitectura.md): las tres capas (reflejos, servicio, agente), qué corre en cada placa y por qué Claude nunca toca los motores.
2. [Simulador](Simulador.md): corre todo en tu PC sin hardware y pruébalo de punta a punta.
3. [Conectar con Claude](Conectar-con-Claude.md): Claude Code, Claude Desktop o el agente de voz.

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

- [Protocolo](Protocolo.md): serial Pi ↔ Arduino, HTTP del servicio de movimiento y de la cabeza, pinout.
- [Instalación en la Pi](Instalacion-en-la-Pi.md): servicios systemd, `/etc/bb8.env` y todas las variables `BB8_*`.
- [Wake word "oye BB-8"](Wake-word-oye-BB-8.md): entrenar el modelo propio en Colab y ponerlo en el robot.
- [Materiales](Materiales.md): lo que ya se compró (pedido UNIT 377466) y lo que falta.
- [Artefactos](Artefactos.md): documento técnico, diagramas, tracker de avance, mecánica y animación.
- [Pendientes](Pendientes.md): lo que falta probar o construir.

## Mapa del repo

```
bb8/
├── partes/
│   ├── 1-protoboard/arduino/       pruebas p1…p5 + bb8_firmware
│   ├── 2-base-melamina/            montaje (sin código)
│   ├── 3-base-andando/             bb8/ (servicio de movimiento), calibrar/
│   ├── 4-agente-movimiento/        bb8_mcp/ (servidor MCP), voz/ (agente de voz)
│   ├── 5-cabeza/                   head/ (servidor de la Pi Zero)
│   └── 6-esfera-baterias/          energia/ (reposo y batería)
├── simulador/                      dummy/ + lanzar_dummy.py
├── sistema/                        systemd e instaladores
├── docs/                           documento técnico y artefactos HTML
├── wiki/                           esta wiki
├── pyproject.toml                  mapea cada paquete a su carpeta (pip install -e .)
└── requirements.txt                Pi principal y PC
```
