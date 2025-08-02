export interface User {
	id: number;
	username: string;
	role: 'user';
	created_at: string;
}

export interface AuthResponse {
	access_token: string;
	refresh_token: string;
	user: User;
}

export interface FileAttachment {
	file_name: string;
	mime_type: string;
}

export interface ChatSession {
	id: number;
	title: string;
	mode: string;
	custom_instructions: string | null;
	created_at: string;
}

export interface HistoryLog {
	id: number;
	user_query: string;
	final_response: string | null;
	timestamp: string;
	attachments: FileAttachment[];
}

export interface ChatMessage {
	id: number;
	author: 'user' | 'ego';
	text: string;
	isThinking?: boolean;
	attachments?: FileAttachment[];
	logId?: number;
}

export interface FilePayload {
	base64_data: string;
	mime_type: string;
	file_name: string;
}

export interface WsMessage {
	query: string;
	mode: string;
	session_id?: number | null;
	files?: FilePayload[];
	is_regeneration?: boolean;
	request_log_id_to_regen?: number;
	custom_instructions?: string;
	temp_id?: number;
}

export interface WsEvent {
	type: string;
	data: any;
}

export interface StreamChunkData {
	text: string;
}