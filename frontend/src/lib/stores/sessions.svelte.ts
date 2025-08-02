import { api } from '$lib/api';
import type { ChatSession } from '$lib/types';

let sessions = $state<ChatSession[]>([]);
let isLoadingSessions = $state(true);

export const sessionStore = {
	get sessions() { return sessions },
	get isLoading() { return isLoadingSessions }
};

export function setInitialSessions(initialSessions: ChatSession[]) {
    sessions = initialSessions.sort((a, b) => b.id - a.id);
    isLoadingSessions = false;
}

export function addSession(newSession: ChatSession) {
    if (sessions.some(s => s.id === newSession.id)) {
        console.warn(`Attempted to add a duplicate session (ID: ${newSession.id}). Ignoring.`);
        return;
    }
    sessions = [newSession, ...sessions];
}

export function updateSession(updatedSession: ChatSession) {
    sessions = sessions.map((s: ChatSession) => s.id === updatedSession.id ? updatedSession : s);
}

export function removeSession(sessionId: number) {
    sessions = sessions.filter((s: ChatSession) => s.id !== sessionId);
}

export function clearUserSessions() {
	sessions = [];
	isLoadingSessions = true;
}