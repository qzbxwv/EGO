let thoughtHeader = $state<string>('');
let textStream = $state<string>('');
let isDone = $state<boolean>(true);
let error = $state<string>('');
let streamingSessionId = $state<number | null>(null);
let lastUserMessage = $state<{ temp_id: number; log_id: number } | null>(null);
let consumedText = $state<string | null>(null);

export const streamStore = {
	get thoughtHeader() { return thoughtHeader; },
	set thoughtHeader(value) { thoughtHeader = value; },
	get textStream() { return textStream; },
	set textStream(value) { textStream = value; },
	get isDone() { return isDone; },
	set isDone(value) { isDone = value; },
	get error() { return error; },
	set error(value) { error = value; },
	get sessionId() { return streamingSessionId; },
	set sessionId(value) { streamingSessionId = value; },
	get lastUserMessage() { return lastUserMessage; }
};

export function startStream(sessionId: number | null) {
	thoughtHeader = '';
	textStream = '';
	isDone = false;
	error = '';
	consumedText = null;
	streamingSessionId = sessionId;
}

export function setThoughtHeader(header: string) {
	thoughtHeader = header;
}

export function appendToStream(chunk: string) {
	textStream += chunk;
}

export function endStream() {
	isDone = true;
	consumedText = textStream;
	textStream = '';
}

export function consumeStreamResult(): { text: string; sessionId: number | null } | null {
    if (consumedText !== null) {
        const result = { text: consumedText, sessionId: streamingSessionId };
        consumedText = null;
        thoughtHeader = '';
        streamingSessionId = null;
        return result;
    }
    return null;
}

export function setStreamError(errorMessage: string) {
	error = errorMessage;
	isDone = true;
	streamingSessionId = null;
}

export function setLastUserMessageLogId(temp_id: number, log_id: number) {
	lastUserMessage = { temp_id, log_id };
}

export function consumeLastUserMessageLogId() {
	const val = lastUserMessage;
	lastUserMessage = null;
	return val;
}

export function resetStreamStore() {
    thoughtHeader = '';
    textStream = '';
    isDone = true;
    error = '';
    streamingSessionId = null;
    lastUserMessage = null;
    consumedText = null;
}