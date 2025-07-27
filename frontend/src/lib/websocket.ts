import { PUBLIC_EGO_BACKEND_URL } from '$env/static/public';
import { auth } from '$lib/stores/auth.svelte.ts';
import type { ChatSession, WsEvent } from '$lib/types';
import { get } from 'svelte/store';

export interface WebSocketHandlers {
	onOpen: () => void;
	onClose: () => void;
	onSessionCreated: (session: ChatSession) => void;
	onThoughtHeader: (header: string) => void;
	onChunk: (text: string) => void;
	onDone: () => void;
	onError: (errorMsg: string) => void;
}

export interface EgoWebSocket {
	send: (payload: { query: string; mode: string; session_id: number | null; files?: any[] }) => void;
	close: () => void;
}

export function connectWebSocket(token: string, handlers: WebSocketHandlers): EgoWebSocket {
	if (!token) {
		handlers.onError('Вы не авторизованы. Обновите страницу.');
		return { send: () => {}, close: () => {} };
	}

	const wsUrl = PUBLIC_EGO_BACKEND_URL.replace(/^http/, 'ws') + `/ws?token=${token}`;
	const ws = new WebSocket(wsUrl);

	ws.onopen = handlers.onOpen;

	ws.onclose = (event) => {
		if (event.code !== 1000) {
			console.error(`WebSocket closed unexpectedly. Code: ${event.code}, Reason: ${event.reason}`);
			handlers.onError(`Соединение потеряно (код: ${event.code})`);
		}
		handlers.onClose();
	};
	
	ws.onerror = (error) => {
		console.error("WebSocket error observed:", error);
	};
	
	ws.onmessage = (event) => {
		if (typeof event.data !== 'string') return;
		try {
			const wsEvent: WsEvent = JSON.parse(event.data);
			const data = wsEvent.data;

			if (wsEvent.type === 'error') {
				const errorMessage = typeof data === 'object' && data.message ? data.message : 'Произошла неизвестная ошибка на сервере';
				handlers.onError(errorMessage);
				return;
			}
			
			switch (wsEvent.type) {
				case 'session_created':
					handlers.onSessionCreated(data);
					break;
				
				case 'thought_header':
					if (typeof data === 'string' && data) {
						handlers.onThoughtHeader(data);
					}
					break;
				
				case 'chunk':
					if (data && typeof data.text === 'string' && data.text) {
						handlers.onChunk(data.text);
					}
					break;

				case 'done':
					handlers.onDone();
					break;

				case 'status':
				case 'usage_update':
				case 'tool_call':
				case 'tool_output':
					break;

				default:
					console.warn("Получен неизвестный тип события WebSocket:", wsEvent.type);
			}
		} catch (error) {
			console.error("Failed to parse WebSocket message:", event.data, error);
			handlers.onError('Ошибка обработки ответа от сервера.');
		}
	};

	return {
		send: (payload) => {
			if (ws.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify(payload));
			} else {
				handlers.onError('Соединение не открыто. Пожалуйста, обновите страницу.');
			}
		},
		close: () => {
			if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
				ws.close(1000, "User initiated disconnect");
			}
		}
	};
}