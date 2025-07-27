let thoughtHeader = $state<string>('');
let textStream = $state<string>('');
let isDone = $state<boolean>(true);
let error = $state<string>('');
let streamingSessionId = $state<number | null>(null);

export const streamStore = {
	get thoughtHeader() { return thoughtHeader; },
	get textStream() { return textStream; },
	get isDone() { return isDone; },
	get error() { return error; },
	get sessionId() { return streamingSessionId; },
};

export function setStreamingSessionId(id: number) {
	streamingSessionId = id;
}

export function startStream() {
	thoughtHeader = '';
	textStream = '';
	isDone = false;
	error = '';
}

export function setThoughtHeader(header: string) {
	thoughtHeader = header;
}

export function appendToStream(chunk: string) {
	textStream += chunk;
}

export function endStream() {
	isDone = true;
	streamingSessionId = null;
}

export function setStreamError(errorMessage: string) {
	error = errorMessage;
	isDone = true;
	streamingSessionId = null;
}