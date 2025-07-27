import { api } from '$lib/api';
import type { PageLoad } from './$types';
import type { ChatSession, HistoryLog, ChatMessage } from '$lib/types';
import { error } from '@sveltejs/kit';

export const load: PageLoad = async ({ params, fetch }) => {
	const { sessionID } = params;
	if (sessionID === 'new' || !sessionID) {
		return {
			session: null,
			messages: [] as ChatMessage[]
		};
	}

	try {
		const [session, historyLogs] = await Promise.all([
			api.get<ChatSession>(`/sessions/${sessionID}`, fetch),
			api.get<HistoryLog[]>(`/sessions/${sessionID}/history`, fetch)
		]);
		const messages: ChatMessage[] = historyLogs.flatMap((log: HistoryLog) => {
			const msgs: ChatMessage[] = [];
			msgs.push({ 
				author: 'user', 
				text: log.user_query, 
				id: log.id * 2,
				attachments: log.attachments 
			});
			
			if (log.final_response) {
				msgs.push({ 
					author: 'ego', 
					text: log.final_response, 
					id: log.id * 2 + 1,
				});
			}
			return msgs;
		});
		return {
			session,
			messages
		};

	} catch (err: any) {
		throw error(404, `Не удалось загрузить чат: ${err.message}`);
	}
};