import type { EgoWebSocket } from "$lib/websocket";

let websocket = $state<EgoWebSocket | null>(null);

export const wsStore = {
    get connection() { return websocket; },
    setConnection(ws: EgoWebSocket | null) {
        websocket = ws;
    }
};