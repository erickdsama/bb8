# BB-8 · Bruno collection

[Bruno](https://www.usebruno.com) collection for the robot: MCP tools, the config API and the local API.

## Open it

1. Bruno → **Open Collection** → pick this `bruno/` folder.
2. Choose the environment: **Pi** (the real robot) or **Dummy** (the simulator, `BB8_MCP_PORT=8775`).
3. In **Pi**, set the secret `token` to the content of `/etc/bb8/token`:
   `ssh bb8.local sudo cat /etc/bb8/token`

## MCP (folder `MCP`)

The MCP server keeps a session. Run **01 Initialize** and **02 Initialized** once; after that any tool request works.
Responses are SSE (`event: message` / `data: {...}`).

## Local API (folder `Local API`)

It listens on 127.0.0.1:8770 of the Pi. Open a tunnel first: `ssh -L 8770:127.0.0.1:8770 bb8.local`.
