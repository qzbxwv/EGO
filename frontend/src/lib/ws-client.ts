import { auth } from '$lib/stores/auth.svelte.ts';
import { wsStore } from '$lib/stores/websocket.svelte.ts';
import { connectWebSocket, type EgoWebSocket } from '$lib/websocket';
import { goto } from '$app/navigation';
import { addSession } from '$lib/stores/sessions.svelte.ts';
import { toast } from 'svelte-sonner';
import { 
    streamStore,
    setThoughtHeader, 
    appendToStream, 
    endStream, 
    setStreamError,
    setLastUserMessageLogId
} from '$lib/stores/stream.svelte.ts';
import { page } from '$app/stores';
import { get } from 'svelte/store';

let wsInstance: EgoWebSocket | null = null;

export function initializeWebSocket() {
    if (wsInstance || !auth.accessToken) {
        return;
    }

    console.log("Initializing WebSocket connection...");
    
    wsInstance = connectWebSocket(auth.accessToken, {
        onOpen: () => {
            console.log("WebSocket connection established successfully.");
            wsStore.setConnection(wsInstance);
        },
        onClose: () => {
            console.log("WebSocket connection closed.");
            wsStore.setConnection(null);
            wsInstance = null; 
        },
        onSessionCreated: (newSession) => {
            if (newSession && newSession.id) {
                const currentPage = get(page);
                addSession(newSession);
                
                streamStore.sessionId = newSession.id;

                if (currentPage.params.sessionID === 'new') {
                    goto(`/chat/${newSession.id}`, { replaceState: true });
                }
            }
        },
        onLogCreated: ({ log_id, temp_id }) => {
            setLastUserMessageLogId(temp_id, log_id);
        },
        onThoughtHeader: setThoughtHeader,
        onChunk: appendToStream,
        onDone: endStream,
        onError: (errorMsg) => {
            setStreamError(errorMsg);
            toast.error(errorMsg);
        },
    });
}

export function disconnectWebSocket() {
    if (wsInstance) {
        console.log("Disconnecting WebSocket.");
        wsInstance.close();
        wsInstance = null;
        wsStore.setConnection(null);
    }
}