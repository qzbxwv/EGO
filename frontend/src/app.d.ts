declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface Platform {}
	}
	interface Window {
		google: any;
		katex: any; 	
		renderMathInElement: (element: HTMLElement, options?: any) => void;
	}
}

declare module 'marked' {
	export interface MarkedOptions {
		highlight?: (code: string, lang: string) => string;
		langPrefix?: string;
	}
}
export {};