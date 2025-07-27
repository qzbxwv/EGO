<script lang="ts">
    import { auth } from '$lib/stores/auth.svelte.ts';
    import { wsStore } from '$lib/stores/websocket.svelte.ts';
    import { connectWebSocket } from '$lib/websocket';
    import { goto } from '$app/navigation';
    import { addSession } from '$lib/stores/sessions.svelte.ts';
    import { toast } from 'svelte-sonner';
    import { 
        streamStore,
        setThoughtHeader, 
        appendToStream, 
        endStream, 
        setStreamError,
        setStreamingSessionId
    } from '$lib/stores/stream.svelte.ts';

    const { children } = $props();

    $effect(() => {
        const token = auth.accessToken;
        if (!token) {
            goto('/login');
            return;
        }

        const egoWs = connectWebSocket(token, {
            onOpen: () => {},
            onClose: () => {
                wsStore.setConnection(null);
            },
            onSessionCreated: (newSession) => {
                if (newSession && newSession.id) {
                    addSession(newSession);
                    setStreamingSessionId(newSession.id);
                    goto(`/chat/${newSession.id}`, { replaceState: true, invalidateAll: true });
                }
            },
            onThoughtHeader: setThoughtHeader,
            onChunk: appendToStream,
            onDone: endStream,
            onError: (errorMsg) => {
                setStreamError(errorMsg);
                toast.error(errorMsg);
            },
        });
        
        wsStore.setConnection(egoWs);

        return () => {
            egoWs?.close();
            wsStore.setConnection(null);
        };
    });
</script>

{@render children()}