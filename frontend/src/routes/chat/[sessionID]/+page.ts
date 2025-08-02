import { api } from '$lib/api';
import type { PageLoad } from './$types';
import type { ChatSession, HistoryLog, ChatMessage } from '$lib/types';
import { error } from '@sveltejs/kit';
import { browser } from '$app/environment';

export const load: PageLoad = async ({ params, fetch }) => {
	const { sessionID } = params;

	if (sessionID === 'new' || !sessionID) {
		return {
			session: null,
			messages: [] as ChatMessage[]
		};
	}

	if (browser) {
		try {
			const [session, historyLogs] = await Promise.all([
				api.get<ChatSession>(`/sessions/${sessionID}`, fetch),
				api.get<HistoryLog[]>(`/sessions/${sessionID}/history`, fetch)
			]);

			const messages: ChatMessage[] = historyLogs.flatMap((log: HistoryLog) => {
				const msgs: ChatMessage[] = [];
				if (log.user_query || (log.attachments && log.attachments.length > 0)) {
					msgs.push({
						author: 'user',
						text: log.user_query,
						id: log.id * 2,
						attachments: log.attachments,
						logId: log.id
					});
				}
				if (log.final_response) {
					msgs.push({
						author: 'ego',
						text: log.final_response,
						id: log.id * 2 + 1
					});
				}
				return msgs;
			});

			const optimisticNewJSON = sessionStorage.getItem('optimistic_new_chat_message');
			if (optimisticNewJSON) {
				const optimisticMessage = JSON.parse(optimisticNewJSON);
				if (!messages.some((m: ChatMessage) => m.id === optimisticMessage.id)) {
					messages.push(optimisticMessage);
				}
				sessionStorage.removeItem('optimistic_new_chat_message');
			}

			const optimisticCurrentKey = `optimistic_message_${sessionID}`;
			const optimisticCurrentJSON = sessionStorage.getItem(optimisticCurrentKey);
			if (optimisticCurrentJSON) {
				const optimisticMessage = JSON.parse(optimisticCurrentJSON);
                if (!messages.some((m: ChatMessage) => m.id === optimisticMessage.id)) {
					messages.push(optimisticMessage);
				}
				sessionStorage.removeItem(optimisticCurrentKey);
			}

			return {
				session,
				messages
			};
		} catch (err: any) {
			throw error(404, `Не удалось загрузить чат: ${err.message}`);
		}
	}

    return {
		session: null,
		messages: [] as ChatMessage[]
	};
};