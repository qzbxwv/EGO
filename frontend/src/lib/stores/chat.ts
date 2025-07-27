import { writable } from 'svelte/store';
import type { ChatMessage } from '$lib/types';

interface OptimisticState {
	userMessage: ChatMessage;
	inProgressMessage: ChatMessage;
}

export const optimisticStateStore = writable<OptimisticState | null>(null);