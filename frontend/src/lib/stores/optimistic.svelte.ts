import type { ChatMessage } from '$lib/types';

let message = $state<ChatMessage | null>(null);

export const optimisticMessageStore = {
    get message() {
		return message;
	},

	set(msg: ChatMessage) {
		message = msg;
	},

	consume(): ChatMessage | null {
		const msg = message;
		message = null;
		return msg;
	}
};