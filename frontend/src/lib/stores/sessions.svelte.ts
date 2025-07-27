import { api } from '$lib/api';
import type { ChatSession } from '$lib/types';

let sessions = $state<ChatSession[]>([]);
let isLoadingSessions = $state(true);

export const sessionStore = {
	get sessions() { return sessions },
	get isLoading() { return isLoadingSessions }
};

let hasFetched = false;

export function addSession(newSession: ChatSession) {
	const exists = sessions.some(s => s.id === newSession.id);
	if (!exists) {
		sessions = [newSession, ...sessions];
	}
}

export function updateSession(updatedSession: ChatSession) {
    sessions = sessions.map((s: ChatSession) => s.id === updatedSession.id ? updatedSession : s);
}

export function removeSession(sessionId: number) {
    sessions = sessions.filter((s: ChatSession) => s.id !== sessionId);
}

export async function fetchSessions() {
	if (hasFetched) {
		isLoadingSessions = false;
		return;
	}
	
	isLoadingSessions = true;
	try {
		const fetchedSessions = await api.get<ChatSession[]>('/sessions');
		sessions = fetchedSessions;
		hasFetched = true;
	} catch (error: any) {
		sessions = [];
		console.error('Не удалось загрузить сессии:', error.message);
	} finally {
		isLoadingSessions = false;
	}
}

export function clearUserSessions() {
	sessions = [];
	isLoadingSessions = true;
	hasFetched = false;
}